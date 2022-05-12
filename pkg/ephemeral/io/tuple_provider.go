package io

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/carbynestack/ephemeral/pkg/castor"
	"github.com/carbynestack/ephemeral/pkg/types"
	"github.com/google/uuid"
	"golang.org/x/sys/unix"
	"math/big"
	"net/url"
	"os"
	"time"
)

// TupleProvider writes Tuples to named Pipes
type TupleProvider interface {
	StartWritingToFiles() error
	StopWritingToFiles()
}

type TupleProviderImpl struct {
	InputTypes      []castor.InputType
	NumberOfThreads int
	config          *types.SPDZEngineTypedConfig

	cancelFunc context.CancelFunc

	castorClient castor.AbstractClient
}

func NewTupleProvider(inputTypes []castor.InputType, numberOfThreads int, config *types.SPDZEngineTypedConfig, castorUrl *url.URL) *TupleProviderImpl {
	client, _ := castor.NewCastorClient(castorUrl)
	return &TupleProviderImpl{
		InputTypes:      inputTypes,
		NumberOfThreads: numberOfThreads,
		config:          config,
		castorClient:    client,
	}
}

func (t *TupleProviderImpl) StartWritingToFiles() error {
	ctx, cancelFunc := context.WithCancel(context.Background())
	t.cancelFunc = cancelFunc

	for threadNumber := 0; threadNumber < t.NumberOfThreads; threadNumber++ {
		for _, inputType := range t.InputTypes {
			writer := newTupleFileWriter(threadNumber, inputType, t.castorClient, t.config)
			if err := writer.startWritingToFile(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (t *TupleProviderImpl) StopWritingToFiles() {
	t.cancelFunc()
}

type tuplePipeWriter struct {
	threadNumber                   int
	inputType                      castor.InputType
	file                           *os.File
	config                         *types.SPDZEngineTypedConfig
	cache                          []byte
	indexAtCache                   int
	requestCounter                 int
	numberOfTuplesToDownloadAtOnce int
	castorClient                   castor.AbstractClient
}

func newTupleFileWriter(threadNumber int, inputType castor.InputType, castorClient castor.AbstractClient, config *types.SPDZEngineTypedConfig) tuplePipeWriter {
	return tuplePipeWriter{
		threadNumber: threadNumber,
		inputType:    inputType,
		castorClient: castorClient,
		cache:        generateHeader(inputType, &config.Prime),
		config:       config,
	}
}

func generateHeader(inputType castor.InputType, prime *big.Int) []byte {
	descriptor := []byte(castor.ProtocolDescriptorFor(inputType))
	lengthOfDescriptor := len(descriptor)

	totalSizeInBytes := uint64(8 + lengthOfDescriptor + 1 + 4 + 16)

	var result []byte

	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, totalSizeInBytes)
	result = append(result, bytes...)
	result = append(result, descriptor...)

	bytes = make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(prime.BitLen()))
	result = append(result, bytes...)

	return append(result, prime.Bytes()...)
}

func (t *tuplePipeWriter) startWritingToFile(ctx context.Context) error {
	fileName := castor.TupleFileNameFor(t.inputType, t.threadNumber, t.config)

	err := unix.Mkfifo(fileName, 0666)
	if err != nil {
		return err
	}

	t.file, err = os.OpenFile(fileName, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return err
	}

	err = t.file.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_ = t.file.SetWriteDeadline(time.Now().Add(5 * time.Second))
				t.writeToFile()
			}
		}
	}()

	return nil
}

func (t *tuplePipeWriter) writeToFile() {

	t.updateCacheAndIndex()
	t.fetchTuplesIfNeeded()

	write, err := t.file.Write(t.cache)
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
	t.indexAtCache = write
}

func (t *tuplePipeWriter) updateCacheAndIndex() {
	if t.indexAtCache == 0 {
		return
	}

	if t.cache == nil {
		t.cache = make([]byte, 0)
		t.indexAtCache = 0
	} else {
		t.cache = t.cache[t.indexAtCache:]
		t.indexAtCache = 0
	}
}

func (t *tuplePipeWriter) fetchTuplesIfNeeded() {
	if len(t.cache) > 0 {
		return
	}

	requestName := fmt.Sprintf("%d-%s-%d", t.threadNumber, t.inputType, t.requestCounter)
	t.requestCounter++
	requestId := uuid.NewMD5(uuid.Nil, []byte(requestName))
	files, err := t.castorClient.DownloadTupleFiles(requestId, t.numberOfTuplesToDownloadAtOnce, t.inputType)
	t.cache = t.convertResult(files)
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
}

func (t *tuplePipeWriter) convertResult(files castor.TupleList) []byte {
	var result []byte

	var b64 []string
	for _, tuple := range files.Tuples {
		for _, share := range tuple.Shares {
			b64 = append(b64, share.Value)
			b64 = append(b64, share.Mac)
		}
	}

	packer := SPDZPacker{
		MaxBulkSize: 10000,
	}
	err := packer.Marshal(b64, &result)
	if err != nil {
		fmt.Printf("Error: %v", err)
	}

	return result
}

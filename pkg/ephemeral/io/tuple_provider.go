package io

import (
	"context"
	"encoding/base64"
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
	StartWritingToFiles(ctx context.Context) error
	StopWritingToFiles()
}

type TupleProviderImpl struct {
	InputTypes      []castor.InputType
	NumberOfThreads int
	config          *types.SPDZEngineTypedConfig

	cancelFunc context.CancelFunc

	castorClient castor.AbstractClient
	gameId       uuid.UUID
}

func NewTupleProvider(inputTypes []castor.InputType, numberOfThreads int, config *types.SPDZEngineTypedConfig, castorUrl *url.URL, gameId uuid.UUID) *TupleProviderImpl {
	client, _ := castor.NewCastorClient(castorUrl)

	return &TupleProviderImpl{
		InputTypes:      inputTypes,
		NumberOfThreads: numberOfThreads,
		config:          config,
		castorClient:    client,
		gameId:          gameId,
	}
}

func (t *TupleProviderImpl) StartWritingToFiles(ctx context.Context) error {

	for threadNumber := 0; threadNumber < t.NumberOfThreads; threadNumber++ {
		for _, inputType := range t.InputTypes {
			writer := newTupleFileWriter(threadNumber, inputType, t.castorClient, t.config, t.gameId)
			if err := writer.startWritingToFile(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (t *TupleProviderImpl) StopWritingToFiles() {
	// No-Op since already handled by the context
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
	gameId                         uuid.UUID
}

func newTupleFileWriter(threadNumber int, inputType castor.InputType, castorClient castor.AbstractClient, config *types.SPDZEngineTypedConfig, gameId uuid.UUID) tuplePipeWriter {
	return tuplePipeWriter{
		threadNumber:                   threadNumber,
		inputType:                      inputType,
		castorClient:                   castorClient,
		cache:                          generateHeader(inputType, &config.Prime),
		config:                         config,
		numberOfTuplesToDownloadAtOnce: 10,
		gameId:                         gameId,
	}
}

func generateHeader(inputType castor.InputType, prime *big.Int) []byte {
	descriptor := []byte(castor.ProtocolDescriptorFor(inputType))
	lengthOfDescriptor := len(descriptor)
	primeBytes := prime.Bytes()
	primeByteLength := len(primeBytes)
	totalSizeInBytes := uint64(lengthOfDescriptor + 1 + 4 + primeByteLength)

	var result []byte

	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, totalSizeInBytes)
	result = append(result, bytes...)      // Total length to follow (e.g. 29 bytes)
	result = append(result, descriptor...) // e.g. "SPDZ gfp"
	result = append(result, byte(0))       //Signum (0 == positive)

	bytes = make([]byte, 4)

	binary.LittleEndian.PutUint32(bytes, uint32(primeByteLength))
	result = append(result, bytes...) // Prime length to follow (e.g. 16 byte == 128 bit)

	result = append(result, primeBytes...) // The prime itself
	fmt.Printf("%x\n", result)
	fmt.Printf("Prime: %s\n", prime.Text(10))

	return result
}

func (t *tuplePipeWriter) startWritingToFile(ctx context.Context) error {
	fileName := castor.TupleFileNameFor(t.inputType, t.threadNumber, t.config)
	fmt.Printf("FileName: %s\n", fileName)

	_ = os.Remove(fileName)
	err := unix.Mkfifo(fileName, 0666)
	if err != nil {
		return err
	}

	go func() {
		t.file, err = os.OpenFile(fileName, os.O_WRONLY, os.ModeNamedPipe)
		if err != nil {
			fmt.Printf("Error Opening File: %v\n", err)
			return
		}

		err = t.file.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			fmt.Printf("Error Setting Write Deadline 1: %v\n", err)
			return
		}
		for {
			select {
			case <-ctx.Done():
				_ = t.file.Close()
				return
			default:
				in5Seconds := time.Now().Add(5 * time.Second)
				if err := t.file.SetWriteDeadline(in5Seconds); err != nil {
					fmt.Printf("Error Setting Write Deadline 1: %v\n", err)
					return
				}
				if err := t.writeToFile(); err != nil {
					fmt.Printf("Error writing to File: %v\n", err)
					return
				}
			}
		}
	}()

	return nil
}

func (t *tuplePipeWriter) writeToFile() error {

	t.updateCacheAndIndex()
	if err := t.fetchTuplesIfNeeded(); err != nil {
		return err
	}

	fmt.Printf("Writing to Pipe of type %s\n", t.inputType)
	write, err := t.file.Write(t.cache)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return err
	}
	fmt.Printf("Wrote %d bytes to file of type %s\n", write, t.inputType)
	t.indexAtCache = write

	return nil
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

func (t *tuplePipeWriter) fetchTuplesIfNeeded() error {
	if len(t.cache) > 0 {
		return nil
	}

	fmt.Printf("Fetching Tuples of type %s\n", t.inputType)

	requestName := fmt.Sprintf("%d-%s-%d", t.threadNumber, t.inputType, t.requestCounter)
	t.requestCounter++
	requestId := uuid.NewMD5(t.gameId, []byte(requestName))
	files, err := t.castorClient.DownloadTupleFiles(requestId, t.numberOfTuplesToDownloadAtOnce, t.inputType)
	if err != nil {
		fmt.Printf("Error: %v", err)
		return err
	}
	t.cache, err = t.convertResult(files)
	if err != nil {
		return err
	}

	return nil
}

func (t *tuplePipeWriter) convertResult(files castor.TupleList) ([]byte, error) {
	var result []byte

	for _, tuple := range files.Tuples {
		for _, share := range tuple.Shares {
			decodeString, err := base64.StdEncoding.DecodeString(share.Value)
			if err != nil {
				return []byte{}, err
			}
			result = append(result, decodeString...)

			decodeString, err = base64.StdEncoding.DecodeString(share.Mac)
			if err != nil {
				return []byte{}, err
			}
			result = append(result, decodeString...)
		}
	}

	return result, nil
}

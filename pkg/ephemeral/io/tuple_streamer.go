//
// Copyright (c) 2022-2023 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//

package io

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/carbynestack/ephemeral/pkg/utils"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/carbynestack/ephemeral/pkg/castor"
	. "github.com/carbynestack/ephemeral/pkg/types"
)

// PipeWriterFactory is a factory method to create new PipeWriter.
//
// It accepts a logger, filepath of the pipe to write to and a deadline for write operations. It either returns a
// PipeWriter or an error if creation failed.
type PipeWriterFactory func(l *zap.SugaredLogger, filePath string, writeDeadline time.Duration) (PipeWriter, error)

// DefaultPipeWriterFactory constructs a new PipeWriter instance
func DefaultPipeWriterFactory(l *zap.SugaredLogger, filePath string, writeDeadline time.Duration) (PipeWriter, error) {
	return NewTuplePipeWriter(l, filePath, writeDeadline)
}

// PipeWriter provides methods to access and write to pipes
type PipeWriter interface {
	Open() error
	Write(data []byte) (int, error)
	Close() error
}

// NewTuplePipeWriter returns a new TuplePipeWriter with the given configuration. It will create a new pipe with the
// given path. If a file with this path already exists, it will be deleted first.
func NewTuplePipeWriter(l *zap.SugaredLogger, filePath string, writeDeadline time.Duration) (*TuplePipeWriter, error) {
	logger := l.With("FilePath", filePath)
	err := utils.Fio.Delete(filePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error deleting existing Tuple file: %v", err)
	}
	err = utils.Fio.CreatePipe(filePath)
	if err != nil {
		logger.Debugw("Error creating pipe", "Error", err)
		return nil, err
	}
	return &TuplePipeWriter{
		tupleFilePath: filePath,
		writeDeadline: writeDeadline,
		logger:        logger,
	}, nil
}

// TuplePipeWriter implements PipeWriter
type TuplePipeWriter struct {
	tupleFilePath string
	tupleFile     utils.File
	writeDeadline time.Duration
	logger        *zap.SugaredLogger
}

// Write writes the given data to the underlying tuple file pipe.
//
// **Note:** Make sure to call Open() first.
func (tpw *TuplePipeWriter) Write(data []byte) (int, error) {
	deadline := time.Now().Add(tpw.writeDeadline)
	err := tpw.tupleFile.SetWriteDeadline(deadline)
	if err != nil {
		return 0, fmt.Errorf("error setting write deadline: %v", err)
	}
	return tpw.tupleFile.Write(data)
}

// Open opens a file as write only pipe.
//
// This function should be called within a go routine as opening a pipe as write-only is a blocking call until opposing
// side opens the file to read.
// This is especially important when streaming tuples to MP-SPDZ, as it does open only those tuple files that are
// actually required for the current computation.
func (tpw *TuplePipeWriter) Open() error {
	var err error
	tpw.tupleFile, err = utils.Fio.OpenWritePipe(tpw.tupleFilePath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	tpw.logger.Debugw("Pipe writer connected")
	return nil
}

// Close calls File.Close() on the tuple file pipe
func (tpw *TuplePipeWriter) Close() error {
	if tpw.tupleFile == nil {
		return os.ErrInvalid
	}
	return tpw.tupleFile.Close()
}

// TupleStreamer is an interface.
type TupleStreamer interface {
	StartStreamTuples(terminateCh chan struct{}, errCh chan error, wg *sync.WaitGroup)
}

// GetTupleFileName returns the filename for a given tuple type, spdz configuration and thread number
func GetTupleFileName(tt castor.TupleType, conf *SPDZEngineTypedConfig, threadNr int) string {
	return fmt.Sprintf("%s-%s-P%d-T%d",
		tt.PreprocessingName, tt.SpdzProtocol.Shorthand, conf.PlayerID, threadNr)
}

// NewCastorTupleStreamer returns a new instance of castor tuple streamer.
func NewCastorTupleStreamer(l *zap.SugaredLogger, tt castor.TupleType, conf *SPDZEngineTypedConfig, playerDataDir string, gameID uuid.UUID, threadNr int) (*CastorTupleStreamer, error) {
	ts, err := NewCastorTupleStreamerWithWriterFactory(l, tt, conf, playerDataDir, gameID, threadNr, DefaultPipeWriterFactory)
	return ts, err
}

// NewCastorTupleStreamerWithWriterFactory returns a new instance of castor tuple streamer.
func NewCastorTupleStreamerWithWriterFactory(l *zap.SugaredLogger, tt castor.TupleType, conf *SPDZEngineTypedConfig, playerDataDir string, gameID uuid.UUID, threadNr int, pipeWriterFactory PipeWriterFactory) (*CastorTupleStreamer, error) {
	loggerWithContext := l.With(GameID, gameID, TupleType, tt, "ThreadNr", threadNr)
	tupleFileName := GetTupleFileName(tt, conf, threadNr)
	filePath := filepath.Join(playerDataDir, tupleFileName)
	pipeWriter, err := pipeWriterFactory(loggerWithContext, filePath, conf.ComputationTimeout)
	if err != nil {
		return nil, fmt.Errorf("error creating pipe writer: %v", err)
	}
	headerData, err := generateHeader(tt.SpdzProtocol, conf)
	if err != nil {
		return nil, fmt.Errorf("error creating header: %v", err)
	}
	loggerWithContext.Debugf("Generated tuple file header: %x", headerData)
	return &CastorTupleStreamer{
		logger:        loggerWithContext,
		pipeWriter:    pipeWriter,
		tupleType:     tt,
		stockSize:     conf.TupleStock,
		castorClient:  conf.CastorClient,
		baseRequestID: uuid.NewMD5(gameID, []byte(tt.Name+strconv.Itoa(threadNr))),
		headerData:    headerData,
	}, nil
}

// CastorTupleStreamer provides tuples to the SPDZ execution for the given type and configuration.
type CastorTupleStreamer struct {
	logger         *zap.SugaredLogger
	pipeWriter     PipeWriter
	tupleType      castor.TupleType
	stockSize      int32
	castorClient   castor.AbstractClient
	baseRequestID  uuid.UUID
	requestCycle   int
	headerData     []byte
	streamData     []byte
	streamerDoneCh chan struct{}
	tupleBufferCh  chan []byte
	fetchTuplesCh  chan struct{}
	// bufferLckCh is used as a synchronization lock, where one routine can lock the channel by writing to it. Each
	// consecutive write will block the writing routine until the channel has been unlocked by reading from it. In
	// combination with a "select" statement, the channel can be used as a "timeout-able" lock.
	//
	// Reading is supposed to be performed by the initial routine which wrote to the channel.
	bufferLckCh   chan struct{}
	streamedBytes int
}

// StartStreamTuples repeatedly downloads a given type of tuples from castor and streams it to the according file as
// required by MP-SPDZ
func (ts *CastorTupleStreamer) StartStreamTuples(terminateCh chan struct{}, errCh chan error, wg *sync.WaitGroup) {
	ts.streamData = append(ts.streamData, ts.headerData...)
	ts.streamerDoneCh = make(chan struct{})
	ts.fetchTuplesCh = make(chan struct{}, 1)
	ts.bufferLckCh = make(chan struct{}, 1)
	ts.tupleBufferCh = make(chan []byte, 1)
	ts.fetchTuplesCh <- struct{}{}
	go func() {
		defer func() {
			close(ts.streamerDoneCh)
			select {
			case ts.bufferLckCh <- struct{}{}:
				// Lock the buffer routine or wait in case the channel is currently "locked". A blocking write indicates
				// that the bufferData routine is currently fetching new tuples from castor. As we want the information
				// on discarded bytes to be as accurate as possible, we will wait some time for the tuples to be fetched
				// before computing discardedTupleBytes.
			case <-time.After(10 * time.Second):
				// However, we will not wait for too long for the bufferData routine to finish
			}
			discardedTupleBytes := 0
			select {
			case buffered := <-ts.tupleBufferCh:
				discardedTupleBytes = len(buffered)
			default:
			}
			var streamedTupleBytes int
			if ts.streamedBytes > len(ts.headerData) {
				streamedTupleBytes = ts.streamedBytes - len(ts.headerData)
			}
			if streamedTupleBytes > 0 {
				discardedTupleBytes += len(ts.streamData)
			} else {
				discardedTupleBytes += len(ts.streamData) - len(ts.headerData) + ts.streamedBytes
			}
			ts.logger.Debugw("Terminate tuple streamer",
				"Provided bytes", streamedTupleBytes, "Discarded bytes", discardedTupleBytes)
			_ = ts.pipeWriter.Close()
			wg.Done()
		}()
		pipeWriterReady := make(chan struct{})
		go func() {
			err := ts.pipeWriter.Open()
			if err != nil {
				errCh <- err
				return
			}
			close(pipeWriterReady)
		}()
		select {
		case <-terminateCh:
			return
		case <-pipeWriterReady:
		}
		streamerErrorCh := make(chan error, 1)
		jobsDoneCh := make(chan struct{}, 2)
		go ts.bufferData(terminateCh, streamerErrorCh, jobsDoneCh)
		go ts.writeDataToPipe(terminateCh, jobsDoneCh)
		select {
		case <-terminateCh:
		case <-jobsDoneCh:
		case err := <-streamerErrorCh:
			errCh <- err
		}
		return
	}()
}

func (ts *CastorTupleStreamer) bufferData(terminateCh chan struct{}, streamerErrorCh chan error, doneCh chan struct{}) {
	defer func() {
		ts.logger.Debug("Buffer job done")
		doneCh <- struct{}{}
	}()
	for {
		select {
		case <-terminateCh:
			return
		case <-ts.streamerDoneCh:
			return
		case <-ts.fetchTuplesCh:
			ts.bufferLckCh <- struct{}{}
			tupleData, err := ts.getTupleData()
			if err == nil {
				ts.tupleBufferCh <- tupleData
			}
			<-ts.bufferLckCh
			if err != nil {
				ts.logger.Debugf("Error fetching tuples: %v", err)
				streamerErrorCh <- err
				return
			}
		}
	}
}

func (ts *CastorTupleStreamer) getTupleData() ([]byte, error) {
	requestID := uuid.NewMD5(ts.baseRequestID, []byte(strconv.Itoa(ts.requestCycle)))
	ts.requestCycle++
	tupleList, err := ts.castorClient.GetTuples(ts.stockSize, ts.tupleType, requestID)
	if err != nil {
		return nil, err
	}
	ts.logger.Debugw("Fetched new tuples from Castor", "RequestID", requestID)
	tupleData, err := ts.tupleListToByteArray(tupleList)
	if err != nil {
		return nil, fmt.Errorf("error parsing received tuple list: %v", err)
	}
	return tupleData, nil
}

// writeDataToPipe pulls more tuples from Castor if required and writes the data to the pipe
func (ts *CastorTupleStreamer) writeDataToPipe(terminateCh chan struct{}, doneCh chan struct{}) {
	defer func() {
		ts.logger.Debug("Write job done")
		doneCh <- struct{}{}
	}()
	for {
		select {
		case <-terminateCh:
			return
		case <-ts.streamerDoneCh:
			return
		default:
			if ts.streamData == nil || len(ts.streamData) == 0 {
				select {
				case <-terminateCh:
					return
				case <-ts.streamerDoneCh:
					return
				case tuples := <-ts.tupleBufferCh:
					ts.streamData = append(ts.streamData, tuples...)
					ts.fetchTuplesCh <- struct{}{}
				}
			}
			c, err := ts.pipeWriter.Write(ts.streamData)
			ts.streamData = ts.streamData[c:]
			ts.streamedBytes += c
			if err != nil {
				// pipe error (most likely "broken pipe") is considered to indicate the computation to be
				// finished and therefore terminate the streamer, but won't cause the tuple streamer to an errant
				// termination . In case the pipe was closed because of a computation error this will be reported by
				// the MPC execution itself
				// in all other cases the tuple streamer will retry
				if errors.Is(err, syscall.EPIPE) {
					ts.logger.Debugw("Received pipe error for tuple stream", "Error", err)
					return
				}
				ts.logger.Debugf("Pipe broke while streaming: %v", err.Error())
			}
		}
	}
}

// tupleListToByteArray converts a given list of tuple to a byte array
func (ts *CastorTupleStreamer) tupleListToByteArray(tl *castor.TupleList) ([]byte, error) {
	var result []byte
	for _, tuple := range tl.Tuples {
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

// generateHeader returns the file header for the given protocol and spdz runtime configuration
func generateHeader(sp castor.SPDZProtocol, conf *SPDZEngineTypedConfig) ([]byte, error) {
	switch sp {
	case castor.SPDZGfp:
		return generateGfpHeader(conf.Prime), nil
	case castor.SPDZGf2n:
		return generateGf2nHeader(conf.Gf2nBitLength, conf.Gf2nStorageSize), nil
	}
	return nil, errors.New("unsupported spdz protocol " + sp.Descriptor)
}

func generateGfpHeader(prime big.Int) []byte {
	descriptor := []byte(castor.SPDZGfp.Descriptor)
	primeBytes := prime.Bytes()
	primeByteLength := len(primeBytes)
	totalSizeInBytes := uint64(len(descriptor) + 1 + 4 + primeByteLength)

	var result []byte

	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, totalSizeInBytes)
	result = append(result, bytes...)      // Total length to follow (e.g. 29 bytes)
	result = append(result, descriptor...) // "SPDZ gfp"
	result = append(result, byte(0))       // Signum (0 == positive)

	bytes = make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(primeByteLength))
	result = append(result, bytes...)      // Prime length to follow (e.g. 16 byte == 128 bit)
	result = append(result, primeBytes...) // The prime itself

	return result
}

func generateGf2nHeader(bitLength int32, storageSize int32) []byte {
	protocol := []byte(castor.SPDZGf2n.Descriptor) // e.g. "SPDZ gf2n"

	var domain []byte
	storageSizeData := make([]byte, 8)
	binary.LittleEndian.PutUint32(storageSizeData, uint32(storageSize))
	nValue := make([]byte, 4)
	binary.LittleEndian.PutUint32(nValue, uint32(bitLength))
	domain = append(domain, storageSizeData...) // e.g. 8
	domain = append(domain, nValue...)          // e.g. 40

	totalSizeInBytes := uint64(len(protocol) + len(domain))
	size := make([]byte, 8)
	binary.LittleEndian.PutUint64(size, totalSizeInBytes)

	var result []byte
	result = append(result, size...)     // Total length to follow (e.g. 29 bytes)
	result = append(result, protocol...) // e.g. "SPDZ gf2n"
	result = append(result, domain...)   // e.g. 40

	return result
}

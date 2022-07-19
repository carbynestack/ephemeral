//
// Copyright (c) 2022 - for information on the respective copyright owner
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
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
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
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("Error deleting existing Tuple file: %v\n", err)
	}
	err = unix.Mkfifo(filePath, 0666)
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
	tupleFile     *os.File
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

// Open opens a file as write only pipe. This function should be called within a go routine as opening a pipe as
// write-only is a blocking call until opposing side opens the file to read.
//
// This is especially important when streaming tuples to MP-SPDZ, as it does open only those tuple files that are
// actually required for the current computation.
func (tpw *TuplePipeWriter) Open() error {
	var err error
	tpw.tupleFile, err = os.OpenFile(tpw.tupleFilePath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return fmt.Errorf("Error opening file: %v\n", err)
	}
	tpw.logger.Debugw("Pipe writer connected")
	return nil
}

// Close calls os.File.Close() on the tuple file pipe
func (tpw *TuplePipeWriter) Close() error {
	return tpw.tupleFile.Close()
}

// TupleStreamer is an interface.
type TupleStreamer interface {
	StartStreamTuples(terminateCh chan struct{}, errCh chan error, wg *sync.WaitGroup)
}

const defaultWriteDeadline = 5 * time.Second

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
	pipeWriter, err := pipeWriterFactory(loggerWithContext, filePath, defaultWriteDeadline)
	if err != nil {
		return nil, fmt.Errorf("error creating pipe writer: %v", err)
	}
	headerData := generateHeader(tt.SpdzProtocol, conf)
	loggerWithContext.Debugw(fmt.Sprintf("Generated tuple file header %x", headerData))
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
	logger        *zap.SugaredLogger
	pipeWriter    PipeWriter
	tupleType     castor.TupleType
	stockSize     int32
	castorClient  castor.AbstractClient
	baseRequestID uuid.UUID
	requestCycle  int
	headerData    []byte
	streamData    []byte
	streamedBytes int
}

// StartStreamTuples repeatedly downloads a given type of tuples from castor and streams it to the according file as
// required by MP-SPDZ
func (ts *CastorTupleStreamer) StartStreamTuples(terminateCh chan struct{}, errCh chan error, wg *sync.WaitGroup) {
	ts.streamData = append(ts.streamData, ts.headerData...)
	pipeWriterReady := make(chan struct{})
	go func() {
		defer func() {
			var streamedTupleBytes, discardedTupleBytes int
			if ts.streamedBytes > len(ts.headerData) {
				streamedTupleBytes = ts.streamedBytes - len(ts.headerData)
			}
			if streamedTupleBytes > 0 {
				discardedTupleBytes = len(ts.streamData)
			}
			if streamedTupleBytes > 0 || discardedTupleBytes > 0 {
				ts.logger.Debugw("Terminate tuple streamer.",
					"Provided bytes", streamedTupleBytes, "Discarded bytes", discardedTupleBytes)
			}
			_ = ts.pipeWriter.Close()
			wg.Done()
		}()
		go func() {
			err := ts.pipeWriter.Open()
			if err != nil {
				errCh <- err
				return
			}
			close(pipeWriterReady)
		}()
		for {
			select {
			case <-terminateCh:
				return
			case <-pipeWriterReady:
				err := ts.writeDataToPipe()
				if err != nil {
					if errors.Is(err, syscall.EPIPE) {
						// pipe error (most likely "broken pipe") is considered to indicate the computation to be
						// terminated and therefore won't cause the tuple streamer to an errant termination . In case
						// the pipe was closed because of a computation error this will be reported by the MPC execution
						// itself
						ts.logger.Debugw("received pipe error for tuple stream", "Error", err)
						return
					}
					errCh <- err
					return
				}
			}
		}
	}()
}

// writeDataToPipe pulls more tuples from Castor if required and the data to the pipe
func (ts *CastorTupleStreamer) writeDataToPipe() error {
	if ts.streamData == nil || len(ts.streamData) == 0 {
		requestID := uuid.NewMD5(ts.baseRequestID, []byte(strconv.Itoa(ts.requestCycle)))
		ts.requestCycle++
		tupleList, err := ts.castorClient.GetTuples(ts.stockSize, ts.tupleType, requestID)
		if err != nil {
			return err
		}
		ts.logger.Debugw("Fetched new tuples from Castor", "RequestID", requestID)
		ts.streamData, err = ts.tupleListToByteArray(tupleList)
	}
	c, err := ts.pipeWriter.Write(ts.streamData)
	if err != nil {
		// if pipe error occurred it is most likely a "broken pipe" indicating file has been closed on opposing side
		// tuple streamer will terminate in this case as computation is considered terminated and tuple streamer is no
		// longer required.
		// in all other cases the tuple streamer will retry
		if errors.Is(err, syscall.EPIPE) {
			return err
		} else {
			ts.logger.Errorw(err.Error())
		}
	}
	ts.streamData = ts.streamData[c:]
	ts.streamedBytes += c
	return nil
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
func generateHeader(sp castor.SPDZProtocol, conf *SPDZEngineTypedConfig) []byte {
	switch sp {
	case castor.SpdzGfp:
		return generateGfpHeader(conf.Prime)
	case castor.SpdzGf2n:
		return generateGf2nHeader(conf.Gf2nBitLength)
	}
	panic("Unsupported spdz protocol " + sp.Descriptor)
}

func generateGfpHeader(prime big.Int) []byte {
	descriptor := []byte(castor.SpdzGfp.Descriptor)
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

func generateGf2nHeader(bitLength int32) []byte {
	protocol := []byte(castor.SpdzGf2n.Descriptor) // e.g. "SPDZ gf2n"

	var domain []byte
	storageSize := make([]byte, 8)
	binary.LittleEndian.PutUint32(storageSize, uint32(8))
	nValue := make([]byte, 4)
	binary.LittleEndian.PutUint32(nValue, uint32(bitLength))
	domain = append(domain, storageSize...) // e.g. 8
	domain = append(domain, nValue...)      // e.g. 40

	totalSizeInBytes := uint64(len(protocol) + len(domain))
	size := make([]byte, 8)
	binary.LittleEndian.PutUint64(size, totalSizeInBytes)

	var result []byte
	result = append(result, size...)     // Total length to follow (e.g. 29 bytes)
	result = append(result, protocol...) // e.g. "SPDZ gf2n"
	result = append(result, domain...)   // e.g. 40

	return result
}

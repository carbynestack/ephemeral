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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/carbynestack/ephemeral/pkg/castor"
	. "github.com/carbynestack/ephemeral/pkg/types"
)

type PipeWriterFactory func(*zap.SugaredLogger, string, time.Duration) (PipeWriter, error)

func DefaultPipeWriterFactory(l *zap.SugaredLogger, filePath string, writeDeadline time.Duration) (PipeWriter, error) {
	pr, err := NewTuplePipeWriter(l, filePath, writeDeadline)
	return pr, err
}

type PipeWriter interface {
	Open() error
	Write([]byte) (int, error)
	Close() error
}

func NewTuplePipeWriter(l *zap.SugaredLogger, filePath string, writeDeadline time.Duration) (*TuplePipeWriter, error) {
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("Error deleting existing Tuple file: %v\n", err)
	}
	err = os.MkdirAll(filePath[0:strings.LastIndex(filePath, "/")], 0755)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("Error creating directory path: %v\n", err)
	}
	err = unix.Mkfifo(filePath, 0666)
	if err != nil {
		return nil, fmt.Errorf("Error creating pipe: %v\n", err)
	}
	return &TuplePipeWriter{
		tupleFilePath: filePath,
		writeDeadline: writeDeadline,
		logger:        l,
	}, nil
}

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
	tpw.logger.Debugw("Pipe writer connected", "filePath", tpw.tupleFilePath)
	return nil
}

// Close calls os.File.Close() on the tuple file pipe
func (tpw *TuplePipeWriter) Close() error {
	return tpw.tupleFile.Close()
}

// TupleStreamer is an interface.
type TupleStreamer interface {
	StartStreamTuples(chan struct{}, chan error, *sync.WaitGroup)
}

const tupleBaseFolder = "Player-Data"
const defaultWriteDeadline = 5 * time.Second

func TupleFilePath(tt castor.TupleType, config *SPDZEngineTypedConfig) string {
	gfpFileFormat := fmt.Sprintf("%s/%d-p-%d/%%s-p-P%d-T0", tupleBaseFolder, config.PlayerCount, config.Prime.BitLen(), config.PlayerID)
	gf2nFileFormat := fmt.Sprintf("%s/%d-2-%d/%%s-p-P%d-T0", tupleBaseFolder, config.PlayerCount, config.SecurityParameter, config.PlayerID)
	switch tt {
	case castor.BitGfp:
		return fmt.Sprintf(gfpFileFormat, "Bits")
	case castor.InputMaskGfp:
		return fmt.Sprintf(gfpFileFormat, "Inputs")
	case castor.InverseTupleGfp:
		return fmt.Sprintf(gfpFileFormat, "Inverses")
	case castor.SquareTupleGfp:
		return fmt.Sprintf(gfpFileFormat, "Squares")
	case castor.MultiplicationTripleGfp:
		return fmt.Sprintf(gfpFileFormat, "Triples")
	case castor.BitGf2n:
		return fmt.Sprintf(gf2nFileFormat, "Bits")
	case castor.InputMaskGf2n:
		return fmt.Sprintf(gf2nFileFormat, "Inputs")
	case castor.InverseTupleGf2n:
		return fmt.Sprintf(gf2nFileFormat, "Inverses")
	case castor.SquareTupleGf2n:
		return fmt.Sprintf(gf2nFileFormat, "Squares")
	case castor.MultiplicationTripleGf2n:
		return fmt.Sprintf(gf2nFileFormat, "Triples")
	}
	panic("Unsupported tuple type " + tt)
}

// NewCastorTupleStreamer returns a new instance of castor tuple streamer.
func NewCastorTupleStreamer(l *zap.SugaredLogger, tt castor.TupleType, conf *SPDZEngineTypedConfig, gameID string) (*CastorTupleStreamer, error) {
	ts, err := NewCastorTupleStreamerWithWriterFactory(l, tt, conf, gameID, DefaultPipeWriterFactory)
	return ts, err
}

// NewCastorTupleStreamerWithWriterFactory returns a new instance of castor tuple streamer.
func NewCastorTupleStreamerWithWriterFactory(l *zap.SugaredLogger, tt castor.TupleType, conf *SPDZEngineTypedConfig, gameID string, pipeWriterFactory PipeWriterFactory) (*CastorTupleStreamer, error) {
	tupleFilePath := TupleFilePath(tt, conf)
	pipeWriter, err := pipeWriterFactory(l, tupleFilePath, defaultWriteDeadline)
	if err != nil {
		return nil, fmt.Errorf("error creating pipe writer: %v", err)
	}
	headerData := generateHeader(tt, &conf.Prime)
	l.Debugw(fmt.Sprintf("Generated tuple file header %x", headerData), TupleType, tt, "Prime", conf.Prime.Text(10))
	gameUUID, err := uuid.Parse(gameID)
	if err != nil {
		return nil, fmt.Errorf("error parsing gameID: %v", err)
	}
	return &CastorTupleStreamer{
		logger:        l,
		pipeWriter:    pipeWriter,
		tupleType:     tt,
		stockSize:     conf.TupleStock,
		castorClient:  conf.CastorClient,
		baseRequestID: uuid.NewMD5(gameUUID, []byte(tt)),
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

// StartStreamTuples repeatedly downloads a given type of tuples from castor and streams it to the according file as required by MP-SPDZ
func (ts *CastorTupleStreamer) StartStreamTuples(terminate chan struct{}, errCh chan error, wg *sync.WaitGroup) {
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
				ts.logger.Debugw("Terminate tuple streamer.", TupleType, ts.tupleType, "Provided bytes", streamedTupleBytes, "Discarded bytes", discardedTupleBytes)
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
			case <-terminate:
				return
			case <-pipeWriterReady:
				err := ts.writeDataToPipe()
				if err != nil {
					if errors.Is(err, syscall.EPIPE) {
						// pipe error (most likely "broken pipe") is considered to indicate the computation to be
						// terminated and therefore won't cause the tuple streamer to an errant termination . In case
						// the pipe was closed because of a computation error this will be reported by the MPC execution
						// itself
						ts.logger.Debugw("received pipe error for tuple stream", TupleType, ts.tupleType, "Error", err)
						return
					}
					errCh <- err
					return
				}
			}
		}
	}()
}

func (ts *CastorTupleStreamer) writeDataToPipe() error {
	if ts.streamData == nil || len(ts.streamData) == 0 {
		requestID := uuid.NewMD5(ts.baseRequestID, []byte(strconv.Itoa(ts.requestCycle))).String()
		ts.requestCycle++
		tupleList, err := ts.castorClient.GetTuples(ts.stockSize, ts.tupleType, requestID)
		if err != nil {
			return err
		}
		ts.logger.Debugw("Fetched new tuples from Castor", TupleType, ts.tupleType, "RequestID", requestID)
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
			ts.logger.Errorw(err.Error(), TupleType, ts.tupleType)
		}
	}
	ts.streamData = ts.streamData[c:]
	ts.streamedBytes += c
	return nil
}

func (ts *CastorTupleStreamer) tupleListToByteArray(tl castor.TupleList) ([]byte, error) {
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

func protocolDescriptorFor(tt castor.TupleType) string {
	var typeString string = fmt.Sprint(tt)
	return fmt.Sprint("SPDZ ", strings.ToLower(typeString[strings.LastIndex(typeString, "_")+1:]))
}

func generateHeader(tt castor.TupleType, prime *big.Int) []byte {
	descriptor := []byte(protocolDescriptorFor(tt))
	primeBytes := prime.Bytes()
	primeByteLength := len(primeBytes)
	totalSizeInBytes := uint64(len(descriptor) + 1 + 4 + primeByteLength)

	var result []byte

	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, totalSizeInBytes)
	result = append(result, bytes...)      // Total length to follow (e.g. 29 bytes)
	result = append(result, descriptor...) // e.g. "SPDZ gfp"
	result = append(result, byte(0))       //Signum (0 == positive)

	bytes = make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, uint32(primeByteLength))
	result = append(result, bytes...)      // Prime length to follow (e.g. 16 byte == 128 bit)
	result = append(result, primeBytes...) // The prime itself

	return result
}
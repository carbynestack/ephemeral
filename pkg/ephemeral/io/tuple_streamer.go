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
	"fmt"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/carbynestack/ephemeral/pkg/castor"
	. "github.com/carbynestack/ephemeral/pkg/types"
)

type PipeWriterFactory func(string, time.Duration) (PipeWriter, error)

func DefaultPipeWriterFactory(filePath string, writeDeadline time.Duration) (PipeWriter, error) {
	pr, err := NewTuplePipeWriter(filePath, writeDeadline)
	return pr, err
}

type PipeWriter interface {
	Write([]byte) (int, error)
	Close() error
}

func NewTuplePipeWriter(filePath string, writeDeadline time.Duration) (*TuplePipeWriter, error) {
	_ = os.Remove(filePath)
	err := unix.Mkfifo(filePath, 0666)
	if err != nil {
		return nil, fmt.Errorf("Error creating pipe: %v\n", err)
	}
	file, err := os.OpenFile(filePath, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		return nil, fmt.Errorf("Error Opening File: %v\n", err)
	}
	return &TuplePipeWriter{
		tupleFile:     file,
		writeDeadline: writeDeadline,
	}, nil
}

type TuplePipeWriter struct {
	tupleFile     *os.File
	writeDeadline time.Duration
}

func (tpw *TuplePipeWriter) Write(data []byte) (int, error) {
	deadline := time.Now().Add(tpw.writeDeadline)
	err := tpw.tupleFile.SetWriteDeadline(deadline)
	if err != nil {
		return 0, fmt.Errorf("Error setting write deadline: %v", err)
	}
	c, err := tpw.tupleFile.Write(data)
	if err != nil {
		return 0, fmt.Errorf("Error wrting data to pipe: %v", err)
	}
	return c, nil
}

func (tpw *TuplePipeWriter) Close() error {
	return tpw.tupleFile.Close()
}

// TupleStreamer is an interface.
type TupleStreamer interface {
	StartStreamTuples(chan struct{}, chan error, *sync.WaitGroup)
}

const tupleBaseFolder = "Player-Data"
const defaultWriteDeadline = 5 * time.Second

func TupleFilePath(t castor.TupleType, config *SPDZEngineTypedConfig) string {
	gfpFileFormat := fmt.Sprintf("%s/%d-p-%d/%%s-p-P%d", tupleBaseFolder, config.PlayerCount, config.Prime.BitLen(), config.PlayerID)
	gf2nFileFormat := fmt.Sprintf("%s/%d-2-%d/%%s-p-P%d", tupleBaseFolder, config.PlayerCount, config.SecurityParameter, config.PlayerID)
	switch t {
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
	panic("Unsupported tuplye type " + t)
}

// NewCastorTupleStreamer returns a new instance of castor tuple streamer.
func NewCastorTupleStreamer(l *zap.SugaredLogger, t castor.TupleType, conf *SPDZEngineTypedConfig, gameID string) (*CastorTupleStreamer, error) {
	ts, err := NewCastorTupleStreamerWithWriterFactory(l, t, conf, gameID, DefaultPipeWriterFactory)
	return ts, err
}

// NewCastorTupleStreamerWithWriterFactory returns a new instance of castor tuple streamer.
func NewCastorTupleStreamerWithWriterFactory(l *zap.SugaredLogger, t castor.TupleType, conf *SPDZEngineTypedConfig, gameID string, pipeWriterFactory PipeWriterFactory) (*CastorTupleStreamer, error) {
	tupleFilePath := TupleFilePath(t, conf)
	pipeWriter, err := pipeWriterFactory(tupleFilePath, defaultWriteDeadline)
	if err != nil {
		return nil, fmt.Errorf("Error creating pipe writer: %v", err)
	}
	headerData := generateHeader(t, &conf.Prime)
	l.Debugw(fmt.Sprintf("Generated tuple file header %x", headerData), "TupleType", t, "Prime", conf.Prime.Text(10))
	gameUUID, err := uuid.Parse(gameID)
	if err != nil {
		return nil, fmt.Errorf("Error parsing gameID: %v", err)
	}
	return &CastorTupleStreamer{
		logger:        l,
		pipeWriter:    pipeWriter,
		tupleType:     t,
		stockSize:     conf.TupleStock,
		castorClient:  conf.CastorClient,
		baseRequestID: uuid.NewMD5(gameUUID, []byte(t)),
		streamData:    headerData,
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
	streamData    []byte
}

// StreamTuples repeatedly downloads a given type of tuples from castor and streams it to the according file as required by MP-SPDZ
func (ts *CastorTupleStreamer) StartStreamTuples(terminate chan struct{}, errCh chan error, wg *sync.WaitGroup) {
	defer func() {
		ts.logger.Debugw(fmt.Sprintf("Done streaming tuples, discarding %d bytes.", len(ts.streamData)), "TupleType", ts.tupleType)
		ts.pipeWriter.Close()
		wg.Done()
	}()
	for {
		select {
		case <-terminate:
			return
		default:
			err := ts.writeDataToPipe()
			if err != nil {
				errCh <- err
				return
			}
		}
	}
}

func (ts *CastorTupleStreamer) writeDataToPipe() error {
	if ts.streamData == nil || len(ts.streamData) == 0 {
		requestID := uuid.NewMD5(ts.baseRequestID, []byte(strconv.Itoa(ts.requestCycle))).String()
		ts.requestCycle++
		tupleList, err := ts.castorClient.GetTuples(ts.stockSize, ts.tupleType, requestID)
		if err != nil {
			return err
		}
		ts.streamData, err = ts.tupleListToByteArray(tupleList)
	}
	c, err := ts.pipeWriter.Write(ts.streamData)
	if err != nil {
		ts.logger.Errorw(err.Error(), "TupleType", ts.tupleType)
	}
	ts.streamData = ts.streamData[c:]
	ts.logger.Debug(fmt.Sprintf("Wrote %d %s to file %s", c, ts.tupleType, ts.pipeWriter.(*TuplePipeWriter).tupleFile.Name()))
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

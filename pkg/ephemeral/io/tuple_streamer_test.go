//
// Copyright (c) 2022 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package io

import (
	"errors"
	"math/big"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/carbynestack/ephemeral/pkg/castor"
	. "github.com/carbynestack/ephemeral/pkg/types"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const tupleStock = 42

var _ = Describe("Tuple Streamer", func() {
	var (
		pipeWriter *FakeConsumingPipeWriter
		ts         CastorTupleStreamer
		cc         *FakeCastorClient
	)
	BeforeEach(func() {
		pipeWriter = &FakeConsumingPipeWriter{
			isClosed: false,
		}
		cc = &FakeCastorClient{}
		ts = CastorTupleStreamer{
			logger:       zap.NewNop().Sugar(),
			pipeWriter:   pipeWriter,
			tupleType:    castor.BitGfp,
			stockSize:    tupleStock,
			castorClient: cc,
		}
	})

	Context("when streaming tuples", func() {
		Context("when fetching tuples from castor", func() {
			Context("when castor client returns an error", func() {
				It("writes error to error channel and stops", func() {
					terminate := make(chan struct{})
					errCh := make(chan error, 1)
					wg := sync.WaitGroup{}
					wg.Add(1)
					ts.castorClient = &BrokenDownloadCastorClient{}
					go ts.StartStreamTuples(terminate, errCh, &wg)
					wg.Wait()
					close(terminate)
					close(errCh)
					Expect(pipeWriter.isClosed).To(BeTrue())
					Expect(<-errCh).NotTo(BeNil())
				})
			})
		})
	})

	Context("when creating a new instance of castor tuple streamer", func() {
		It("sets required parameters and returns a new instance", func() {
			logger := zap.NewNop().Sugar()
			tupleType := castor.BitGfp
			var prime big.Int
			prime.SetString("198766463529478683931867765928436695041", 10)
			conf := &SPDZEngineTypedConfig{
				TupleStock:   42,
				CastorClient: &FakeCastorClient{},
				Prime:        prime,
			}
			gameID, _ := uuid.NewRandom()
			fakePipeWriterFactory := func(fp string, wd time.Duration) (PipeWriter, error) {
				return &FakeConsumingPipeWriter{
					filePath: fp,
				}, nil
			}
			ts, _ := NewCastorTupleStreamerWithWriterFactory(logger, tupleType, conf, gameID.String(), fakePipeWriterFactory)
			Expect(ts.logger).To(Equal(logger))
			Expect(ts.pipeWriter.(*FakeConsumingPipeWriter).filePath).To(Equal("Player-Data/0-p-128/Bits-p-P0"))
			Expect(ts.tupleType).To(Equal(tupleType))
			Expect(ts.stockSize).To(Equal(conf.TupleStock))
			Expect(ts.castorClient).To(Equal(conf.CastorClient))
			Expect(ts.baseRequestID).To(Equal(uuid.NewMD5(gameID, []byte(tupleType))))
			Expect(ts.streamData).To(Equal(generateHeader(tupleType, &prime)))
			Expect(ts.requestCycle).To(Equal(0))
		})
	})

	Context("when building protocol descriptor", func() {
		It("returns correct string for gfp txpes", func() {
			gfpTypes := []castor.TupleType{castor.BitGfp, castor.MultiplicationTripleGfp, castor.InputMaskGfp, castor.InverseTupleGfp, castor.SquareTupleGfp}
			for _, tt := range gfpTypes {
				Expect(protocolDescriptorFor(tt)).To(Equal("SPDZ gfp"))
			}
		})

		It("returns correct string for gf2n txpes", func() {
			gfpTypes := []castor.TupleType{castor.BitGf2n, castor.MultiplicationTripleGf2n, castor.InputMaskGf2n, castor.InverseTupleGf2n, castor.SquareTupleGf2n}
			for _, tt := range gfpTypes {
				Expect(protocolDescriptorFor(tt)).To(Equal("SPDZ gf2n"))
			}
		})
	})

	Context("when generating tuple file header", func() {
		It("returns correct header", func() {
			expecteHeader := []byte{29, 0, 0, 0, 0, 0, 0, 0, 83, 80, 68, 90, 32, 103, 102, 112, 0, 16, 0, 0, 0, 149, 137, 7, 69, 143, 33, 54, 134, 27, 215, 85, 74, 36, 52, 0, 1}
			var prime big.Int
			prime.SetString("198766463529478683931867765928436695041", 10)
			Expect(generateHeader(castor.BitGfp, &prime)).To(Equal(expecteHeader))
		})
	})
})

type FakeConsumingPipeWriter struct {
	filePath string
	isClosed bool
}

func (ff *FakeConsumingPipeWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

func (ff *FakeConsumingPipeWriter) Close() error {
	ff.isClosed = true
	return nil
}

type FakeCastorClient struct {
}

func (fcc *FakeCastorClient) GetTuples(int32, castor.TupleType, string) (castor.TupleList, error) {
	return castor.TupleList{}, nil
}

type BrokenDownloadCastorClient struct{}

func (fcc *BrokenDownloadCastorClient) GetTuples(int32, castor.TupleType, string) (castor.TupleList, error) {
	return castor.TupleList{}, errors.New("Fetching tuples failed")
}

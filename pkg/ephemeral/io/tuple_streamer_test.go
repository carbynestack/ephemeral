//
// Copyright (c) 2022-2023 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//

package io

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/carbynestack/ephemeral/pkg/utils"
	"io/ioutil"
	"math/big"
	"os"
	"strconv"
	"sync"
	"syscall"
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
	var defaultWriteDeadline = 5 * time.Second
	Context("when StartStreamTuples", func() {
		var (
			fcpw      *FakeConsumingPipeWriter
			ts        *CastorTupleStreamer
			cc        *FakeCastorClient
			terminate chan struct{}
			errCh     chan error
			wg        *sync.WaitGroup
		)
		BeforeEach(func() {
			terminate = make(chan struct{})
			errCh = make(chan error, 1)
			wg = &sync.WaitGroup{}
			fcpw = &FakeConsumingPipeWriter{
				isClosed: false,
			}
			cc = &FakeCastorClient{}
			ts = &CastorTupleStreamer{
				logger:       zap.NewNop().Sugar(),
				pipeWriter:   fcpw,
				tupleType:    castor.BitGfp,
				stockSize:    tupleStock,
				castorClient: cc,
			}
		})
		Context("when opening pipe fails", func() {
			It("return error", func() {
				expectedError := errors.New("expected error")
				fcpw.openError = expectedError
				wg.Add(1)
				ts.StartStreamTuples(terminate, errCh, wg)
				var err error
				select {
				case err = <-errCh:
				case <-time.After(5 * time.Second):
					Fail("test timed out")
				}
				close(terminate)
				close(errCh)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			})
		})
		Context("when opening pipe blocks", func() {
			It("return on terminate", func() {
				fbwpw := &FakeBlockingWritePipeWriter{}
				ts.pipeWriter = fbwpw
				wg.Add(1)
				ts.StartStreamTuples(terminate, errCh, wg)
				close(terminate)
				wg.Wait()
				close(errCh)
				err := <-errCh
				Expect(err).To(BeNil())
				Expect(fbwpw.writeCalled).To(BeFalse())
			})
		})
		Context("when streamData is empty", func() {
			Context("when castor client returns an error", func() {
				BeforeEach(func() {
					ts.castorClient = &BrokenDownloadCastorClient{}
				})
				It("writes error to error channel and stops", func() {
					wg.Add(1)
					ts.StartStreamTuples(terminate, errCh, wg)
					wg.Wait()
					close(terminate)
					close(errCh)
					Expect(fcpw.isClosed).To(BeTrue())
					Expect(<-errCh).NotTo(BeNil())
				})
			})
			Context("when tuples fetched successfully", func() {
				shareValue := "value"
				shareMac := "value"
				share := castor.Share{
					Value: base64.StdEncoding.EncodeToString([]byte(shareValue)),
					Mac:   base64.StdEncoding.EncodeToString([]byte(shareMac)),
				}
				shares := make([]castor.Share, 1)
				shares[0] = share
				tuples := make([]castor.Tuple, 1)
				tuples[0] = castor.Tuple{Shares: shares}
				initialStreamData := []byte(shareValue + shareMac)
				BeforeEach(func() {
					cc.TupleData = initialStreamData
				})
				Context("when writing data to pipe fails with broken pipe", func() {
					var expectedError = syscall.EPIPE
					BeforeEach(func() {
						fcpw.writeError = expectedError
					})
					It("return without error", func() {
						wg.Add(1)
						ts.StartStreamTuples(terminate, errCh, wg)
						wg.Wait()
						close(terminate)
						close(errCh)
						Expect(fcpw.isClosed).To(BeTrue())
						Expect(<-errCh).NotTo(HaveOccurred())
					})
				})
				Context("when data is partially written to pipe", func() {
					var fpcpw *FakePartialConsumingFailSecondCallPipeWriter
					BeforeEach(func() {
						fpcpw = &FakePartialConsumingFailSecondCallPipeWriter{writeLess: 2}
						ts.pipeWriter = fpcpw
					})
					It("update fields accordingly", func() {
						wg.Add(1)
						ts.StartStreamTuples(terminate, errCh, wg)
						wg.Wait()
						close(terminate)
						close(errCh)
						Expect(ts.streamedBytes).To(Equal(len(initialStreamData) - fpcpw.writeLess))
						Expect(ts.streamData).To(Equal(initialStreamData[ts.streamedBytes:]))
					})
				})
			})
		})
	})

	Context("when creating a new instance of castor tuple streamer", func() {
		It("sets required parameters and returns a new instance", func() {
			logger := zap.NewNop().Sugar()
			tupleType := castor.BitGfp
			threadNr := 1
			var prime big.Int
			prime.SetString("198766463529478683931867765928436695041", 10)
			conf := &SPDZEngineTypedConfig{
				TupleStock:   42,
				CastorClient: &FakeCastorClient{},
				Prime:        prime,
			}
			playerDataDir := "Player-Data/0-p-128/"
			gameID, _ := uuid.NewRandom()
			fakePipeWriterFactory := func(l *zap.SugaredLogger, filePath string, wd time.Duration) (PipeWriter, error) {
				return &FakeConsumingPipeWriter{
					filePath: filePath,
				}, nil
			}
			ts, _ := NewCastorTupleStreamerWithWriterFactory(logger, tupleType, conf, playerDataDir, gameID, threadNr, fakePipeWriterFactory)
			Expect(ts.logger).To(Equal(logger))
			Expect(ts.pipeWriter.(*FakeConsumingPipeWriter).filePath).To(Equal("Player-Data/0-p-128/Bits-p-P0-T1"))
			Expect(ts.tupleType).To(Equal(tupleType))
			Expect(ts.stockSize).To(Equal(conf.TupleStock))
			Expect(ts.castorClient).To(Equal(conf.CastorClient))
			Expect(ts.baseRequestID).To(Equal(uuid.NewMD5(gameID, []byte(tupleType.Name+strconv.Itoa(threadNr)))))
			expectedHeader, _ := generateHeader(tupleType.SpdzProtocol, conf)
			Expect(ts.headerData).To(Equal(expectedHeader))
			Expect(ts.requestCycle).To(Equal(0))
		})
		Context("when header cannot be generated", func() {
			Context("when protocol is unsupported", func() {
				It("return error", func() {
					logger := zap.NewNop().Sugar()
					conf := &SPDZEngineTypedConfig{}
					gameID, _ := uuid.NewRandom()
					unsupportedProtocol := castor.SPDZProtocol{Shorthand: "u", Descriptor: "unsupported"}
					unsupportedTupleType := castor.TupleType{Name: "unsupported", PreprocessingName: "unknown", SpdzProtocol: unsupportedProtocol}
					ts, err := NewCastorTupleStreamerWithWriterFactory(logger, unsupportedTupleType, conf, "", gameID, 0, func(*zap.SugaredLogger, string, time.Duration) (PipeWriter, error) {
						return &FakeConsumingPipeWriter{}, nil
					})
					Expect(ts).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(errors.New("error creating header: unsupported spdz protocol " + unsupportedProtocol.Descriptor)))
				})
			})
		})
		Context("when delete existing tuple file fails", func() {
			var oldFio utils.FileIO
			var mockedFio *utils.MockedFileIO
			BeforeEach(func() {
				oldFio = utils.Fio
				mockedFio = &utils.MockedFileIO{}
				utils.Fio = mockedFio
			})
			AfterEach(func() {
				utils.Fio = oldFio
			})
			It("return error", func() {
				logger := zap.NewNop().Sugar()
				tupleType := castor.BitGfp
				threadNr := 1
				var prime big.Int
				prime.SetString("198766463529478683931867765928436695041", 10)
				conf := &SPDZEngineTypedConfig{
					TupleStock:   42,
					CastorClient: &FakeCastorClient{},
					Prime:        prime,
				}
				playerDataDir := "Player-Data/0-p-128/"
				gameID, _ := uuid.NewRandom()
				expectedError := fmt.Errorf("expected error")
				mockedFio.DeleteResponse = expectedError
				cts, err := NewCastorTupleStreamer(logger, tupleType, conf, playerDataDir, gameID, threadNr)
				Expect(cts).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(fmt.Errorf("error creating pipe writer: %v",
					fmt.Errorf("error deleting existing Tuple file: %v", expectedError))))
				Expect(len(mockedFio.DeleteCalls)).To(Equal(1))
				Expect(mockedFio.DeleteCalls[0]).To(Equal("Player-Data/0-p-128/Bits-p-P0-T1"))
			})
		})
	})

	Context("when generateHeader", func() {
		Context("when protocol is SPD gfp", func() {
			It("returns correct header", func() {
				expectedHeader := []byte{29, 0, 0, 0, 0, 0, 0, 0, 83, 80, 68, 90, 32, 103, 102, 112, 0, 16, 0, 0, 0, 149, 137, 7, 69, 143, 33, 54, 134, 27, 215, 85, 74, 36, 52, 0, 1}
				var prime big.Int
				prime.SetString("198766463529478683931867765928436695041", 10)
				config := SPDZEngineTypedConfig{Prime: prime}
				Expect(generateHeader(castor.SPDZGfp, &config)).To(Equal(expectedHeader))
			})
		})
		Context("when protocol is SPD gf2n", func() {
			It("returns correct header", func() {
				expectedHeader := []byte{22, 0, 0, 0, 0, 0, 0, 0, 83, 80, 68, 90, 32, 103, 102, 50, 110, 95, 16, 0, 0, 0, 0, 0, 0, 0, 40, 0, 0, 0}
				gf2nBitLength := int32(40)
				gf2nStorageSize := int32(16)
				config := SPDZEngineTypedConfig{Gf2nBitLength: gf2nBitLength, Gf2nStorageSize: gf2nStorageSize}
				Expect(generateHeader(castor.SPDZGf2n, &config)).To(Equal(expectedHeader))
			})
		})
	})

	Context("when creating a new TuplePipeWriter", func() {
		Context("when using mocked fileIO", func() {
			var oldFio utils.FileIO
			var mockedFio *utils.MockedFileIO
			BeforeEach(func() {
				oldFio = utils.Fio
				mockedFio = &utils.MockedFileIO{}
				utils.Fio = mockedFio
			})
			AfterEach(func() {
				utils.Fio = oldFio
			})
			Context("when deleting existing pipe fails", func() {
				It("return error", func() {
					logger := zap.NewNop().Sugar()
					filePath := "tupleFilePath"
					expectedError := fmt.Errorf("expected error")
					mockedFio.DeleteResponse = expectedError
					mockedFio.CreatePipeResponse = nil

					tpw, err := NewTuplePipeWriter(logger, filePath, defaultWriteDeadline)

					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(fmt.Errorf("error deleting existing Tuple file: %v", expectedError)))
					Expect(tpw).To(BeNil())
					Expect(len(mockedFio.DeleteCalls)).To(Equal(1))
					Expect(mockedFio.DeleteCalls[0]).To(Equal(filePath))
					Expect(len(mockedFio.CreatePipeCalls)).To(Equal(0))
				})
			})
			Context("when creating named pipe fails", func() {
				It("return error", func() {
					logger := zap.NewNop().Sugar()
					filePath := "tupleFilePath"
					expectedError := fmt.Errorf("expected error")
					mockedFio.DeleteResponse = nil
					mockedFio.CreatePipeResponse = expectedError

					tpw, err := NewTuplePipeWriter(logger, filePath, defaultWriteDeadline)

					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(expectedError))
					Expect(tpw).To(BeNil())
					Expect(len(mockedFio.DeleteCalls)).To(Equal(1))
					Expect(mockedFio.DeleteCalls[0]).To(Equal(filePath))
					Expect(len(mockedFio.CreatePipeCalls)).To(Equal(1))
					Expect(mockedFio.DeleteCalls[0]).To(Equal(filePath))
				})
			})
			Context("when using the DefaultPipeWriterFactory", func() {
				It("sets required parameters and returns a new instance", func() {
					logger := zap.NewNop().Sugar()
					filePath := "tupleFilePath"
					mockedFio.DeleteResponse = nil
					mockedFio.CreatePipeResponse = nil

					tpw, err := NewTuplePipeWriter(logger, filePath, defaultWriteDeadline)

					Expect(err).NotTo(HaveOccurred())
					Expect(tpw).NotTo(BeNil())
					Expect(tpw.tupleFilePath).To(Equal(filePath))
					Expect(tpw.logger).To(Equal(logger)) //.With("FilePath", filePath)
					Expect(tpw.writeDeadline).To(Equal(defaultWriteDeadline))
					Expect(len(mockedFio.DeleteCalls)).To(Equal(1))
					Expect(mockedFio.DeleteCalls[0]).To(Equal(filePath))
					Expect(len(mockedFio.CreatePipeCalls)).To(Equal(1))
					Expect(mockedFio.CreatePipeCalls[0]).To(Equal(filePath))
				})
			})
		})
	})

	Context("when calling NewTuplePipeWriter", func() {
		Context("when using os filesystem", func() {
			It("set required parameters, create named pipe and return a new instance", func() {
				logger := zap.NewNop().Sugar()
				testFolder, _ := ioutil.TempDir("", "ephemeral_test_")
				filePath := fmt.Sprintf("%s/tuple.file", testFolder)
				_ = ioutil.WriteFile(filePath, []byte("dummy data"), 0644)
				defer func() {
					_ = os.RemoveAll(testFolder)
				}()
				tpw, err := NewTuplePipeWriter(logger, filePath, defaultWriteDeadline)

				Expect(err).NotTo(HaveOccurred())
				Expect(tpw).NotTo(BeNil())
				Expect(tpw.tupleFilePath).To(Equal(filePath))
				Expect(tpw.logger).To(Equal(logger))
				Expect(tpw.writeDeadline).To(Equal(defaultWriteDeadline))
				stats, err := os.Stat(filePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(stats.Mode() & os.ModeNamedPipe).To(Equal(os.ModeNamedPipe))
			})
		})
	})

	Context("when interacting with TuplePipeWriter", func() {
		var oldFio utils.FileIO
		var mockedFio *utils.MockedFileIO
		var tpw *TuplePipeWriter
		logger := zap.NewNop().Sugar()
		filePath := "tupleFilePath"
		BeforeEach(func() {
			oldFio = utils.Fio
			mockedFio = &utils.MockedFileIO{}
			utils.Fio = mockedFio
			tpw = &TuplePipeWriter{
				logger:        logger,
				tupleFilePath: filePath,
				writeDeadline: defaultWriteDeadline,
			}
		})
		AfterEach(func() {
			utils.Fio = oldFio
		})
		Context("when opening pipe fails", func() {
			It("return error", func() {
				expectedError := fmt.Errorf("expected error")
				mockedFio.OpenWritePipeResponse = utils.OpenWritePipeResponse{Error: expectedError}
				err := tpw.Open()
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(fmt.Errorf("error opening file: %v", expectedError)))
				Expect(len(mockedFio.OpenWritePipeCalls)).To(Equal(1))
				Expect(mockedFio.OpenWritePipeCalls[0]).To(Equal(filePath))
			})
		})
		Context("when opening pipe is successful", func() {
			It("return nil", func() {
				mockedFio.OpenWritePipeResponse = utils.OpenWritePipeResponse{Error: nil}
				err := tpw.Open()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(mockedFio.OpenWritePipeCalls)).To(Equal(1))
				Expect(mockedFio.OpenWritePipeCalls[0]).To(Equal(filePath))
			})
		})
		Context("when pipe is connected", func() {
			var pipeFile *utils.SimpleFileMock
			BeforeEach(func() {
				pipeFile = &utils.SimpleFileMock{}
				tpw.tupleFile = pipeFile
			})
			testdata := []byte("dummy data")
			Context("when writing data to pipe", func() {
				Context("when setting writeDeadline fails", func() {
					It("return error", func() {
						expectedError := fmt.Errorf("expected error")
						pipeFile.SetDeadlineError = expectedError
						_, err := tpw.Write(testdata)
						Expect(err).To(HaveOccurred())
						Expect(err).To(Equal(fmt.Errorf("error setting write deadline: %v", expectedError)))
						Expect(pipeFile.WrittenData).To(BeNil())
					})
				})
				Context("when write operation fails", func() {
					It("return error", func() {
						start := time.Now()
						expectedError := fmt.Errorf("expected error")
						pipeFile.IOError = expectedError
						_, err := tpw.Write(testdata)
						end := time.Now()
						Expect(err).To(HaveOccurred())
						Expect(err).To(Equal(expectedError))
						Expect(pipeFile.WriteDeadline).NotTo(BeNil())
						Expect(
							pipeFile.WriteDeadline.After(start.Add(defaultWriteDeadline)) &&
								pipeFile.WriteDeadline.Before(end.Add(defaultWriteDeadline))).To(BeTrue())
					})
				})
			})
			Context("when closing pipe", func() {
				Context("when close on pipe returns error", func() {
					It("return error", func() {
						expectedError := fmt.Errorf("expected error")
						pipeFile.CloseError = expectedError
						err := tpw.Close()
						Expect(err).To(HaveOccurred())
						Expect(err).To(Equal(expectedError))
					})
				})
			})
		})
	})
})

type FakeConsumingPipeWriter struct {
	filePath   string
	isClosed   bool
	writeError error
	openError  error
}

func (fcpw *FakeConsumingPipeWriter) Open() error {
	return fcpw.openError
}

func (fcpw *FakeConsumingPipeWriter) Write(data []byte) (int, error) {
	return len(data), fcpw.writeError
}

func (fcpw *FakeConsumingPipeWriter) Close() error {
	fcpw.isClosed = true
	return nil
}

type FakeBlockingWritePipeWriter struct {
	writeCalled bool
}

func (fbwpr *FakeBlockingWritePipeWriter) Open() error {
	return nil
}

func (fbwpr *FakeBlockingWritePipeWriter) Write(data []byte) (int, error) {
	fbwpr.writeCalled = true
	time.Sleep(5 * time.Second)
	return len(data), nil
}

func (fbwpr *FakeBlockingWritePipeWriter) Close() error {
	return nil
}

type FakePartialConsumingFailSecondCallPipeWriter struct {
	count     int
	writeLess int
}

func (fpcfpw *FakePartialConsumingFailSecondCallPipeWriter) Open() error {
	return nil
}

func (fpcfpw *FakePartialConsumingFailSecondCallPipeWriter) Write(data []byte) (int, error) {
	fpcfpw.count++
	if fpcfpw.count == 2 {
		return 0, syscall.EPIPE
	}
	l := len(data) - fpcfpw.writeLess
	if l < 0 {
		l = 0
	}
	return l, nil
}

func (fpcfpw *FakePartialConsumingFailSecondCallPipeWriter) Close() error {
	return nil
}

type FakeCastorClient struct {
	TupleData []byte
}

func (fcc *FakeCastorClient) GetTuples(int32, castor.TupleType, uuid.UUID) ([]byte, error) {
	tl := fcc.TupleData
	if tl == nil {
		tl = []byte{}
	}
	return tl, nil
}

type BrokenDownloadCastorClient struct{}

func (fcc *BrokenDownloadCastorClient) GetTuples(int32, castor.TupleType, uuid.UUID) ([]byte, error) {
	return []byte{}, errors.New("fetching tuples failed")
}

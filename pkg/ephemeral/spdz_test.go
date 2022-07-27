//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package ephemeral

import (
	"context"
	"errors"
	"fmt"
	"github.com/carbynestack/ephemeral/pkg/castor"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"github.com/carbynestack/ephemeral/pkg/ephemeral/io"
	. "github.com/carbynestack/ephemeral/pkg/types"
	"github.com/carbynestack/ephemeral/pkg/utils"
	"github.com/google/uuid"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var _ = Describe("Spdz", func() {

	var (
		cmder utils.Commander
	)

	BeforeEach(func() {
		cmder = utils.Commander{
			Command: "bash",
			Options: []string{"-c"},
		}
	})
	Context("compiling the user code", func() {
		var (
			fileName   string
			prepFolder string
			random     int32
		)
		BeforeEach(func() {
			rand.Seed(time.Now().UnixNano())
			random = rand.Int31()
			prepFolder, _ := ioutil.TempDir("", "ephemeral_")
			fileName = fmt.Sprintf("%s/program.mpc", prepFolder)
		})
		AfterEach(func() {
			_ = os.RemoveAll(prepFolder)
		})
		Context("writing succeeds", func() {
			It("writes the source code on the disk and runs the compiler", func() {
				s := &SPDZEngine{
					cmder:          &FakeExecutor{},
					sourceCodePath: fileName,
					logger:         zap.NewNop().Sugar(),
					config:         &SPDZEngineTypedConfig{PrepFolder: prepFolder},
				}
				conf := &CtxConfig{
					Act: &Activation{
						Code: "a",
					},
				}
				err := s.Compile(conf)
				Expect(err).NotTo(HaveOccurred())
				out, _, err := cmder.CallCMD(context.TODO(), []string{fmt.Sprintf("cat %s", s.sourceCodePath)}, "./")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(out)).To(Equal("a"))
			})
		})
		Context("writing fails", func() {
			It("returns an error", func() {
				s := &SPDZEngine{
					cmder:          &FakeExecutor{},
					sourceCodePath: fmt.Sprintf("/non-existing-dir-%d/non-existing-file-%d", random, random),
					config:         &SPDZEngineTypedConfig{PrepFolder: prepFolder},
				}
				conf := &CtxConfig{
					Act: &Activation{
						Code: "a",
					},
				}
				err := s.Compile(conf)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(And(ContainSubstring(s.sourceCodePath), HaveSuffix("no such file or directory")))
			})
		})
		Context("compilation fails", func() {
			It("returns an error", func() {
				s := &SPDZEngine{
					cmder:          &BrokenFakeExecutor{},
					sourceCodePath: fileName,
					logger:         zap.NewNop().Sugar(),
					config:         &SPDZEngineTypedConfig{PrepFolder: prepFolder},
				}
				conf := &CtxConfig{
					Act: &Activation{
						Code: "a",
					},
				}
				err := s.Compile(conf)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("some error"))
			})
		})
	})
	Context("executing the user code", func() {
		var (
			random     int32
			fileName   string
			prepFolder string
			s          *SPDZEngine
			ctx        *CtxConfig
		)
		BeforeEach(func() {
			rand.Seed(time.Now().UnixNano())
			random = rand.Int31()
			prepFolder, _ := ioutil.TempDir("", "ephemeral_")
			fileName = fmt.Sprintf("%s/ip-file", prepFolder)
			s = &SPDZEngine{
				proxy:   &FakeProxy{},
				logger:  zap.NewNop().Sugar(),
				cmder:   &FakeExecutor{},
				feeder:  &FakeFeeder{},
				baseDir: "/tmp",
				ipFile:  fileName,
				config: &SPDZEngineTypedConfig{
					PlayerID:   int32(0),
					PrepFolder: prepFolder,
				},
			}
			ctx = &CtxConfig{
				Act: &Activation{
					GameID: "71b2a100-f3f6-11e9-81b4-2a2ae2dbcce4",
				},
				Context: context.TODO(),
				Spdz: &SPDZEngineTypedConfig{
					PlayerCount: 2,
				},
			}
		})
		AfterEach(func() {
			_ = os.RemoveAll(prepFolder)
		})
		Context("when amphora parameters are provided", func() {
			It("returns the result of the execution", func() {
				input := "a"
				ctx.Act.AmphoraParams = []string{input}
				res, err := s.Activate(ctx)
				Expect(err).NotTo(HaveOccurred())
				// Fake feeder simply echoes the first input parameter.
				Expect(res).To(Equal([]byte(input)))
			})
		})
		Context("when input parameters are defined in the request", func() {
			It("returns the result of the execution", func() {
				input := "b"
				ctx.Act.SecretParams = []string{input}
				res, err := s.Activate(ctx)
				Expect(err).NotTo(HaveOccurred())
				// Fake feeder simply echoes the first input parameter.
				Expect(res).To(Equal([]byte(input)))
			})
		})
		Context("when no input parameters are specified", func() {
			It("returns an error", func() {
				res, err := s.Activate(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("no MPC parameters specified"))
				Expect(res).To(BeNil())
			})
		})
		Context("when proxy fails to start", func() {
			It("returns an error", func() {
				s.proxy = &BrokenFakeProxy{}
				res, err := s.Activate(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("error starting the tcp proxy: some error"))
				Expect(res).To(BeNil())
			})
		})
		Context("when writing the IP file fails", func() {
			It("returns an error", func() {
				s.ipFile = fmt.Sprintf("/non-existing-dir-%d/non-existing-file-%d", random, random)
				res, err := s.Activate(ctx)
				Expect(err).To(HaveOccurred())
				Expect(res).To(BeNil())
			})
		})
	})

	Context("when executing MPC computation", func() {
		var (
			oldFio    utils.FileIO
			mockedFio *utils.MockedFileIO
			errCh     chan error
			s         *SPDZEngine
			act       *Activation
			ctx       *CtxConfig
		)
		BeforeEach(func() {
			oldFio = utils.Fio
			mockedFio = &utils.MockedFileIO{}
			utils.Fio = mockedFio
			errCh = make(chan error, 1)
			s = &SPDZEngine{
				logger:  zap.NewNop().Sugar(),
				baseDir: "/tmp",
				config: &SPDZEngineTypedConfig{
					PlayerID: int32(0),
				},
				playerDataPaths: map[castor.SPDZProtocol]string{castor.SPDZGf2n: "gf2n", castor.SPDZGfp: "gfp"},
			}
			act = &Activation{}
			ctx = &CtxConfig{
				Act:     act,
				Context: context.TODO(),
				Spdz: &SPDZEngineTypedConfig{
					PlayerCount: 2,
				},
				ErrCh: errCh,
			}
		})
		AfterEach(func() {
			utils.Fio = oldFio
		})
		Context("when getNumberOfThreads fails", func() {
			Context("with schedule file cannot be opened", func() {
				It("return error", func() {
					expectedError := fmt.Errorf("expected error")
					mockedFio.OpenReadResponse = utils.OpenReadResponse{File: nil, Error: expectedError}
					s.startMPC(ctx)
					err := <-errCh
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(And(
						HavePrefix("failed to determine the number of threads:"),
						HaveSuffix("error accessing the program's schedule: %v", expectedError)))
				})
			})
			Context("with line cannot be read", func() {
				It("return error", func() {
					scheduleFile := &utils.SimpleFileMock{}
					expectedError := fmt.Errorf("expected error")
					mockedFio.OpenReadResponse = utils.OpenReadResponse{File: scheduleFile, Error: nil}
					mockedFio.ReadLineResponse = utils.ReadLineResponse{Line: "", Error: expectedError}
					s.startMPC(ctx)
					err := <-errCh
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(And(
						HavePrefix("failed to determine the number of threads:"),
						HaveSuffix("error reading number of threads: %v", expectedError)))
				})
			})
		})
		Context("when multiple threads defined", func() {
			var scheduleFile utils.File
			numberOfThreads := 2
			BeforeEach(func() {
				scheduleFile = &utils.SimpleFileMock{}
				mockedFio.OpenReadResponse = utils.OpenReadResponse{File: scheduleFile, Error: nil}
				mockedFio.ReadLineResponse = utils.ReadLineResponse{Line: strconv.Itoa(numberOfThreads), Error: nil}
			})
			Context("when invalid gameID defined", func() {
				It("return error", func() {
					invalidGameID := "invalidUUID"
					ctx.Act.GameID = invalidGameID
					s.startMPC(ctx)
					err := <-errCh
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(fmt.Errorf("error parsing gameID: invalid UUID length: %d", len(invalidGameID))))
				})
			})
			Context("when gameId defined", func() {
				gameID, _ := uuid.NewUUID()
				BeforeEach(func() {
					ctx.Act.GameID = gameID.String()
				})
				Context("when tuple streamers cannot be created", func() {
					It("return error", func() {
						s.streamerFactory = func(*zap.SugaredLogger, castor.TupleType, *SPDZEngineTypedConfig, string, uuid.UUID, int) (io.TupleStreamer, error) {
							return nil, fmt.Errorf("expected error")
						}
						s.startMPC(ctx)
						err := <-errCh
						Expect(err).To(HaveOccurred())
						Expect(err).To(Equal(fmt.Errorf("expected error")))
					})
				})
				Context("with tuple streamer started successfully", func() {
					Context("when SPDZ process fails", func() {
						BeforeEach(func() {
							s.streamerFactory = func(*zap.SugaredLogger, castor.TupleType, *SPDZEngineTypedConfig, string, uuid.UUID, int) (io.TupleStreamer, error) {
								return &FakeTupleStreamer{}, nil
							}
							s.cmder = &BrokenFakeExecutor{}
						})
						It("return error", func() {
							s.startMPC(ctx)
							err := <-errCh
							Expect(err).To(HaveOccurred())
							Expect(err).To(Equal(fmt.Errorf("error while executing the user code: some error")))
						})
					})
					Context("when one tuple streamer emits error", func() {
						expectedError := fmt.Errorf("expected error")
						var cfe *CallbackFakeExecutor
						BeforeEach(func() {
							cfe = &CallbackFakeExecutor{
								callback: func(fts *FakeTupleStreamer) {
									fts.errCh <- expectedError
									select {
									case <-fts.terminateChan:
									case <-time.After(10 * time.Second):
										Fail("terminate timed out unexpectedly")
									}
								}}
							s.cmder = cfe
							s.streamerFactory = func(*zap.SugaredLogger, castor.TupleType, *SPDZEngineTypedConfig, string, uuid.UUID, int) (io.TupleStreamer, error) {
								fts := &FakeTupleStreamer{}
								cfe.fts = fts
								return fts, nil
							}
						})
						It("return error", func() {
							go func() {
								s.startMPC(ctx)
							}()
							var err error
							select {
							case err = <-errCh:
							case <-time.After(5 * time.Second):
								Fail("test timed out")
							}
							Expect(err).To(HaveOccurred())
							Expect(err).To(Equal(fmt.Errorf("error while streaming tuples: %v", expectedError)))
							select {
							case <-cfe.fts.terminateChan:
							case <-time.After(5 * time.Second):
								Fail("terminate not received")
							}
						})
					})
				})
			})
		})
	})

	Context("when creating a new instance of SPDZEngine", func() {
		It("sets the required parameters", func() {
			prepFolder, _ := ioutil.TempDir("", "ephemeral_")
			defer os.RemoveAll(prepFolder)
			logger := zap.NewNop().Sugar()
			cmder := &utils.Commander{}
			config := &SPDZEngineTypedConfig{PrepFolder: prepFolder}
			s, _ := NewSPDZEngine(logger, cmder, config)
			Expect(s.baseDir).To(Equal(baseDir))
			Expect(s.ipFile).To(Equal(ipFile))
			gf2nMacFile := fmt.Sprintf("%s/%d-%s-%d/Player-MAC-Keys-%s-P%d",
				config.PrepFolder, config.PlayerCount, castor.SPDZGf2n.Shorthand, config.Gf2nBitLength, castor.SPDZGf2n.Shorthand, config.PlayerID)
			gfpMacFile := fmt.Sprintf("%s/%d-%s-%d/Player-MAC-Keys-%s-P%d",
				config.PrepFolder, config.PlayerCount, castor.SPDZGfp.Shorthand, config.Prime.BitLen(), castor.SPDZGfp.Shorthand, config.PlayerID)
			gfpParamsFile := fmt.Sprintf("%s/%d-%s-%d/Params-Data",
				config.PrepFolder, config.PlayerCount, castor.SPDZGfp.Shorthand, config.Prime.BitLen())
			Expect(gf2nMacFile).To(BeAnExistingFile())
			Expect(gfpMacFile).To(BeAnExistingFile())
			Expect(gfpParamsFile).To(BeAnExistingFile())
		})
	})
	Context("executing SPDZWrapper", func() {
		var (
			respCh chan []byte
			errCh  chan error
			w      *SPDZWrapper
		)
		BeforeEach(func() {
			respCh = make(chan []byte, 1)
			errCh = make(chan error, 1)
			w = &SPDZWrapper{
				errCh: errCh,
				activate: func(*CtxConfig) ([]byte, error) {
					return []byte("a"), nil
				},
				respCh: respCh,
				logger: zap.NewNop().Sugar(),
				ctx: &CtxConfig{
					ProxyEntries: []*ProxyConfig{{}},
					Spdz: &SPDZEngineTypedConfig{
						PlayerID: 0,
					},
					Act: &Activation{
						GameID: "71b2a100-f3f6-11e9-81b4-2a2ae2dbcce4",
					},
				},
			}
		})
		Context("when no error occurs", func() {
			It("writes the response to the channel", func() {
				event := &pb.Event{
					Players: []*pb.Player{
						&pb.Player{
							Id: 100,
						},
						&pb.Player{
							Id: 101,
						},
					},
				}
				err := w.Execute(event)
				Expect(err).NotTo(HaveOccurred())
				res := <-respCh
				Expect(res).To(Equal([]byte("a")))
			})
		})
		Context("when there is no second player in the list", func() {
			It("returns an error", func() {
				event := &pb.Event{
					Players: []*pb.Player{
						// There is no player with id=101 in the list.
						&pb.Player{
							Id: 100,
						},
					},
				}
				err := w.Execute(event)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("you must provide at least two players"))
			})
		})
		Context("when activation fails", func() {
			It("returns err to the channels and responds with an error", func() {
				w.activate = func(*CtxConfig) ([]byte, error) {
					return nil, errors.New("some error")
				}
				event := &pb.Event{
					Players: []*pb.Player{
						&pb.Player{
							Id: 100,
						},
						&pb.Player{
							Id: 101,
						},
					},
				}
				err := w.Execute(event)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("some error"))
				res := <-errCh
				Expect(res).NotTo(BeNil())
			})
		})
	})
})

type FakeTupleStreamer struct {
	terminateChan chan struct{}
	errCh         chan error
	wg            *sync.WaitGroup
}

func (fts *FakeTupleStreamer) StartStreamTuples(terminateCh chan struct{}, errCh chan error, wg *sync.WaitGroup) {
	wg.Done()
	fts.terminateChan = terminateCh
	fts.errCh = errCh
	fts.wg = wg
}

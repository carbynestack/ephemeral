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
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	. "github.com/carbynestack/ephemeral/pkg/types"
	"time"

	"github.com/carbynestack/ephemeral/pkg/utils"

	"math/rand"

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
			fileName string
			random   int32
		)
		BeforeEach(func() {
			rand.Seed(time.Now().UnixNano())
			random = rand.Int31()
			fileName = fmt.Sprintf("/tmp/program-%d.mpc", random)
		})
		AfterEach(func() {
			cmder.CallCMD([]string{fmt.Sprintf("rm %s", fileName)}, "./")
		})
		Context("writing succeeds", func() {
			It("writes the source code on the disk and runs the compiler", func() {
				s := &SPDZEngine{
					cmder:          &FakeExecutor{},
					sourceCodePath: fileName,
					logger:         zap.NewNop().Sugar(),
				}
				conf := &CtxConfig{
					Act: &Activation{
						Code: "a",
					},
				}
				err := s.Compile(conf)
				Expect(err).NotTo(HaveOccurred())
				out, _, err := cmder.CallCMD([]string{fmt.Sprintf("cat %s", s.sourceCodePath)}, "./")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(out)).To(Equal("a"))
			})
		})
		Context("writing fails", func() {
			It("returns an error", func() {
				s := &SPDZEngine{
					cmder:          &FakeExecutor{},
					sourceCodePath: fmt.Sprintf("/non-existing-dir-%d/non-existing-file-%d", random, random),
				}
				conf := &CtxConfig{
					Act: &Activation{
						Code: "a",
					},
				}
				err := s.Compile(conf)
				Expect(err).To(HaveOccurred())
			})
		})
		Context("compilation fails", func() {
			It("returns an error", func() {
				s := &SPDZEngine{
					cmder:          &BrokenFakeExecutor{},
					sourceCodePath: fileName,
					logger:         zap.NewNop().Sugar(),
				}
				conf := &CtxConfig{
					Act: &Activation{
						Code: "a",
					},
				}
				err := s.Compile(conf)
				Expect(err).To(HaveOccurred())
			})
		})
	})
	Context("executing the user code", func() {
		var (
			random   int32
			fileName string
			s        *SPDZEngine
			ctx      *CtxConfig
		)
		BeforeEach(func() {
			rand.Seed(time.Now().UnixNano())
			random = rand.Int31()
			fileName = fmt.Sprintf("/tmp/ip-file-%d", random)
			s = &SPDZEngine{
				proxy:   &FakeProxy{},
				logger:  zap.NewNop().Sugar(),
				cmder:   &FakeExecutor{},
				feeder:  &FakeFeeder{},
				baseDir: "/tmp",
				ipFile:  fileName,
				config: &SPDZEngineTypedConfig{
					PlayerID: int32(0),
				},
			}
			ctx = &CtxConfig{
				Act: &Activation{
					GameID: "0",
				},
				Context: context.TODO(),
				Spdz: &SPDZEngineTypedConfig{
					PlayerCount: 2,
				},
			}
		})
		AfterEach(func() {
			cmder.Run(fmt.Sprintf("rm %s", fileName))
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
	Context("when creating a new instance of SPDZEngine", func() {
		It("sets the required parameters", func() {
			logger := zap.NewNop().Sugar()
			cmder := &utils.Commander{}
			config := &SPDZEngineTypedConfig{}
			s := NewSPDZEngine(logger, cmder, config)
			Expect(s.baseDir).To(Equal(baseDir))
			Expect(s.ipFile).To(Equal(ipFile))
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
						GameID: "abc",
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

// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package client

import (
	"context"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	. "github.com/carbynestack/ephemeral/pkg/discovery/transport/server"
	"time"

	"go.uber.org/zap/zapcore"

	. "github.com/carbynestack/ephemeral/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

var _ = Describe("Client", func() {

	var (
		serverIn, serverOut, clientIn, clientOut chan *pb.Event
		errCh                                    chan error
		tr                                       *TransportServer
		client                                   *Client
		port                                     = "9596"
		cb                                       = func() {}
		gameID                                   string
	)
	Context("when using the client", func() {
		BeforeEach(func() {
			serverIn = make(chan *pb.Event)
			serverOut = make(chan *pb.Event)
			clientIn = make(chan *pb.Event)
			clientOut = make(chan *pb.Event)
			errCh = make(chan error)
			logger := zap.NewNop().Sugar()
			serverConf := &TransportConfig{
				In:     serverIn,
				Out:    serverOut,
				ErrCh:  errCh,
				Port:   port,
				Logger: logger,
			}
			gameID = "71b2a100-f3f6-11e9-81b4-2a2ae2dbcce4"
			tr = NewTransportServer(serverConf)
			conf := &TransportClientConfig{
				In:         clientIn,
				Out:        clientOut,
				ErrCh:      errCh,
				Host:       "localhost",
				Port:       port,
				EventScope: EventScopeAll,
				ConnID:     gameID,
				Logger:     logger,
				Timeout:    10 * time.Second,
				Context:    context.TODO(),
			}
			client, _ = NewClient(conf)
		})
		AfterEach(func() {
			tr.Stop()
			client.Stop()
		})
		It("sends an event to the server and receives a response back", func() {
			// The GRPC server is backed by a simple echo server.
			go func() {
				ev := <-tr.GetIn()
				tr.GetOut() <- ev
			}()
			go tr.Run(cb)
			// TODO: get rid of the sleeps.
			time.Sleep(100 * time.Millisecond)
			conn, _ := client.Connect()
			cl := pb.NewDiscoveryClient(conn)
			go client.Run(cl)
			ev := &pb.Event{
				GameID: gameID,
			}
			clientOut <- ev
			var resp *pb.Event
			select {
			case resp = <-clientIn:
				Expect(resp.GameID).To(Equal(gameID))
			case err := <-errCh:
				Expect(err).To(BeNil())
			case <-time.After(5 * time.Second):
				Expect(resp).NotTo(BeNil())
			}
		})
	})
	Context("when creating a new client", func() {
		It("returns an error if an empty connection id is provided", func() {
			conf := &TransportClientConfig{
				ConnID: "",
			}
			_, err := NewClient(conf)
			Expect(err).To(HaveOccurred())
		})
		It("returns an error if no event scope is provided", func() {
			conf := &TransportClientConfig{
				ConnID:     "abc",
				EventScope: "",
			}
			_, err := NewClient(conf)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error if no host is provided", func() {
			conf := &TransportClientConfig{
				ConnID:     "abc",
				EventScope: EventScopeAll,
				Host:       "",
			}
			_, err := NewClient(conf)
			Expect(err).To(HaveOccurred())
		})
		It("return an error if no port is provided", func() {
			conf := &TransportClientConfig{
				ConnID:     "abc",
				EventScope: EventScopeAll,
				Host:       "localhost",
				Port:       "",
			}
			_, err := NewClient(conf)
			Expect(err).To(HaveOccurred())
		})

		It("returns no error when all required properties are provided", func() {
			conf := &TransportClientConfig{
				ConnID:     "abc",
				EventScope: EventScopeAll,
				Host:       "localhost",
				Port:       "8080",
			}
			_, err := NewClient(conf)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when sending events *to* the server", func() {
		var (
			outCh  chan *pb.Event
			errCh  chan error
			conf   *TransportClientConfig
			ctx    context.Context
			cancel context.CancelFunc
		)
		BeforeEach(func() {
			outCh = make(chan *pb.Event)
			errCh = make(chan error)
			ctx, cancel = context.WithCancel(context.Background())
			conf = &TransportClientConfig{
				Out:     outCh,
				ErrCh:   errCh,
				Context: ctx,
			}
		})
		Context("when no error occurs", func() {
			It("sends an event to the stream", func() {
				st := &FakeStream{
					sendCh: make(chan struct{}),
				}
				cl := Client{
					conf:   conf,
					stream: st,
				}
				go cl.streamOut()
				cl.conf.Out <- &pb.Event{}
				// The expectation is that the test exits without blocking.
				<-st.sendCh
			})
		})
		Context("when an error occurs", func() {
			It("forwards it to the error channel", func() {
				st := &BrokenStream{}
				cl := Client{
					conf:   conf,
					stream: st,
				}
				go cl.streamOut()
				cl.conf.Out <- &pb.Event{}
				// The expectation is that the test exits without blocking.
				err := <-errCh
				Expect(err.Error()).To(Equal("crazyFrog"))
			})
		})
		Context("when the context is cancelled", func() {
			It("stops the execution", func() {
				st := &FakeStream{}
				cl := Client{
					conf:   conf,
					stream: st,
				}
				cancel()
				err := cl.streamOut()
				Expect(err).To(BeNil())
			})
		})
	})

	Context("when receiving events *from* the server", func() {
		var (
			outCh  chan *pb.Event
			errCh  chan error
			conf   *TransportClientConfig
			ctx    context.Context
			cancel context.CancelFunc
		)
		BeforeEach(func() {
			outCh = make(chan *pb.Event)
			errCh = make(chan error)
			ctx, cancel = context.WithCancel(context.Background())
			conf = &TransportClientConfig{
				Out:     outCh,
				ErrCh:   errCh,
				Context: ctx,
			}
		})
		Context("when the context is cancelled", func() {
			It("closes the connection", func() {
				st := &FakeStream{
					closeSendCh: make(chan struct{}, 1),
				}
				conf.Logger = zap.NewNop().Sugar()
				cl := Client{
					conf:   conf,
					stream: st,
					conn:   &FakeTransportConn{},
				}
				cancel()
				cl.streamIn()
				<-st.closeSendCh
			})
			Context("when an error occurs when closing the connection", func() {
				It("prints it out", func() {
					st := &BrokenStream{}
					core, recorded := observer.New(zapcore.ErrorLevel)
					conf.Logger = zap.New(core).Sugar()
					cl := Client{
						conf:   conf,
						stream: st,
						conn:   &FakeTransportConn{},
					}
					cancel()
					err := cl.streamIn()
					Expect(err).To(BeNil())
					Expect(recorded.Len()).To(Equal(1))
					Expect(recorded.AllUntimed()[0].Entry.Message).To(Equal("error stopping gRPC client " + st.CloseSend().Error()))
				})
			})
		})
		Context("when the stream was closed", func() {
			It("prints out a message", func() {
				st := &BrokenStream{}
				core, recorded := observer.New(zapcore.DebugLevel)
				conf.Logger = zap.New(core).Sugar()
				cl := Client{
					conf:   conf,
					stream: st,
					conn:   &FakeTransportConn{},
				}
				err := cl.streamIn()
				Expect(err).To(BeNil())
				Expect(recorded.Len()).To(Equal(3))
				Expect(recorded.AllUntimed()[0].Entry.Message).To(Equal("server closed the connection"))
			})
		})
	})
	Context("when using client interfaces", func() {
		It("returns In channel", func() {
			inCh := make(chan *pb.Event)
			conf := &TransportClientConfig{
				In: inCh,
			}
			cl := Client{
				conf: conf,
			}
			Expect(cl.GetIn()).To(Equal(inCh))
		})
		It("returns Out channel", func() {
			outCh := make(chan *pb.Event)
			conf := &TransportClientConfig{
				Out: outCh,
			}
			cl := Client{
				conf: conf,
			}
			Expect(cl.GetOut()).To(Equal(outCh))
		})
	})
	Context("when establishing a connection fails", func() {
		It("sends a message to the error channel", func() {
			conf := &TransportClientConfig{
				Timeout: 1 * time.Millisecond,
				Logger:  zap.NewNop().Sugar(),
			}
			cl := Client{
				conf: conf,
			}
			_, err := cl.Connect()
			Expect(err.Error()).To(Equal("context deadline exceeded"))
		})
	})
	Context("when reading events fails", func() {
		It("sends a message to the error channel", func() {
			cl := &BrokenDiscoveryClient{}
			errCh := make(chan error, 1)
			conf := &TransportClientConfig{
				ErrCh:   errCh,
				Context: context.TODO(),
			}
			client := Client{
				conf: conf,
			}
			client.Run(cl)
			err := <-errCh
			Expect(err).NotTo(BeNil())
		})
	})
})

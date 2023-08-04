// Copyright (c) 2021-2023 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package server

import (
	"context"

	"net"
	"time"

	. "github.com/onsi/ginkgo"
	"google.golang.org/grpc"

	. "github.com/carbynestack/ephemeral/pkg/types"

	. "github.com/onsi/gomega"

	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc/metadata"
)

var _ = Describe("Server", func() {

	Context("when opening real connections between client and server", func() {
		var (
			in, out  chan *pb.Event
			errCh    chan error
			tr       *TransportServer
			conn     *grpc.ClientConn
			port     = "30000"
			cb       = func() {}
			deadline = 10 * time.Second
			stopCh   chan struct{}
		)
		BeforeEach(func() {
			in = make(chan *pb.Event)
			out = make(chan *pb.Event)
			errCh = make(chan error)
			logger := zap.NewNop().Sugar()
			conf := &TransportConfig{
				In:     in,
				Out:    out,
				ErrCh:  errCh,
				Port:   port,
				Logger: logger,
			}
			tr = NewTransportServer(conf)
			stopCh = make(chan struct{})
		})
		AfterEach(func() {
			conn.Close()
			tr.Stop()
			stopCh <- struct{}{}
		})
		Context("when a single client connects", func() {
			Context("the client is an ephemeral client", func() {
				It("sends the events corresponding to the client's connection ID", func() {
					game42 := "42"
					game43 := "43"
					// The server is simply echoing the received events.
					go echoServer(tr, stopCh)
					go tr.Run(cb)
					time.Sleep(100 * time.Millisecond)
					conn, _ = grpc.Dial("localhost:"+port, grpc.WithInsecure())
					client := pb.NewDiscoveryClient(conn)
					ephemeralClientCtx, _ := getContext(game43, EventScopeSelf, deadline)
					stream, err := client.Events(ephemeralClientCtx)
					Expect(err).To(BeNil())
					sendEvents(stream, game42, game43)

					ev, err := stream.Recv()
					Expect(err).To(BeNil())
					Expect(ev.GameID).To(Equal(game43))
				})
			})
			Context("the client is the discovery slave", func() {
				It("sends the events from all games", func() {
					game42 := "42"
					game43 := "43"
					go echoServer(tr, stopCh)
					go tr.Run(cb)
					time.Sleep(100 * time.Millisecond)
					conn, _ = grpc.Dial("localhost:"+port, grpc.WithInsecure())
					client := pb.NewDiscoveryClient(conn)
					slaveCtx, _ := getContext("", EventScopeAll, deadline)
					stream, err := client.Events(slaveCtx)
					Expect(err).To(BeNil())
					sendEvents(stream, game42, game43)

					ev, err := stream.Recv()
					Expect(err).To(BeNil())
					Expect(ev.GameID).To(Equal(game42))

					ev, err = stream.Recv()
					Expect(err).To(BeNil())
					Expect(ev.GameID).To(Equal(game43))
				})
			})
		})
		Context("when several clients connect", func() {
			Context("they belong to the same game", func() {
				It("sends the game events to all those clients", func() {
					game42 := "42"
					go echoServer(tr, stopCh)
					go tr.Run(cb)
					time.Sleep(100 * time.Millisecond)
					conn, _ = grpc.Dial("localhost:"+port, grpc.WithInsecure())
					ephemeralClientCtx, _ := getContext(game42, EventScopeSelf, deadline)

					client1 := pb.NewDiscoveryClient(conn)
					stream1, err := client1.Events(ephemeralClientCtx)
					Expect(err).To(BeNil())

					client2 := pb.NewDiscoveryClient(conn)
					stream2, err := client2.Events(ephemeralClientCtx)
					Expect(err).To(BeNil())
					time.Sleep(100 * time.Millisecond)

					// We send a single event and expect the response in both clients.
					sendEvents(stream1, game42)

					ev, err := stream1.Recv()
					Expect(err).To(BeNil())
					Expect(ev.GameID).To(Equal(game42))

					ev, err = stream2.Recv()
					Expect(err).To(BeNil())
					Expect(ev.GameID).To(Equal(game42))
				})
			})
			Context("they belong to different games", func() {
				It("sends events to the clients based on their game IDs", func() {
					game42 := "42"
					game43 := "43"
					go echoServer(tr, stopCh)
					go tr.Run(cb)
					time.Sleep(100 * time.Millisecond)
					conn, err := grpc.Dial("localhost:"+port, grpc.WithInsecure())
					Expect(err).To(BeNil())

					client0 := pb.NewDiscoveryClient(conn)
					slaveCtx, _ := getContext("abc", EventScopeAll, deadline)
					stream0, err := client0.Events(slaveCtx)
					Expect(err).To(BeNil())

					client1 := pb.NewDiscoveryClient(conn)
					ctx1, _ := getContext(game42, EventScopeSelf, deadline)
					stream1, err := client1.Events(ctx1)
					Expect(err).To(BeNil())

					client2 := pb.NewDiscoveryClient(conn)
					ctx2, _ := getContext(game43, EventScopeSelf, deadline)
					stream2, err := client2.Events(ctx2)
					Expect(err).To(BeNil())

					time.Sleep(200 * time.Millisecond)
					sendEvents(stream1, game42)
					sendEvents(stream2, game43)
					ev, err := stream0.Recv()
					Expect(err).To(BeNil())
					Expect(ev.GameID).NotTo(Equal(""))

					ev, err = stream0.Recv()
					Expect(err).To(BeNil())
					Expect(ev.GameID).NotTo(Equal(""))

					ev, err = stream1.Recv()
					Expect(err).To(BeNil())
					Expect(ev.GameID).To(Equal(game42))

					ev, err = stream2.Recv()
					Expect(err).To(BeNil())
					Expect(ev.GameID).To(Equal(game43))
				})
			})
			Context("one of the clients disconnects", func() {
				It("broadcasts messages to the clients that connect afterwards", func() {
					game42 := "42"
					go echoServer(tr, stopCh)
					go tr.Run(cb)
					time.Sleep(100 * time.Millisecond)
					var conn1, conn2, conn3 *grpc.ClientConn
					defer func() {
						conn1.Close()
						conn2.Close()
						conn3.Close()
					}()
					conn1, _ = grpc.Dial("localhost:"+port, grpc.WithInsecure())
					client1 := pb.NewDiscoveryClient(conn1)
					ctx, _ := getContext(game42, EventScopeSelf, deadline)
					stream1, err := client1.Events(ctx)
					Expect(err).To(BeNil())
					// client 1 disconnects shortly.
					err = stream1.CloseSend()
					Expect(err).To(BeNil())

					conn2, _ = grpc.Dial("localhost:"+port, grpc.WithInsecure())
					client2 := pb.NewDiscoveryClient(conn2)
					stream2, err := client2.Events(ctx)
					Expect(err).To(BeNil())

					conn3, _ = grpc.Dial("localhost:"+port, grpc.WithInsecure())
					client3 := pb.NewDiscoveryClient(conn3)
					stream3, err := client3.Events(ctx)
					Expect(err).To(BeNil())
					sendEvents(stream3, game42)

					ev, err := stream2.Recv()
					Expect(err).To(BeNil())
					Expect(ev.GameID).To(Equal(game42))

					ev, err = stream3.Recv()
					Expect(err).To(BeNil())
					Expect(ev.GameID).To(Equal(game42))
				})
			})
		})
	})

	Context("when extracting stream metadata", func() {
		Context("when failures take place", func() {
			Context("when no metadata is provided in the context", func() {
				It("responds with an error", func() {
					ts := TransportServer{}
					ctx := context.TODO()
					_, _, err := ts.extractMeta(ctx)
					Expect(err.Error()).To(Equal("no metadata in the stream context"))
				})
			})
			Context("when metadata is provided in the context", func() {
				var (
					ts  TransportServer
					ctx context.Context
					md  metadata.MD
				)
				BeforeEach(func() {
					ts = TransportServer{}
					ctx = context.Background()
					md = metadata.MD{}
				})
				Context("when ConnID has more then a single value", func() {
					It("responds with an error", func() {
						md.Append(ConnID, "a", "b")
						ctx = metadata.NewIncomingContext(ctx, md)
						_, _, err := ts.extractMeta(ctx)
						Expect(err.Error()).To(Equal("ConnID must contain exactly one element"))
					})
				})
				Context("when EventScope has more then a single value", func() {
					It("responds with an error", func() {
						md.Append(ConnID, "a")
						md.Append(EventScope, "a", "b")
						ctx = metadata.NewIncomingContext(ctx, md)
						_, _, err := ts.extractMeta(ctx)
						Expect(err.Error()).To(Equal("EventScope must contain exactly one element"))
					})
				})
				Context("when an error happens within extractMeta method", func() {
					It("returns an error", func() {
						md.Append(ConnID, "a", "b")
						ctx = metadata.NewIncomingContext(ctx, md)
						st := &FakeStream{
							context: ctx,
						}
						err := ts.Events(st)
						Expect(err.Error()).To(Equal("ConnID must contain exactly one element"))
					})
				})
			})
		})
	})
	Context("when forwarding events from stream", func() {
		Context("when stream context is cancelled", func() {
			It("sends context cancellation error to the error channel", func() {
				ctx := context.Background()
				ctx, cancel := context.WithCancel(ctx)
				st := &FakeStream{
					context: ctx,
				}
				errCh := make(chan error, 1)
				ts := TransportServer{}
				cancel()
				ts.forwardFromStream(st, errCh)
				err := <-errCh
				Expect(err.Error()).To(Equal("context canceled"))
			})
		})
	})
	Context("when forwarding events from stream", func() {
		Context("when an unknown event scope is provided", func() {
			It("logs an error", func() {
				invalidScope := "invalidScope"
				core, recorded := observer.New(zapcore.ErrorLevel)
				conf := &TransportConfig{
					Logger: zap.New(core).Sugar(),
				}
				ts := TransportServer{
					conf: conf,
				}
				st := &FakeStream{}

				f := ts.forwardToStream(st, invalidScope, "abc")
				ev := &pb.Event{}
				f(ev)
				Expect(recorded.Len()).To(Equal(1))
				Expect(recorded.AllUntimed()[0].Entry.Message).To(Equal("Unknown event scope " + invalidScope))
			})
		})
	})
	Context("when the events are sent back to the stream", func() {
		Context("when there is an error", func() {
			It("prints out an error message", func() {
				core, recorded := observer.New(zapcore.ErrorLevel)
				conf := &TransportConfig{
					Logger: zap.New(core).Sugar(),
				}
				ts := TransportServer{
					conf: conf,
				}
				st := &BrokenStream{}
				ts.sendEvent(st, &pb.Event{Name: "abc"})
				Expect(recorded.Len()).To(Equal(1))
				Expect(recorded.AllUntimed()[0].Entry.Message).To(Equal("Error broadcasting the event abc"))
			})
		})
	})
	Context("when starting a failing listener", func() {
		It("notifies the done channel", func() {
			ts := TransportServer{
				grpcServer: &grpc.Server{},
			}
			ls := BrokenListener{}
			done := make(chan struct{}, 1)
			ts.serve(&ls, done)
			<-done
		})
	})
	Context("when broadcasting events", func() {
		It("exits upon a message from 'done' channel", func() {
			core, recorded := observer.New(zapcore.DebugLevel)
			conf := &TransportConfig{
				Out:    make(chan *pb.Event),
				Logger: zap.New(core).Sugar(),
			}
			ts := TransportServer{
				conf: conf,
			}
			done := make(chan struct{}, 1)
			done <- struct{}{}
			// The command below should not block.
			ts.broadcast(done)
			Expect(recorded.Len()).To(Equal(1))
			Expect(recorded.AllUntimed()[0].Entry.Message).To(Equal("Stopped broadcasting"))
		})
	})

	Context("when net listener fails", func() {
		It("returns an error", func() {
			p := "7777"
			lis, err := net.Listen("tcp", ":"+p)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				if lis != nil {
					lis.Close()
				}
			}()
			conf := &TransportConfig{
				Port: p,
			}
			tr := TransportServer{
				conf: conf,
			}
			err = tr.Run(func() {})
			Expect(err).To(HaveOccurred())
		})
	})
})

func getStream(id, scope, port string, deadline time.Duration) pb.Discovery_EventsClient {
	conn, _ := grpc.Dial("localhost:"+port, grpc.WithInsecure())
	client1 := pb.NewDiscoveryClient(conn)
	ctx1, _ := getContext(id, scope, deadline)
	stream, _ := client1.Events(ctx1)
	return stream
}

func echoServer(tr *TransportServer, stopCh chan struct{}) {
	for {
		select {
		case ev := <-tr.GetIn():
			tr.GetOut() <- ev
		case <-stopCh:
			return
		}
	}
}

func sendEvents(stream pb.Discovery_EventsClient, events ...string) {
	for _, e := range events {
		ev := &pb.Event{
			GameID: e,
		}
		err := stream.Send(ev)
		Expect(err).To(BeNil())
	}
}

// getContext creates a context with a timeout, connection id and event scope.
func getContext(id, scope string, t time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), t)
	ctx = metadata.AppendToOutgoingContext(ctx, ConnID, id, EventScope, scope)
	return ctx, cancel
}

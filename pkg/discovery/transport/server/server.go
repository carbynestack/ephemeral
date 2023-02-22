// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package server

import (
	"context"
	"errors"
	"io"
	"net"

	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"

	. "github.com/carbynestack/ephemeral/pkg/types"

	mb "github.com/vardius/message-bus"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const broadcastTopic = "broadcast"

// TransportConfig is configuration of the GRPC Server.
type TransportConfig struct {
	// In, Out is the external interface for the libraries that would like to use this client. Events received from "In" are forwarded to the server. The responses are sent back to "Out"
	In, Out chan *pb.Event

	// ErrCh is the sink for all errors from the client.
	ErrCh chan error

	// Port - the port to open up the connection.
	Port string

	Logger *zap.SugaredLogger
}

// Transport is in interface covering the discovery service transport.
type Transport interface {
	Run(func()) error
	Stop()
	GetIn() chan *pb.Event
	GetOut() chan *pb.Event
	Events(stream pb.Discovery_EventsServer) error
}

// NewTransportServer returns a new transport server.
func NewTransportServer(conf *TransportConfig) *TransportServer {
	tr := &TransportServer{
		conf:       conf,
		mb:         mb.New(10000),
		grpcServer: grpc.NewServer(),
	}
	return tr
}

// TransportServer is a server the dispatches messsages from and to GRPC based transport.
type TransportServer struct {
	conf       *TransportConfig
	grpcServer *grpc.Server
	mb         mb.MessageBus
}

// GetIn returns the input channel of the transport.
func (d *TransportServer) GetIn() chan *pb.Event {
	return d.conf.In
}

// GetOut returns the output channel of the transport.
func (d *TransportServer) GetOut() chan *pb.Event {
	return d.conf.Out
}

// Run starts the transport server.
func (d *TransportServer) Run(cb func()) error {
	lis, err := net.Listen("tcp", ":"+d.conf.Port)
	if err != nil {
		return err
	}
	pb.RegisterDiscoveryServer(d.grpcServer, d)
	done := make(chan struct{}, 1)
	go d.broadcast(done)
	cb()
	d.serve(lis, done)
	return nil
}

// Stop stops the transport server.
func (d *TransportServer) Stop() {
	d.grpcServer.Stop()
}

// Events is a Remote Procedure that is executed by GRPC clietns. it instantiates the communication with the server. The messages are sent and read from In and Out channels instead of manipulating the GRPC stream directly.
func (d *TransportServer) Events(stream pb.Discovery_EventsServer) error {
	ctx := stream.Context()
	connID, scope, err := d.extractMeta(ctx)
	if err != nil {
		return err
	}
	// Read all outgoing events from the broadcast topic.
	d.mb.Subscribe(broadcastTopic, d.forwardToStream(stream, scope, connID))
	errCh := make(chan error)
	go d.forwardFromStream(stream, errCh)
	// Block until we receive an error.
	err = <-errCh
	d.mb.Unsubscribe(broadcastTopic, d.forwardToStream(stream, scope, connID))
	d.conf.Logger.Debug("Unsubscribed forwardToStream from the broadcast topic")
	return err
}

func (d *TransportServer) serve(lis net.Listener, done chan struct{}) error {
	if err := d.grpcServer.Serve(lis); err != nil {
		done <- struct{}{}
		return err
	}
	return nil
}

// Publish all outgoing events to the broadcast topic until done.
func (d *TransportServer) broadcast(done chan struct{}) {
	for {
		select {
		case ev := <-d.conf.Out:
			d.mb.Publish(broadcastTopic, ev)
		case <-done:
			return
		}
	}
}

// extractMeta extracts metadata from the stream connection context.
// It expects ConnID and EventScope to be set, it returns errors otherwise.
func (d *TransportServer) extractMeta(ctx context.Context) (connID string, scope string, err error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if ok {
		IDs := meta.Get(ConnID)
		if len(IDs) != 1 {
			return connID, scope, errors.New("ConnID must contain exactly one element")
		}
		connID = IDs[0]
		eventScopes := meta.Get(EventScope)
		if len(eventScopes) != 1 {
			return connID, scope, errors.New("EventScope must contain exactly one element")
		}
		scope = eventScopes[0]
		return connID, scope, nil
	}
	return connID, scope, errors.New("no metadata in the stream context")
}

// forwardToStream returns a function that is used as an event handler for the message bus. Depending on the event scope it forwards the events to the corresponding message bus topic.
func (d *TransportServer) forwardToStream(stream pb.Discovery_EventsServer, scope, connID string) func(e interface{}) {
	return func(e interface{}) {
		ev := e.(*pb.Event)
		switch scope {
		// This is the slave, forward all events.
		case EventScopeAll:
			d.sendEvent(stream, ev)
		// This is an ordinary discovery client, only the events belonging to the gameID are forwarded.
		case EventScopeSelf:
			if connID == ev.GameID {
				d.sendEvent(stream, ev)
			}
		default:
			d.conf.Logger.Errorf("unknown event scope %v", scope)
		}
	}
}

// sendEvent sents out an event and potentially prints an error.
func (d *TransportServer) sendEvent(stream pb.Discovery_EventsServer, ev *pb.Event) {
	err := stream.Send(ev)
	if err != nil {
		d.conf.Logger.Errorf("error broadcasting the event %s", ev.Name)
	}
}

// forwardFromStream consumes events from the stream and forwards it to the In channel.
func (d *TransportServer) forwardFromStream(stream pb.Discovery_EventsServer, errCh chan error) {
	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		default:
			ev, err := stream.Recv()
			if err == io.EOF {
				d.conf.Logger.Debugf("server is exiting due to an EOF")
				return
			}
			if err != nil {
				errCh <- err
				return
			}
			d.conf.In <- ev
		}
	}
}

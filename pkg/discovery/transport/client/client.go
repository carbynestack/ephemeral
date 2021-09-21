//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package client

import (
	"context"
	"errors"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"io"
	"time"

	. "github.com/carbynestack/ephemeral/pkg/types"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// TransportClientConfig preserves config params of the client.
type TransportClientConfig struct {
	// In, Out is the external interface for the libraries that would like to use this client. Events received from "In" are forwarded to the server. The responses are sent back to "Out"
	In, Out chan *pb.Event

	// ErrCh is the sink for all errors from the client.
	ErrCh chan error

	// Host, Port - the server endpoint to connect to.
	Host, Port string

	// EventScope defines the scope of events the client subscribes to. "all" - events from all games are current games, "ConnID" - events associated with this connection ID.
	EventScope string

	// ConnID is the ID of the connection. In case of pure discovery clients, it is equal the gameID.
	ConnID string

	// Timeout is the gRPC dial timeout.
	Timeout time.Duration

	Logger *zap.SugaredLogger

	Context context.Context
}

// TransportConn is an interface for the underlying gRPC transport connection.
type TransportConn interface {
	Close() error
}

// NewClient returns a new transport client.
func NewClient(conf *TransportClientConfig) (*Client, error) {
	if conf.ConnID == "" {
		return nil, errors.New("connection id cannot be empty")
	}
	if conf.EventScope == "" {
		return nil, errors.New("event scope must be set")
	}
	if conf.Host == "" {
		return nil, errors.New("a host must be provided")
	}
	if conf.Port == "" {
		return nil, errors.New("a port must be provided")
	}
	cl := &Client{
		conf: conf,
	}
	return cl, nil
}

// TransportClient is an interface for the underlying transport connection, e.g. GRPC.
type TransportClient interface {
	GetIn() chan *pb.Event
	GetOut() chan *pb.Event
	Connect() (*grpc.ClientConn, error)
	Run(client pb.DiscoveryClient)
	Stop() error
}

// Client is used to communicate with Discovery service.
// It is a wrapper around the gRPC client.
// To send events one writes to the Out channel, reading is done by consuming messages from the In channel.
// Errors are forwarded to the errCh specified in the config. Thus it must be monitored.
type Client struct {
	conf   *TransportClientConfig
	stream pb.Discovery_EventsClient
	conn   TransportConn
}

// GetIn returns In channel of the client.
func (c *Client) GetIn() chan *pb.Event {
	return c.conf.In
}

// GetOut returns Out channel of the client.
func (c *Client) GetOut() chan *pb.Event {
	return c.conf.Out
}

// Connect dials the server and returns a connection.
func (c *Client) Connect() (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(c.conf.Host+":"+c.conf.Port, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(c.conf.Timeout))
	if err != nil {
		c.conf.Logger.Error("error establishing a gRPC connection")
		return nil, err
	}
	c.conn = conn
	return conn, nil
}

// Run starts forwarding of the events. It blocks until the gRPC channel is closed or an error occurs.
func (c *Client) Run(client pb.DiscoveryClient) {
	ctx := c.conf.Context
	ctx = metadata.AppendToOutgoingContext(ctx, ConnID, c.conf.ConnID, EventScope, c.conf.EventScope)
	stream, err := client.Events(ctx)
	if err != nil {
		c.conf.ErrCh <- err
		return
	}
	c.stream = stream

	go c.streamIn()
	go c.streamOut()
}

// Stop closes the underlying gRPC stream and its TCP connection.
func (c *Client) Stop() error {
	c.conf.Logger.Debug("Stopping the gRPC client")
	err := c.stream.CloseSend()
	if err != nil {
		return err
	}
	return c.conn.Close()
}

// streamOut forwards events from Out channel to the server.
// Must be run in a separate go routine.
// Note, the return type is only added for the test purpose. We never return an error,
// it is send to the error channel instead.
func (c *Client) streamOut() error {
	for {
		select {
		case ev := <-c.conf.Out:
			err := c.stream.Send(ev)
			if err != nil {
				c.conf.ErrCh <- err
				return nil
			}
		case <-c.conf.Context.Done():
			return nil
		}
	}
}

// streamIn reads events from the server and forwards them to the In channel.
// Must be run in a separate go routine.
// Note, the return type is only added for the test purpose. We never return an error,
// it is send to the error channel instead.
func (c *Client) streamIn() error {
	defer func() {
		err := c.Stop()
		if err != nil {
			c.conf.Logger.Errorf("error stopping gRPC client %v", err)
		}
	}()
	for {
		select {
		case <-c.conf.Context.Done():
			return nil
		case <-c.stream.Context().Done():
			c.conf.Logger.Errorf("The gRPC stream was closed")
			return nil
		default:
			ev, err := c.stream.Recv()
			if err == io.EOF {
				c.conf.Logger.Debug("server closed the connection")
				return nil
			}
			if err != nil {
				c.conf.Logger.Errorf("error from the gRPC stream %s", err.Error())
				c.conf.ErrCh <- err
				return nil
			}
			c.conf.In <- ev
		}
	}
}

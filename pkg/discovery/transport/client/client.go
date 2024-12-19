// Copyright (c) 2021-2024 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package client

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"time"

	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"

	. "github.com/carbynestack/ephemeral/pkg/types"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// TransportClientConfig preserves config params of the client.
type TransportClientConfig struct {
	// In, Out is the external interface for the libraries that would like to use this client. Events received from "In" are forwarded to the server. The responses are sent back to "Out"
	In, Out chan *pb.Event

	// ErrCh is the sink for all errors from the client. It is supposed to be a buffered channel with a minimum capacity of `1`.
	ErrCh chan error

	// Host, Port - the server endpoint to connect to.
	Host, Port string

	// EventScope defines the scope of events the client subscribes to. "all" - events from all games are current games, "ConnID" - events associated with this connection ID.
	EventScope string

	// ConnID is the ID of the connection. In case of pure discovery clients, it is equal the gameID.
	ConnID string

	// ConnectTimeout is the gRPC dial timeout.
	ConnectTimeout time.Duration

	Logger *zap.SugaredLogger

	Context context.Context

	TlsConfig *tls.Config
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
	ctx, cancelConnect := context.WithTimeout(context.Background(), c.conf.ConnectTimeout)
	defer cancelConnect()

	var opts []grpc.DialOption
	if c.conf.TlsConfig != nil {
		c.conf.Logger.Debug("Using TLS for gRPC connection")
		creds := credentials.NewTLS(c.conf.TlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	conn, err := grpc.DialContext(ctx, c.conf.Host+":"+c.conf.Port, append(opts, grpc.WithBlock())...)
	if err != nil {
		c.conf.Logger.Errorf("Error establishing a gRPC connection: %v", err)
		return nil, err
	}
	c.conn = conn
	c.conf.Logger.Debug("Client gRPC connection established")
	return conn, nil
}

// Run starts forwarding of the events. The functionality is started as separate go routines which run until the given
// context is closed, or a communication error occurs.
func (c *Client) Run(client pb.DiscoveryClient) {
	ctx := c.conf.Context
	ctx = metadata.AppendToOutgoingContext(ctx, ConnID, c.conf.ConnID, EventScope, c.conf.EventScope)
	c.conf.Logger.Debug("Register client to events", ConnID, c.conf.ConnID, EventScope, c.conf.EventScope)
	stream, err := client.Events(ctx)
	if err != nil {
		c.conf.ErrCh <- err
		return
	}
	c.stream = stream

	go func() {
		for {
			select {
			case <-ctx.Done():
				err := c.Stop()
				if err != nil {
					c.conf.Logger.Errorf("Error stopping gRPC client %v", err)
				}
				return
			}
		}
	}()
	go c.streamIn()
	go c.streamOut()
}

// Stop closes the underlying gRPC stream and its TCP connection.
func (c *Client) Stop() error {
	c.conf.Logger.Debug("Stopping client connection")
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
		case <-c.conf.Context.Done():
			c.conf.Logger.Debug("Close the event forwarding as context is done")
			return nil
		case ev := <-c.conf.Out:
			c.conf.Logger.Debugf("Sending event %v", ev)
			err := c.stream.Send(ev)
			if err != nil {
				c.conf.Logger.Errorf("Close the event forwarding as an error occurred: %v", err)
				select {
				case c.conf.ErrCh <- err:
				default:
					// The ErrCh is a buffered channel shared by multiple subroutines. Any error written to the channel
					// indicates that the current procedure has failed.
					// While the "root" error is sufficient to indicate that the routine failed, it may cause further
					// errors in other routines. If write to ErrCh fails, err is classified as a consequent error. In
					// this case, "err" is discarded to prevent the routine from blocking.
				}
				return nil
			}
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
			c.conf.Logger.Errorf("Error stopping gRPC client %v", err)
		}
	}()
	for {
		ev, err := c.stream.Recv()
		select {
		case <-c.conf.Context.Done():
			c.conf.Logger.Debugf("Stop receiiving events as context is done. (err: %v)", err)
			return nil
		default:
			c.conf.Logger.Debugf("Received event %v", ev)
			if err == io.EOF {
				c.conf.Logger.Debug("Server closed the connection")
				return nil
			}
			if err != nil {
				c.conf.Logger.Errorf("Error from the gRPC stream %s", err.Error())
				select {
				case c.conf.ErrCh <- err:
				default:
					// The ErrCh is a buffered channel shared by multiple subroutines. Any error written to the channel
					// indicates that the current procedure has failed.
					// While the "root" error is sufficient to indicate that the routine failed, it may cause further
					// errors in other routines. If write to ErrCh fails, err is classified as a consequent error. In
					// this case, "err" is discarded to prevent the routine from blocking.
				}
				return nil
			}
			c.conf.In <- ev
		}
	}
}

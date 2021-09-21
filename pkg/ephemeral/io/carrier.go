//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package io

import (
	"context"
	"errors"
	"github.com/carbynestack/ephemeral/pkg/amphora"
	"io/ioutil"
	"net"
)

// Result contains the response from SPDZ runtime computation.
type Result struct {
	Response []string `json:"response"`
}

// AbstractCarrier is the carriers interface.
type AbstractCarrier interface {
	Connect(context.Context, string, string) error
	Close() error
	Send([]amphora.SecretShare) error
	Read(ResponseConverter, bool) (*Result, error)
}

// Carrier is a TCP client for TCP sockets.
type Carrier struct {
	Dialer    func(ctx context.Context, addr, port string) (net.Conn, error)
	Conn      net.Conn
	Packer    Packer
	connected bool
}

// Config contains TCP connection properties of Carrier.
type Config struct {
	Port string
	Host string
}

// Connect establishes a TCP connection to a socket on a given host and port.
func (c *Carrier) Connect(ctx context.Context, host, port string) error {
	conn, err := c.Dialer(ctx, host, port)
	if err != nil {
		return err
	}
	c.Conn = conn
	c.connected = true
	return nil
}

// Close closes the underlying TCP connection.
func (c *Carrier) Close() error {
	if c.connected {
		c.Conn.Close()
	}
	return nil
}

// Send transmits Amphora secret shares to a TCP socket opened by an MPC runtime.
func (c *Carrier) Send(secret []amphora.SecretShare) error {
	input := []byte{}
	shares := []string{}
	for i := range secret {
		shares = append(shares, secret[i].Data)
	}
	err := c.Packer.Marshal(shares, &input)
	if err != nil {
		return err
	}
	_, err = c.Conn.Write(input)
	if err != nil {
		return err
	}
	return nil
}

// Read reads the response from the TCP connection and unmarshals it.
func (c *Carrier) Read(conv ResponseConverter, bulkObjects bool) (*Result, error) {
	resp := []byte{}
	resp, err := ioutil.ReadAll(c.Conn)
	if len(resp) == 0 {
		return nil, errors.New("empty result from socket")
	}
	if err != nil {
		return nil, err
	}
	out, err := c.Packer.Unmarshal(&resp, conv, bulkObjects)
	if err != nil {
		return nil, err
	}
	return &Result{Response: out}, nil
}

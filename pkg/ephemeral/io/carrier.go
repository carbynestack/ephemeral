//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package io

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/carbynestack/ephemeral/pkg/amphora"
	"io"
	"io/ioutil"
	"net"
)

// Result contains the response from SPDZ runtime computation.
type Result struct {
	Response []string `json:"response"`
}

// AbstractCarrier is the carriers interface.
type AbstractCarrier interface {
	Connect(context.Context, int32, string, string) error
	Close() error
	Send([]amphora.SecretShare) error
	Read(ResponseConverter, bool) (*Result, error)
}

// Carrier is a TCP client for TCP sockets.
type Carrier struct {
	Dialer       func(ctx context.Context, addr, port string) (net.Conn, error)
	TLSConnector func(conn net.Conn, playerID int32) (net.Conn, error)
	Conn         net.Conn
	Packer       Packer
	connected    bool
}

// Config contains TCP connection properties of Carrier.
type Config struct {
	Port string
	Host string
}

// Connect establishes a TCP connection to a socket on a given host and port.
func (c *Carrier) Connect(ctx context.Context, playerID int32, host string, port string) error {
	conn, err := c.Dialer(ctx, host, port)
	if err != nil {
		return err
	}
	_, err = conn.Write(c.buildHeader(playerID))
	if err != nil {
		return err
	}
	c.Conn, err = c.TLSConnector(conn, playerID)
	if err != nil {
		return err
	}
	if playerID == 0 {
		err = c.readSpec()
		if err != nil {
			return err
		}
	}
	c.connected = true
	return nil
}

func (c Carrier) readSpec() error {
	const size = 4
	readBytes := make([]byte, size)
	_, err := io.LimitReader(c.Conn, size).Read(readBytes)
	if err != nil {
		return err
	}

	sizeOfHeader := binary.LittleEndian.Uint32(readBytes)
	readBytes = make([]byte, sizeOfHeader)
	_, err = io.LimitReader(c.Conn, int64(sizeOfHeader)).Read(readBytes)
	if err != nil {
		return err
	}
	//ToDo, compare read PRIME with prime number from config?
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

// Returns a new Slice with the header appended
// The header consists of the clientId as string:
// - 1 Long (4 Byte) that contains the length of the string in bytes
// - Then come X Bytes for the String
func (c *Carrier) buildHeader(playerID int32) []byte {
	playerIDString := []byte(fmt.Sprintf("%d", playerID))
	lengthOfString := make([]byte, 4)
	binary.LittleEndian.PutUint32(lengthOfString, uint32(len(playerIDString)))
	return append(lengthOfString, playerIDString...)
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

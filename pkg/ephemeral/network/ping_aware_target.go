// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package network

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/google/tcpproxy"
	"go.uber.org/zap"
)

const (
	// PingMsg - message received from the clients.
	PingMsg = "ping"
	// PongMsg - message sent back to the clients.
	PongMsg = "pong"
)

// PingAwareTarget is a target that notifies about the start of the listener by means of the ping message.
type PingAwareTarget struct {
	Next   tcpproxy.Target
	Logger *zap.SugaredLogger
}

// HandleConn checks whether the byte stream contains ping.
// If it is a ping, we respond back with pong indicating the listener has started.
// The sender of the ping is responsible for closing the connection.
// In case of no ping, the data is forwarded to the Next Target.
func (n *PingAwareTarget) HandleConn(conn net.Conn) {
	br := bufio.NewReader(conn)
	ping, err := n.read(br, PingMsg)
	if err != nil {
		n.Logger.Errorf("error while handling a ping message: %s", err)
		conn.Close()
		return
	}
	if ping > 0 {
		n.Logger.Debug("Received a ping message")
		br.Discard(ping)
		pong := []byte(PongMsg)
		_, err := conn.Write(pong)
		if err != nil {
			n.Logger.Errorf("error while writing pong message: %s", err)
		}
		n.Logger.Debug("Responded with pong message")
		return
	}
	n.Logger.Debug("No ping message received - process as usual")
	// Copy the read data back to the connection.
	if n := br.Buffered(); n > 0 {
		peeked, _ := br.Peek(br.Buffered())
		conn = &tcpproxy.Conn{
			HostName: "",
			Peeked:   peeked,
			Conn:     conn,
		}
	}
	n.Next.HandleConn(conn)
}

// WaitUntilStarted pings a proxy, waits for the pong and closes the connection.
func (n *PingAwareTarget) WaitUntilStarted(address, port string, timeout time.Duration, dial func(string, string) (net.Conn, error)) (bool, error) {
	conn, err := dial(address, port)
	if err != nil {
		return false, err
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			n.Logger.Errorf("error closing ping connection", err)
		}
		n.Logger.Debug("Closing the ping connection")
	}()
	_, err = conn.Write([]byte(PingMsg))
	if err != nil {
		return false, err
	}
	br := bufio.NewReader(conn)
	pong, err := n.read(br, PongMsg)
	if err != nil {
		return false, err
	}
	if pong > 0 {
		return true, nil
	}
	return false, nil
}

// readPing reads the first 4 bytes from the buffer and checks whether it contains the ping message.
// It returns the size of the ping message if it succeeded reading it from the buffer, 0 otherwise.
func (n *PingAwareTarget) read(br *bufio.Reader, match string) (int, error) {
	pingSize := 4 // bytes
	n.Logger.Debugf("Checking if the next %d bytes contain the ping header", pingSize)
	msg, err := br.Peek(pingSize)
	if err != nil {
		if err != io.EOF {
			return 0, err
		}
		return 0, fmt.Errorf("received less data then the provided size - %s", err)
	}
	if string(msg) == match {
		return pingSize, nil
	}
	return 0, nil
}

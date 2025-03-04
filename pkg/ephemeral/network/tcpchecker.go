// Copyright (c) 2021-2025 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package network

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"

	"go.uber.org/zap"
)

// NetworkChecker verifies the network connectivity between the players before starting the computation.
type NetworkChecker interface {
	Verify(context.Context, string, string) error
}

// NoopChecker verifies the network for all MPC players is in place.
type NoopChecker struct {
}

// Verify checks network connectivity between the players and communicates its results to discovery and players FSM.
func (t *NoopChecker) Verify(context.Context, string, string) error {
	return nil
}

// TCPCheckerConf is the configuration of TCPChecker
type TCPCheckerConf struct {
	DialTimeout  time.Duration
	RetryTimeout time.Duration
	Logger       *zap.SugaredLogger
	TlsConfig    *tls.Config
}

// NewTCPChecker returns an instance of TCPChecker
func NewTCPChecker(conf *TCPCheckerConf) *TCPChecker {
	return &TCPChecker{
		conf: conf,
	}
}

// TCPChecker verifies the network for all MPC players is in place.
type TCPChecker struct {
	conf    *TCPCheckerConf
	retries int32
}

// Verify checks network connectivity between the players and communicates its results to discovery and players FSM.
func (t *TCPChecker) Verify(ctx context.Context, host, port string) error {
	done := time.After(t.conf.RetryTimeout)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("TCPCheck for '%s:%s' aborted after %d attempts", host, port, t.retries)
		case <-done:
			return fmt.Errorf("TCPCheck for '%s:%s' failed after %s and %d attempts", host, port, t.conf.RetryTimeout.String(), t.retries)
		default:
			if t.tryToConnect(host, port) {
				return nil
			}
			t.sleepAndIncrement()
		}
	}
}

// tryToConnect spins up a new TCP connection, returns true if the connection succeeds, false otherwise.
// The exact errors are not returned, but printed out instead.
func (t *TCPChecker) tryToConnect(host, port string) bool {
	var conn net.Conn
	var err error
	defer func() {
		if conn != nil {
			err := conn.Close()
			if err != nil {
				t.conf.Logger.Error(err)
			}
		}
	}()
	if t.conf.TlsConfig != nil {
		dialer := &net.Dialer{Timeout: t.conf.DialTimeout}
		t.conf.Logger.Debugf("Attempting to establish mTLS connection to %s:%s", host, port)
		conn, err = tls.DialWithDialer(dialer, "tcp", host+":"+port, t.conf.TlsConfig)
		if err != nil {
			t.conf.Logger.Debugf("Error getting tcp connection using mTLS %s", err.Error())
			conn = nil // Ensure conn is nil to avoid defer panic
			return false
		}
	} else {
		t.conf.Logger.Debugf("Attempting to establish TCP connection to %s:%s", host, port)
		conn, err = net.DialTimeout("tcp", host+":"+port, t.conf.DialTimeout) // Here is TCP connection established
		if err != nil {
			t.conf.Logger.Debugf("Error getting tcp connection %s", err.Error())
			return false
		}
	}
	err = conn.SetReadDeadline(time.Now().Add(t.conf.DialTimeout))
	if err != nil {
		t.conf.Logger.Errorf("Error setting read deadline, %s\n", err.Error())

		return false
	}
	arr := make([]byte, 1)
	_, err = conn.Read(arr)

	if err != nil {
		if err == io.EOF {
			// This is when Istio network is configured, but player's SPDZ binary is not started.
			t.conf.Logger.Debugf("TCPCheck - error connection closed %s", t.conf.DialTimeout)
			// trigger a retry.
			return false
		} else if neterr, ok := err.(net.Error); ok && neterr.Timeout() {
			// We do not expect to read anything from the socket here, so the timeout is a success.
			t.conf.Logger.Debug("TCPCheck - connection established")
			// success, no retries are expected anymore.
			return true
		} else {
			t.conf.Logger.Errorf("TCPCheck - exit on unknown error: %s", err.Error())
			return false
		}
	}
	return true
}

// sleepAndIncrement sleeps for the period of DialTimeout, increments the number of retries and prints out a log entry.
func (t *TCPChecker) sleepAndIncrement() {
	t.retries++
	time.Sleep(t.conf.DialTimeout)
	t.conf.Logger.Debugf("Retrying TCPCheck after %s", t.conf.DialTimeout)
}

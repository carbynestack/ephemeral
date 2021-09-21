//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	. "github.com/carbynestack/ephemeral/pkg/types"

	"github.com/google/tcpproxy"
	"go.uber.org/zap"
)

const (
	timeout = 20 * time.Second
)

// AbstractProxy is an interface for proxy.
type AbstractProxy interface {
	Run(*CtxConfig, chan error) error
	Stop()
}

// NewProxy returns a new instance of ephemeral proxy.
func NewProxy(lg *zap.SugaredLogger, conf *SPDZEngineTypedConfig, checker NetworkChecker) *Proxy {
	return &Proxy{
		logger:       lg,
		retrySleep:   conf.RetrySleep,
		retryTimeout: conf.RetryTimeout,
		tcpChecker:   checker,
	}
}

// Proxy is a wrapper around the tcpproxy and ping aware proxy.
// It establishes the connection between MPC master and slave.
type Proxy struct {
	logger       *zap.SugaredLogger
	retrySleep   time.Duration
	retryTimeout time.Duration
	proxy        *tcpproxy.Proxy
	ctx          *CtxConfig
	tcpChecker   NetworkChecker
}

// Run start the tcpproxy, makes sure it has started by means of a ping.
func (p *Proxy) Run(ctx *CtxConfig, errCh chan error) error {
	p.proxy = &tcpproxy.Proxy{}
	p.ctx = ctx
	// Start the TCP proxy to forward the requests from the base partner address to the target one.
	p.logger.Infow(fmt.Sprintf("Starting TCP Proxy with the following parameters: %s", p.ctx.Proxy), GameID, p.ctx.Act.GameID)
	address := p.ctx.Proxy.Host + ":" + p.ctx.Proxy.Port
	dialProxy := tcpproxy.DialProxy{Addr: address, DialTimeout: timeout}
	pat := &PingAwareTarget{
		Next:   &dialProxy,
		Logger: p.logger,
	}
	p.proxy.AddRoute(":"+p.ctx.Proxy.LocalPort, pat)
	// As long as the number of players equals 2, the first one is the master who does the TCP check.
	if p.ctx.Spdz.PlayerID == int32(0) {
		err := p.tcpChecker.Verify(p.ctx.Proxy.Host, p.ctx.Proxy.Port)
		if err != nil {
			return fmt.Errorf("error checking connection to the slave: %s", err)
		}
		p.logger.Info("TCP check is OK")
	}
	go func() {
		err := p.proxy.Run()
		errCh <- err
	}()
	dialer := RetryingDialer(p.retrySleep, p.retryTimeout, func() {
		p.logger.Debugw(fmt.Sprintf("retrying to ping after %s", p.retrySleep), GameID, p.ctx.Act.GameID)
	})
	_, err := pat.WaitUntilStarted("", p.ctx.Proxy.LocalPort, timeout, dialer)
	if err != nil {
		return err
	}
	return nil
}

// Stop closes the underlying tcpproxy and waits for it to finish.
func (p *Proxy) Stop() {
	p.logger.Debugw("Waiting for TCP proxy to stop", GameID, p.ctx.Act.GameID)
	p.proxy.Close()
	p.proxy.Wait()
	p.logger.Debugw("Stopped the TCP proxy", GameID, p.ctx.Act.GameID)
}

// RetryingDialer tries to establish a TCP connection to a socket until the timeout is reached.
func RetryingDialer(sleep, timeout time.Duration, sideEffect func()) func(addr, port string) (conn net.Conn, err error) {
	return func(addr, port string) (conn net.Conn, err error) {
		started := time.Now()
		for {
			var tcpAddr *net.TCPAddr
			tcpAddr, err = net.ResolveTCPAddr("tcp", addr+":"+port)
			if err != nil {
				return nil, err
			}
			conn, err = net.DialTCP("tcp", nil, tcpAddr)
			if err != nil && time.Now().Sub(started) < timeout {
				sideEffect()
				time.Sleep(sleep)
				continue
			}
			break
		}
		return conn, err
	}
}

// RetryingDialerWithContext tries to establish a TCP connection to a socket until the timeout is reached or the context is cancelled.
func RetryingDialerWithContext(sleep, timeout time.Duration, sideEffect func()) func(ctx context.Context, addr, port string) (conn net.Conn, err error) {
	return func(ctx context.Context, addr, port string) (conn net.Conn, err error) {
		started := time.Now()
		for {
			select {
			case <-ctx.Done():
				return conn, errors.New("context cancelled")
			default:
				var tcpAddr *net.TCPAddr
				tcpAddr, err = net.ResolveTCPAddr("tcp", addr+":"+port)
				if err != nil {
					return nil, err
				}
				conn, err = net.DialTCP("tcp", nil, tcpAddr)
				if err != nil && time.Now().Sub(started) < timeout {
					sideEffect()
					time.Sleep(sleep)
					continue
				}
				return conn, err
			}
		}
	}
}

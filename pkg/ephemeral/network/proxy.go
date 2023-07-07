// Copyright (c) 2021-2023 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
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
		retryTimeout: conf.NetworkEstablishTimeout,
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
	// activeProxyIndicatorCh indicates that proxy was successfully started (see [tcpproxy.Proxy.Start]) if the channel
	// is closed.
	activeProxyIndicatorCh chan struct{}
}

// Run start the tcpproxy, makes sure it has started by means of a ping.
func (p *Proxy) Run(ctx *CtxConfig, errCh chan error) error {
	p.proxy = &tcpproxy.Proxy{}
	p.ctx = ctx

	var pats []*PingAwareTarget
	for _, proxyEntry := range ctx.ProxyEntries {
		pat := p.addProxyEntry(proxyEntry)
		pats = append(pats, pat)
	}

	err := p.checkConnectionToPeers()
	if err != nil {
		return err
	}

	p.logger.Infow("Starting TCP Proxy", GameID, ctx.Act.GameID)
	go func() {
		defer close(errCh)
		p.activeProxyIndicatorCh = make(chan struct{})
		err := p.proxy.Start()
		if err == nil {
			close(p.activeProxyIndicatorCh)
			err = p.proxy.Wait()
		}
		errCh <- err
	}()
	dialer := RetryingDialer(p.retrySleep, p.retryTimeout, func() {
		p.logger.Debugw(fmt.Sprintf("Retrying to ping after %s", p.retrySleep), GameID, p.ctx.Act.GameID)
	})

	for i, pat := range pats {
		localPort := p.ctx.ProxyEntries[i].LocalPort
		p.logger.Info(fmt.Sprintf("Waiting until proxyEntry is started for local Port %s", localPort))
		_, err := pat.WaitUntilStarted("", localPort, timeout, dialer)
		if err != nil {
			return err
		}
	}
	return nil
}

// checkConnectionToPeers verifies that all peers can communicate with each other.
// Since the connectivity check requires some Proxy's to be already running, each connection between two parties is only checked one way!
//
// This implementation assumes that Proxy.ctx is set.
// This implementation assumes that Proxy.ctx.ProxyEntries is ordered by playerId
func (p *Proxy) checkConnectionToPeers() error {
	var waitGroup sync.WaitGroup
	var errorsCheckingConnection []error

	// Check fully connected Graph, each edge checked once
	// Player i connects to all in [i+1, N]
	// Assumes that ctx.ProxyEntries is sorted by PlayerId
	for _, proxyEntry := range p.ctx.ProxyEntries[p.ctx.Spdz.PlayerID:] {
		proxyEntry := proxyEntry
		waitGroup.Add(1)
		go func() {
			err := p.checkTCPConnectionToPeer(proxyEntry)
			defer waitGroup.Done()
			if err != nil {
				errorsCheckingConnection = append(errorsCheckingConnection, err)
			}
		}()
	}

	// Wait for all proxy connections to be completed
	waitGroup.Wait()

	if len(errorsCheckingConnection) > 0 {
		message := fmt.Sprintf("could not connect to %d proxies", len(errorsCheckingConnection))
		p.logger.Errorw(message, "errors", errorsCheckingConnection)
		return errors.New(message)
	}
	return nil
}

func (p *Proxy) addProxyEntry(config *ProxyConfig) *PingAwareTarget {
	// Start the TCP proxy to forward the requests from the base partner address to the target one.
	address := config.Host + ":" + config.Port
	p.logger.Infow(fmt.Sprintf("Adding TCP Proxy Entry for 'localhost:%s' -> '%s'", config.LocalPort, address), GameID, p.ctx.Act.GameID)
	dialProxy := tcpproxy.DialProxy{Addr: address, DialTimeout: timeout}
	pat := &PingAwareTarget{
		Next:   &dialProxy,
		Logger: p.logger,
	}
	p.proxy.AddRoute(":"+config.LocalPort, pat)
	return pat
}

func (p *Proxy) checkTCPConnectionToPeer(config *ProxyConfig) error {
	p.logger.Info(fmt.Sprintf("Checking if connection to peer works for config: %s", config))
	err := p.tcpChecker.Verify(config.Host, config.Port)
	if err != nil {
		return fmt.Errorf("error checking connection to the peer '%s:%s': %s", config.Host, config.Port, err)
	}
	p.logger.Info(fmt.Sprintf("TCP check to '%s:%s' is OK", config.Host, config.Port))
	return nil
}

// Stop closes the underlying tcpproxy and waits for it to finish.
func (p *Proxy) Stop() {
	p.logger.Debugw("Waiting for TCP proxy to stop", GameID, p.ctx.Act.GameID)
	p.proxy.Close()
	select {
	case <-p.activeProxyIndicatorCh:
		p.proxy.Wait()
	default:
	}
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
func RetryingDialerWithContext(sleep time.Duration, l *zap.SugaredLogger) func(ctx context.Context, addr, port string, timeout time.Duration) (conn net.Conn, err error) {
	return func(ctx context.Context, addr, port string, timeout time.Duration) (conn net.Conn, err error) {
		started := time.Now()
		logTicker := time.NewTicker(5 * time.Second)
		connectTimer := time.NewTimer(0)
		defer logTicker.Stop()
		defer connectTimer.Stop()
		for {
			select {
			case <-ctx.Done():
				return conn, errors.New(fmt.Sprintf("cancelled connection attempt for %s:%s - context done", addr, port))
			case <-logTicker.C:
				l.Debugf("Connection attempt to %s:%s active for %s", addr, port, time.Now().Sub(started))
			case <-connectTimer.C:
				var tcpAddr *net.TCPAddr
				tcpAddr, err = net.ResolveTCPAddr("tcp", addr+":"+port)
				if err != nil {
					return nil, err
				}
				conn, err = net.DialTCP("tcp", nil, tcpAddr)
				if err != nil && time.Now().Sub(started) < timeout {
					connectTimer.Reset(sleep)
					continue
				}
				if conn != nil {
					if err := conn.(*net.TCPConn).SetKeepAlive(true); err != nil {
						return nil, err
					}
				}
				l.Debugw("Dialer done", "Conn", conn, "Err", err)
				return conn, err
			}
		}
	}
}

//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package network

import (
	"bufio"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/google/tcpproxy"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var _ = Describe("PingAwareTarget", func() {
	It("forwards the packets without modification", func() {
		lg := zap.NewNop().Sugar()
		noPing := "gnip"
		address := "localhost:9999"
		localAddr := ":8888"
		proxy := tcpproxy.Proxy{}
		target := tcpproxy.DialProxy{Addr: address, DialTimeout: time.Second * 20}
		pat := &PingAwareTarget{
			Next:   &target,
			Logger: lg,
		}
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer func() {
				GinkgoRecover()
				wg.Done()
			}()
			ln, err := net.Listen("tcp", ":9999")
			conn, err := ln.Accept()
			if err != nil {
				panic(err)
			}
			br := bufio.NewReader(conn)
			i, err := pat.read(br, noPing)
			Expect(err).NotTo(HaveOccurred())
			Expect(i).To(Equal(4))
			err = ln.Close()
			Expect(err).NotTo(HaveOccurred())
		}()

		dialer := RetryingDialer(50*time.Millisecond, 60*time.Second, func() {})
		// Forward packets from :8888 to :9999
		proxy.AddRoute(localAddr, pat)
		proxy.Start()
		ok, err := pat.WaitUntilStarted("", "8888", time.Second*2, dialer)
		Expect(err).To(BeNil())
		Expect(ok).To(BeTrue())
		conn, err := net.Dial("tcp", localAddr)
		Expect(err).NotTo(HaveOccurred())
		_, err = conn.Write([]byte(noPing))
		Expect(err).NotTo(HaveOccurred())
		conn.Close()
		wg.Wait()
	})
	Context("when waiting for PingAwareTarget to start", func() {
		Context("when dialing fails", func() {
			It("returns an error", func() {
				dialer := func(host, port string) (net.Conn, error) {
					return nil, errors.New("dialing error")
				}
				p := &PingAwareTarget{
					Logger: zap.NewNop().Sugar(),
				}
				ok, err := p.WaitUntilStarted("", "", time.Millisecond, dialer)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("dialing error"))
				Expect(ok).To(BeFalse())
			})
		})
		Context("when writing pong message fails", func() {
			It("returns an error", func() {
				server, client := net.Pipe()
				// writing on the closed connection must not succeed.
				server.Close()
				dialer := func(host, port string) (net.Conn, error) {
					return client, nil
				}
				p := &PingAwareTarget{
					Logger: zap.NewNop().Sugar(),
				}
				ok, err := p.WaitUntilStarted("", "", time.Millisecond, dialer)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("io: read/write on closed pipe"))
				Expect(ok).To(BeFalse())
			})
		})
	})
})

type DeadendHandler struct {
}

func (d *DeadendHandler) HandleConn(conn net.Conn) {
	return
}

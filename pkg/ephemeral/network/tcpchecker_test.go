// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package network

import (
	"io"
	"net"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var _ = Describe("TcpChecker", func() {

	const (
		host = "localhost"
		port = "9999"
	)

	It("returns nil if dialing TCP succeeds", func() {
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			var ln net.Listener
			defer func() {
				ln.Close()
				wg.Done()
			}()
			ln, _ = net.Listen("tcp", host+":"+port)
			conn, err := ln.Accept()
			Expect(err).NotTo(HaveOccurred())
			buf := make([]byte, 1)
			_, err = conn.Read(buf)
			// Expect the connection to be closed.
			Expect(err).To(Equal(io.EOF))
			Expect(len(buf)).To(Equal(1))
			Expect(buf[0]).To(Equal(uint8(0)))
		}()

		conf := &TCPCheckerConf{
			DialTimeout:  1 * time.Second,
			RetryTimeout: 2 * time.Second,
			Logger:       zap.NewNop().Sugar(),
		}
		checker := NewTCPChecker(conf)
		err := checker.Verify(host, port)
		Expect(err).NotTo(HaveOccurred())
		wg.Wait()
	})
	It("return an error if dialing fails", func() {
		conf := &TCPCheckerConf{
			DialTimeout:  50 * time.Millisecond,
			RetryTimeout: 50 * time.Millisecond,
			Logger:       zap.NewNop().Sugar(),
		}
		checker := NewTCPChecker(conf)
		err := checker.Verify(host, port)
		Expect(err).To(HaveOccurred())
	})
	It("returns an error if dialing succeeds but the connection is closed down shortly", func() {
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
				GinkgoRecover()
			}()
			ln, err := net.Listen("tcp", ":"+port)
			// Allow dialing to succeed and then close the connection.
			conn, err := ln.Accept()
			if err != nil {
				panic(err)
			}
			err = conn.Close()
			Expect(err).NotTo(HaveOccurred())
			err = ln.Close()
			Expect(err).NotTo(HaveOccurred())
		}()
		conf := &TCPCheckerConf{
			DialTimeout:  50 * time.Millisecond,
			RetryTimeout: 100 * time.Millisecond,
			Logger:       zap.NewNop().Sugar(),
		}
		checker := NewTCPChecker(conf)
		err := checker.Verify(host, port)
		Expect(err).To(HaveOccurred())
		Expect(checker.retries > 1).To(BeTrue())
		wg.Wait()

	})
	It("retries to connect if the first attempt fails", func() {
		conf := &TCPCheckerConf{
			DialTimeout:  50 * time.Millisecond,
			RetryTimeout: 100 * time.Millisecond,
			Logger:       zap.NewNop().Sugar(),
		}
		checker := NewTCPChecker(conf)
		err := checker.Verify(host, port)
		Expect(err).To(HaveOccurred())
		Expect(checker.retries > 1).To(BeTrue())
	})
})

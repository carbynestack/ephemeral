// Copyright (c) 2021-2023 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package network

import (
	"context"
	. "github.com/carbynestack/ephemeral/pkg/types"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var _ = Describe("Proxy", func() {

	Context("when starting and stopping the proxy", func() {
		It("succeeds", func() {
			logger := zap.NewNop().Sugar()
			tcpChecker := &NoopChecker{}
			spdzConfig := &SPDZEngineTypedConfig{
				PlayerID:                0,
				RetrySleep:              50 * time.Millisecond,
				NetworkEstablishTimeout: 10 * time.Second,
			}
			p := NewProxy(logger, spdzConfig, tcpChecker)
			ctx := &CtxConfig{
				Act: &Activation{
					GameID: "71b2a100-f3f6-11e9-81b4-2a2ae2dbcce4",
				},
				ProxyEntries: []*ProxyConfig{
					{
						Host:      "localhost",
						Port:      "5001",
						LocalPort: "5000",
					},
				},
				Spdz: spdzConfig,
			}
			errCh := make(chan error, 1)
			err := p.Run(ctx, errCh)
			Expect(err).To(BeNil())
			p.Stop()
		})
	})
	Context("when using the retrying dialer", func() {
		It("quits after a timeout", func() {
			counter := 0
			side := func() {
				counter++
			}
			dialer := RetryingDialer(1*time.Millisecond, 50*time.Millisecond, side)
			conn, err := dialer("localhost", "5555")
			Expect(conn).To(BeNil())
			Expect(counter).To(BeNumerically(">", 0))
			Expect(err).To(HaveOccurred())
		})
		It("fails if address cannot be resolved", func() {
			dialer := RetryingDialer(1*time.Millisecond, 50*time.Millisecond, func() {})
			conn, err := dialer("inva<l>id", "5555")
			Expect(conn).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("lookup inva<l>id: no such host"))
		})
	})
	Context("when using the retrying dialer with context", func() {
		It("fails if address cannot be resolved", func() {
			ctx := context.TODO()
			logger := zap.NewNop().Sugar()
			dialer := RetryingDialerWithContext(1*time.Millisecond, 50*time.Millisecond, logger)
			conn, err := dialer(ctx, "inva<l>id", "5555")
			Expect(conn).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("lookup inva<l>id: no such host"))
		})
		It("returns error when context is done", func() {
			logger := zap.NewNop().Sugar()
			ctx, cancel := context.WithCancel(context.TODO())
			cancel()
			dialer := RetryingDialerWithContext(0, 0, logger)
			conn, err := dialer(ctx, "localhost", "5555")
			Expect(conn).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("cancelled connection attempt for localhost:5555 - context done"))
		})
	})
	Context("when using the retrying dialer with context", func() {
		It("periodically logs status messages", func() {
			core, recorded := observer.New(zapcore.DebugLevel)
			logger := zap.New(core).Sugar()
			ctx := context.TODO()
			dialer := RetryingDialerWithContextAndLogTimeout(100*time.Millisecond, 50*time.Millisecond, logger, 5*time.Millisecond)
			conn, err := dialer(ctx, "localhost", "5555")
			Expect(conn).To(BeNil())
			Expect(err).To(HaveOccurred())
			var logs []string
			for _, l := range recorded.All() {
				if strings.HasPrefix(l.Entry.Message, "Connection attempt") {
					logs = append(logs, l.Message)
				}
			}
			Expect(recorded.Len() > 1).To(BeTrue())
		})
	})
})

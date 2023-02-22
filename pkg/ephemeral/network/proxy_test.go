// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package network

import (
	. "github.com/carbynestack/ephemeral/pkg/types"
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
				PlayerID:     0,
				RetrySleep:   50 * time.Millisecond,
				RetryTimeout: 10 * time.Second,
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
	})
})

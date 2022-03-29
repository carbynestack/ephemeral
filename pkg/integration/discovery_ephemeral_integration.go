//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package integration

import (
	"context"
	"fmt"
	discovery "github.com/carbynestack/ephemeral/pkg/discovery"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"github.com/carbynestack/ephemeral/pkg/discovery/transport/server"
	p "github.com/carbynestack/ephemeral/pkg/ephemeral"
	"github.com/carbynestack/ephemeral/pkg/ephemeral/io"
	. "github.com/carbynestack/ephemeral/pkg/types"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	mb "github.com/vardius/message-bus"
	"go.uber.org/zap"
)

const frontendAddress = "23.97.246.132"

var _ = Describe("Ephemeral integration test", func() {
	generateEphemeralIntegrationTestsWithPlayerCount(2)
	generateEphemeralIntegrationTestsWithPlayerCount(5)
})

func generateEphemeralIntegrationTestsWithPlayerCount(playerCount int) {
	// Please note, this test doesn't require a real k8s cluster with ephemeral, it runs locally.
	Context("when connecting ephemeral to discovery", func() {
		It("finishes the game successfully", func() {
			port := "8080"
			conf := &io.Config{
				Host: "localhost",
				Port: port,
			}
			logger := zap.NewNop().Sugar()
			doneCh := make(chan struct{})
			spdz := &FakeSPDZEngine{
				doneCh: doneCh,
			}
			bus := mb.New(10000)
			in := make(chan *pb.Event)
			out := make(chan *pb.Event)
			errCh := make(chan error)
			serverConf := &server.TransportConfig{
				In:     in,
				Out:    out,
				ErrCh:  errCh,
				Port:   port,
				Logger: logger,
			}
			tr := server.NewTransportServer(serverConf)
			pb := discovery.NewPublisher(bus)
			stateTimeout := 10 * time.Second
			n := &discovery.FakeNetworker{
				FreePorts: []int32{30000, 30001, 30002, 30003, 30004, 30005},
			}
			cl := &discovery.FakeDClient{}
			s := discovery.NewServiceNG(bus, pb, stateTimeout, tr, n, frontendAddress, logger, ModeMaster, cl, playerCount)
			defer s.Stop()
			go s.Start()
			s.WaitUntilReady(5 * time.Second)

			act := &Activation{
				GameID: "0",
			}

			players := make([]*p.PlayerWithIO, playerCount)
			for i := 0; i < playerCount; i++ {
				ctxConf := &CtxConfig{
					Act: act,
					Spdz: &SPDZEngineTypedConfig{
						FrontendURL: frontendAddress,
						PlayerID:    int32(i),
					},
					Context: context.TODO(),
				}

				pod := fmt.Sprintf("abc%d", i)
				player, err := p.NewPlayerWithIO(ctxConf, conf, pod, spdz, errCh, logger)
				Expect(err).NotTo(HaveOccurred())
				players[i] = player
			}

			for _, player := range players {
				player.Start()
			}
			for range players {
				<-doneCh
			}
		})
	})
}

type FakeSPDZEngine struct {
	doneCh chan struct{}
}

func (s *FakeSPDZEngine) Execute(event *pb.Event) error {
	s.doneCh <- struct{}{}
	return nil
}

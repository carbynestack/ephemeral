// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package ephemeral

import (
	"context"
	"github.com/carbynestack/ephemeral/pkg/discovery"
	"github.com/carbynestack/ephemeral/pkg/discovery/fsm"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"time"

	. "github.com/carbynestack/ephemeral/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	mb "github.com/vardius/message-bus"
	"go.uber.org/zap"
)

var _ = Describe("Forwarder", func() {
	Context("when forwarding events to player", func() {
		var (
			inCh       chan *pb.Event
			outCh      chan *pb.Event
			doneCh     chan struct{}
			bus        mb.MessageBus
			spdz       MPCEngine
			params     *PlayerParams
			logger     *zap.SugaredLogger
			forwarder  *Forwarder
			playerName = "0"
			timeout    = 10 * time.Second
		)

		BeforeEach(func() {
			inCh = make(chan *pb.Event, 1)
			outCh = make(chan *pb.Event, 1)
			doneCh = make(chan struct{})
			bus = mb.New(10000)
			spdz = &FakeSPDZEngine{}
			playerName = "0"
			params = &PlayerParams{}
			logger = zap.NewNop().Sugar()
			conf := &ForwarderConf{
				Logger: logger,
				InCh:   inCh,
				OutCh:  outCh,
				Topic:  playerName,
			}
			forwarder = NewForwarder(conf)
		})

		It("forwards events in both directions", func() {
			ctx := context.TODO()
			testEvent := "test"
			pl, _ := NewPlayer(ctx, bus, timeout, spdz, params, logger)
			event := &pb.Event{
				Name: testEvent,
			}
			forwarder.conf.Ctx = ctx
			forwarder.conf.Player = pl
			discovery.Assert(testEvent, pl, doneCh, func([]string) {}, playerName)
			go forwarder.Run()
			inCh <- event
			discovery.WaitDoneOrTimeout(doneCh, 10*time.Second)
			grpcEvent := &fsm.Event{
				Name: testEvent,
				Meta: &fsm.Metadata{
					TransportMsg: &pb.Event{},
				},
			}
			pl.Bus().Publish(DiscoveryTopic, grpcEvent)
			<-forwarder.conf.OutCh
		})
		Context("when the context is canceled", func() {
			It("stops the player", func() {
				ctx, cancel := context.WithCancel(context.Background())
				pl, _ := NewPlayer(ctx, bus, timeout, spdz, params, logger)
				cancel()
				forwarder.conf.Ctx = ctx
				forwarder.conf.Player = pl
				err := forwarder.Run()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

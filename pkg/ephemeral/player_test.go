// Copyright (c) 2021-2023 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package ephemeral

import (
	"context"
	"time"

	. "github.com/carbynestack/ephemeral/pkg/discovery"

	. "github.com/carbynestack/ephemeral/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	mb "github.com/vardius/message-bus"
	"go.uber.org/zap"
)

var _ = Describe("Player", func() {

	var (
		bus     mb.MessageBus
		id      string
		done    chan struct{}
		timeout = 10 * time.Second
		me      FakeSPDZEngine
		errCh   chan error
		params  *PlayerParams
		logger  = zap.NewNop().Sugar()
		ctx     context.Context
	)
	BeforeEach(func() {
		bus = mb.New(10000)
		id = "player0"
		done = make(chan struct{})
		params = &PlayerParams{
			PlayerID: 1,
			Players:  2,
			Pod:      "pod",
			IP:       "x.x.x.x",
			GameID:   "71b2a100-f3f6-11e9-81b4-2a2ae2dbcce4",
			Name:     id,
		}
		me = FakeSPDZEngine{}
		errCh = make(chan error)
		ctx = context.TODO()
	})

	Context("when game is successful", func() {
		It("notifies discovery and transitions to PlayerFinishedWithSuccess", func() {
			client := NewFakeDiscoveryClient(bus, id)
			pl, _ := NewPlayer(ctx, bus, timeout, timeout, &me, params, errCh, logger)
			client.Run()
			Assert(GameFinishedWithSuccess, pl, done, func(states []string) {
				Expect(states[0]).To(Equal(Init))
				Expect(states[1]).To(Equal(Registering))
				Expect(states[2]).To(Equal("Playing"))
				Expect(states[3]).To(Equal("PlayerFinishedWithSuccess"))
			})
			pl.Init()
			WaitDoneOrTimeout(done)
		})
	})
	Context("when the game failed", func() {
		It("transitions to the PlayerDone state", func() {
			client := NewFakeDiscoveryClient(bus, id)
			me := BrokenSPDZEngine{}
			pl, _ := NewPlayer(ctx, bus, timeout, timeout, &me, params, errCh, logger)
			client.Run()
			Assert(PlayerDone, pl, done, func(states []string) {
				Expect(states[0]).To(Equal(Init))
				Expect(states[1]).To(Equal(Registering))
				Expect(states[2]).To(Equal(Playing))
				Expect(states[3]).To(Equal(PlayerFinishedWithError))
				Expect(states[4]).To(Equal(PlayerDone))
			}, ServiceEventsTopic)
			pl.Init()
			WaitDoneOrTimeout(done)
		})
	})

	Context("when GameError is received from the discovery service", func() {
		Context("in Registering state", func() {
			It("transitions to the PlayerDone state", func() {
				client := NewFakeBrokenDiscoveryClient(bus, id, false, false)
				pl, _ := NewPlayer(ctx, bus, timeout, timeout, &me, params, errCh, logger)
				client.Run()
				Assert(PlayerDone, pl, done, func(states []string) {
					Expect(states[0]).To(Equal(Init))
					Expect(states[1]).To(Equal(Registering))
					Expect(states[2]).To(Equal(PlayerFinishedWithError))
				}, ServiceEventsTopic)
				pl.Init()
				WaitDoneOrTimeout(done)
			})
		})
		Context("in WaitPlayersReady state", func() {
			It("transitions to the PlayerFinishedWithError state", func() {
				client := NewFakeBrokenDiscoveryClient(bus, id, true, false)
				pl, _ := NewPlayer(ctx, bus, timeout, timeout, &me, params, errCh, logger)
				client.Run()
				Assert(GameFinishedWithError, pl, done, func(states []string) {
					Expect(states[0]).To(Equal(Init))
					Expect(states[1]).To(Equal(Registering))
					Expect(states[2]).To(Equal(PlayerFinishedWithError))
				})
				pl.Init()
				WaitDoneOrTimeout(done)
			})
		})
	})
})

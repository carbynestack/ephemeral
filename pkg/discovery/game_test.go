//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package discovery

import (
	"context"
	"time"

	. "github.com/carbynestack/ephemeral/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	mb "github.com/vardius/message-bus"
	"go.uber.org/zap"
)

var _ = Describe("Game", func() {
	var (
		done chan struct{}
		bus  mb.MessageBus

		timeout     time.Duration
		gameID      string
		game        *Game
		playerCount int
		pb          Publisher
		errCh       chan error
		logger      = zap.NewNop().Sugar()
		ctx         = context.TODO()
	)
	BeforeEach(func() {
		done = make(chan struct{})
		bus = mb.New(10000)
		timeout = 10 * time.Second
		gameID = "0"
		playerCount = 2
		game, _ = NewGame(ctx, gameID, bus, timeout, logger, playerCount)
		pb = Publisher{
			Bus: bus,
			Fsm: game.fsm,
		}
		errCh = make(chan error)
	})
	Context("when the game finishes with success", func() {
		It("it goes from Init to GameSuccess state", func() {
			game.Init(errCh)
			// Emulate messages from real players.
			Assert(PlayersReady, game, done, func(states []string) {})
			pb.Publish(PlayerReady, gameID)
			pb.Publish(PlayerReady, gameID)
			WaitDoneOrTimeout(done)
			Assert(TCPCheckSuccessAll, game, done, func(states []string) {})
			pb.Publish(TCPCheckSuccess, gameID)
			pb.Publish(TCPCheckSuccess, gameID)
			WaitDoneOrTimeout(done)
			Assert(GameDone, game, done, func(states []string) {
				Expect(states[0]).To(Equal(Init))
				Expect(states[1]).To(Equal(WaitPlayersReady))
				Expect(states[2]).To(Equal(WaitPlayersReady))
				// the states are repeated 3 times because of one initial entrance
				// and 2 times due to the stay in the state.
				Expect(states[3]).To(Equal(WaitTCPCheck))
				Expect(states[4]).To(Equal(WaitTCPCheck))
				Expect(states[5]).To(Equal(WaitTCPCheck))
				Expect(states[6]).To(Equal(Playing))
				Expect(states[7]).To(Equal(Playing))
				Expect(states[8]).To(Equal(Playing))
				Expect(states[9]).To(Equal(GameDone))
			}, ServiceEventsTopic)
			pb.Publish(GameFinishedWithSuccess, gameID)
			pb.Publish(GameFinishedWithSuccess, gameID)
			WaitDoneOrTimeout(done)
		})
	})
	Context("when at least one player fails", func() {
		Context("during the game", func() {
			It("transitions to the GameError state", func() {
				game.Init(errCh)
				Assert(PlayersReady, game, done, func(states []string) {})
				pb.Publish(PlayerReady, gameID)
				pb.Publish(PlayerReady, gameID)
				WaitDoneOrTimeout(done)
				Assert(TCPCheckSuccessAll, game, done, func(states []string) {})
				pb.Publish(TCPCheckSuccess, gameID)
				pb.Publish(TCPCheckSuccess, gameID)
				WaitDoneOrTimeout(done)
				Assert(GameError, game, done, func(states []string) {})
				Assert(GameDone, game, done, func(states []string) {
					Expect(states[0]).To(Equal(Init))
					Expect(states[1]).To(Equal(WaitPlayersReady))
					Expect(states[2]).To(Equal(WaitPlayersReady))
					Expect(states[3]).To(Equal(WaitTCPCheck))
					Expect(states[4]).To(Equal(WaitTCPCheck))
					Expect(states[5]).To(Equal(WaitTCPCheck))
					Expect(states[6]).To(Equal(Playing))
					Expect(states[7]).To(Equal(Playing))
					Expect(states[8]).To(Equal(GameError))
					Expect(states[9]).To(Equal(GameDone))
					Expect(len(states)).To(Equal(10))
				}, ServiceEventsTopic)
				pb.Publish(GameFinishedWithSuccess, gameID)
				pb.Publish(GameFinishedWithError, gameID)
				WaitDoneOrTimeout(done)
				WaitDoneOrTimeout(done)
			})
		})
		Context("at TCP check", func() {
			It("transitions to the GameError state", func() {
				game.Init(errCh)
				Assert(PlayersReady, game, done, func(states []string) {})
				pb.Publish(PlayerReady, gameID)
				pb.Publish(PlayerReady, gameID)
				WaitDoneOrTimeout(done)
				Assert(GameDone, game, done, func(states []string) {
					Expect(states[0]).To(Equal(Init))
					Expect(states[1]).To(Equal(WaitPlayersReady))
					Expect(states[2]).To(Equal(WaitPlayersReady))
					Expect(states[3]).To(Equal(WaitTCPCheck))
					Expect(states[4]).To(Equal(WaitTCPCheck))
					Expect(states[5]).To(Equal(GameError))
				}, ServiceEventsTopic)
				pb.Publish(TCPCheckSuccess, gameID)
				pb.Publish(TCPCheckFailure, gameID)
				WaitDoneOrTimeout(done)
			})
		})
	})
	Context("state timeout occurs", func() {
		It("transitions to the GameError state", func() {
			timeout := 10 * time.Millisecond
			game, _ := NewGame(ctx, gameID, bus, timeout, logger, playerCount)
			// No player publishes an event, simulate a state timeout.
			Assert(GameDone, game, done, func(states []string) {
				Expect(states[0]).To(Equal(Init))
				Expect(states[1]).To(Equal(GameError))
			}, ServiceEventsTopic)
			game.Init(errCh)
			WaitDoneOrTimeout(done)
		})
	})
})

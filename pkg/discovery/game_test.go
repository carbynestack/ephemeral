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
			for i := 0; i < playerCount; i++ {
				pb.Publish(PlayerReady, gameID)
			}
			WaitDoneOrTimeout(done)
			Assert(TCPCheckSuccessAll, game, done, func(states []string) {})
			for i := 0; i < playerCount; i++ {
				pb.Publish(TCPCheckSuccess, gameID)
			}
			WaitDoneOrTimeout(done)
			Assert(GameDone, game, done, func(states []string) {
				statesAsserter := &StatesAsserter{states: states}

				statesAsserter.ExpectNext().To(Equal(Init))
				for i := 0; i < playerCount; i++ {
					statesAsserter.ExpectNext().To(Equal(WaitPlayersReady))
				}
				statesAsserter.ExpectNext().To(Equal(WaitTCPCheck))
				for i := 0; i < playerCount; i++ {
					statesAsserter.ExpectNext().To(Equal(WaitTCPCheck))
				}
				statesAsserter.ExpectNext().To(Equal(Playing))
				for i := 0; i < playerCount; i++ {
					statesAsserter.ExpectNext().To(Equal(Playing))
				}
				statesAsserter.ExpectNext().To(Equal(GameDone))
			}, ServiceEventsTopic)
			for i := 0; i < playerCount; i++ {
				pb.Publish(GameFinishedWithSuccess, gameID)
			}
			WaitDoneOrTimeout(done)
		})
	})
	Context("when at least one player fails", func() {
		Context("during the game", func() {
			It("transitions to the GameError state", func() {
				game.Init(errCh)
				Assert(PlayersReady, game, done, func(states []string) {})
				for i := 0; i < playerCount; i++ {
					pb.Publish(PlayerReady, gameID)
				}
				WaitDoneOrTimeout(done)
				Assert(TCPCheckSuccessAll, game, done, func(states []string) {})
				for i := 0; i < playerCount; i++ {
					pb.Publish(TCPCheckSuccess, gameID)
				}
				WaitDoneOrTimeout(done)
				Assert(GameError, game, done, func(states []string) {})
				Assert(GameDone, game, done, func(states []string) {
					statesAsserter := &StatesAsserter{states: states}

					statesAsserter.ExpectNext().To(Equal(Init))
					for i := 0; i < playerCount; i++ {
						statesAsserter.ExpectNext().To(Equal(WaitPlayersReady))
					}
					statesAsserter.ExpectNext().To(Equal(WaitTCPCheck))
					for i := 0; i < playerCount; i++ {
						statesAsserter.ExpectNext().To(Equal(WaitTCPCheck))
					}
					for i := 0; i < playerCount; i++ {
						statesAsserter.ExpectNext().To(Equal(Playing))
					}
					statesAsserter.ExpectNext().To(Equal(GameError))
					statesAsserter.ExpectNext().To(Equal(GameDone))

					Expect(len(states)).To(Equal(3*playerCount + 4))
				}, ServiceEventsTopic)

				for i := 0; i < playerCount-1; i++ {
					pb.Publish(GameFinishedWithSuccess, gameID)
				}
				pb.Publish(GameFinishedWithError, gameID)
				WaitDoneOrTimeout(done)
				WaitDoneOrTimeout(done)
			})
		})
		Context("at TCP check", func() {
			It("transitions to the GameError state", func() {
				game.Init(errCh)
				Assert(PlayersReady, game, done, func(states []string) {})
				for i := 0; i < playerCount; i++ {
					pb.Publish(PlayerReady, gameID)
				}
				WaitDoneOrTimeout(done)
				Assert(GameDone, game, done, func(states []string) {
					statesAsserter := &StatesAsserter{states: states}
					statesAsserter.ExpectNext().To(Equal(Init))
					for i := 0; i < playerCount; i++ {
						statesAsserter.ExpectNext().To(Equal(WaitPlayersReady))
					}
					for i := 0; i < playerCount; i++ {
						statesAsserter.ExpectNext().To(Equal(WaitTCPCheck))
					}
					statesAsserter.ExpectNext().To(Equal(GameError))
				}, ServiceEventsTopic)
				for i := 0; i < playerCount-1; i++ {
					pb.Publish(TCPCheckSuccess, gameID)
				}
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

type StatesAsserter struct {
	states       []string
	currentIndex int
}

func (s *StatesAsserter) ExpectNext() Assertion {
	state := s.states[s.currentIndex]
	s.currentIndex++
	return Expect(state)
}

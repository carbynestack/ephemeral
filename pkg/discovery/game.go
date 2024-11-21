// Copyright (c) 2021-2024 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package discovery

import (
	"context"
	"errors"
	"github.com/carbynestack/ephemeral/pkg/discovery/fsm"
	"time"

	. "github.com/carbynestack/ephemeral/pkg/types"

	mb "github.com/vardius/message-bus"
	"go.uber.org/zap"
)

// FSMWithBus is a Finate State Machine coupled with message bus.
type FSMWithBus interface {
	History() *fsm.History
	Bus() mb.MessageBus
}

// Game is a single execution of MPC.
type Game struct {
	id  string
	fsm *fsm.FSM
	bus mb.MessageBus
	pb  *Publisher
}

// Init starts the fsm of the Game with its initial state.
//
// `errChan` is expected to be a buffered channel with minimum capacity of "1".
func (g *Game) Init(errCh chan error) {
	// TODO: Think of another option how to assign the fsm to the publisher.
	g.pb.Fsm = g.fsm
	go g.fsm.Run(errCh)
}

// History returns history of Game's FSM
func (g *Game) History() *fsm.History {
	return g.fsm.History()
}

// Bus returns the bus used by game.
func (g *Game) Bus() mb.MessageBus {
	return g.bus
}

// NewGame returns an instance of Game.
func NewGame(ctx context.Context, id string, bus mb.MessageBus, stateTimeout time.Duration, computationTimeout time.Duration, logger *zap.SugaredLogger, playerCount int) (*Game, error) {
	publisher := &Publisher{
		Bus: bus,
	}
	callbacker := GameCallbacker{
		pb:     publisher,
		gameID: id,
		logger: logger.With("gameID", id),
	}
	cb := []*fsm.Callback{
		fsm.AfterEnter(WaitPlayersReady).Do(callbacker.sendRegistered()),
		fsm.AfterEnter(WaitPlayersReady).Do(callbacker.checkSomethingReady(playerCount, PlayerReady, PlayersReady)),
		fsm.AfterEnter(WaitTCPCheck).Do(callbacker.checkSomethingReady(playerCount, TCPCheckSuccess, TCPCheckSuccessAll)),
		fsm.AfterEnter(Playing).Do(callbacker.checkSomethingReady(playerCount, GameFinishedWithSuccess, GameSuccess)),
		fsm.AfterEnter(GameDone).Do(callbacker.gameDone()),
		fsm.AfterEnter(GameError).Do(callbacker.gameError()),
		fsm.WhenStateTimeout().Do(callbacker.stateTimeout()),
	}
	trs := []*fsm.Transition{
		fsm.WhenIn(Init).GotEvent(PlayerReady).GoTo(WaitPlayersReady),
		fsm.WhenIn(WaitPlayersReady).GotEvent(PlayerReady).Stay(),
		fsm.WhenIn(WaitPlayersReady).GotEvent(PlayersReady).GoTo(WaitTCPCheck),
		fsm.WhenIn(WaitTCPCheck).GotEvent(TCPCheckSuccess).Stay(),
		fsm.WhenIn(WaitTCPCheck).GotEvent(TCPCheckSuccessAll).GoTo(Playing).WithTimeout(computationTimeout),
		fsm.WhenIn(WaitTCPCheck).GotEvent(TCPCheckFailure).GoTo(GameError),
		fsm.WhenIn(WaitTCPCheck).GotEvent(GameFinishedWithError).GoTo(GameError),
		fsm.WhenIn(Playing).GotEvent(GameFinishedWithSuccess).Stay(),
		fsm.WhenIn(Playing).GotEvent(GameFinishedWithError).GoTo(GameError),
		fsm.WhenIn(Playing).GotEvent(GameSuccess).GoTo(GameDone),
		fsm.WhenIn(Playing).GotEvent(GameError).GoTo(GameError),
		fsm.WhenInAnyState().GotEvent(StateTimeoutError).GoTo(GameError),
		fsm.WhenInAnyState().GotEvent(GameDone).GoTo(GameDone),
	}
	callbacks, transitions := fsm.InitCallbacksAndTransitions(cb, trs)
	f, err := fsm.NewFSM(ctx, Init, transitions, callbacks, stateTimeout, logger)
	if err != nil {
		return nil, err
	}
	err = bus.Subscribe(id, func(e interface{}) {
		// All events received on player's name topic are forwarded to the state machine.
		ev := e.(*fsm.Event)
		// Rewrite the FSM link to treat them as internal events.
		ev.Meta.FSM = f
		f.Write(ev)
	})
	return &Game{
		id:  id,
		fsm: f,
		bus: bus,
		pb:  publisher,
	}, nil
}

// GameCallbacker contains methods to react on game events.
type GameCallbacker struct {
	pb     *Publisher
	gameID string
	logger *zap.SugaredLogger
}

// sendRegistered notifies the client that it was registered for the game.
func (c *GameCallbacker) sendRegistered() func(e interface{}) error {
	return func(e interface{}) error {
		meta := e.(*fsm.Event).Meta
		c.logger.Debugw("Client registered", "meta", meta)
		c.pb.Publish(Registered, ServiceEventsTopic, meta.TargetTopic)
		return nil
	}
}

// gameDone publishes GameDone event to discovery topic.
func (c *GameCallbacker) gameDone() func(e interface{}) error {
	return func(e interface{}) error {
		meta := e.(*fsm.Event).Meta
		c.logger.Debugw("Game done", "meta", meta)
		c.pb.Publish(GameDone, ServiceEventsTopic, meta.TargetTopic)
		meta.FSM.Stop()
		return nil
	}
}

// gameError sends out GameError and GameDone events to the discovery topic.
func (c *GameCallbacker) gameError() func(e interface{}) error {
	return func(e interface{}) error {
		meta := e.(*fsm.Event).Meta
		var history *fsm.History
		if meta.FSM != nil {
			history = meta.FSM.History()
		}
		c.logger.Debugw("Game failed", "meta", meta, "event history", history)
		c.pb.Publish(GameError, DiscoveryTopic, meta.TargetTopic)
		c.pb.Publish(GameDone, c.gameID)
		return nil
	}
}

// stateTimeout sends out a StateTimeoutError.
func (c *GameCallbacker) stateTimeout() func(e interface{}) error {
	return func(e interface{}) error {
		c.logger.Debug("Send state timeout")
		c.pb.Publish(StateTimeoutError, c.gameID)
		return nil
	}
}

// checkSomethingReady verifies the state "in" was sent by all players.
// And if it is the case, it sends out the state "out" to discovery and to itself.
func (c *GameCallbacker) checkSomethingReady(players int, in string, out string) func(e interface{}) error {
	return func(e interface{}) error {
		c.logger.Debugw("Check readiness", "Players", players, "Event", in)
		meta := e.(*fsm.Event).Meta
		f := meta.FSM
		if f == nil {
			// TODO: make sure this error handling is tested.
			return errors.New("fsm is nil")
		}
		events := f.History().GetEvents()
		readyPlayers := countEvents(events, in)
		if readyPlayers == players {
			c.logger.Debugf("Players ready - sending message %v", out)
			// the targetTopic of previous event includes the game id we would need for further event forwarding.
			c.pb.Publish(out, DiscoveryTopic, meta.TargetTopic)
			c.pb.Publish(out, c.gameID)
		}
		return nil
	}
}

// countEvents returns the number of events in the history matching by name.
func countEvents(events []*fsm.Event, event string) int {
	i := 0
	for _, j := range events {
		if j.Name == event {
			i++
		}
	}
	return i
}

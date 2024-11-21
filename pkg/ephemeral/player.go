// Copyright (c) 2021-2024 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package ephemeral

import (
	"context"
	"errors"
	"fmt"
	d "github.com/carbynestack/ephemeral/pkg/discovery"
	"github.com/carbynestack/ephemeral/pkg/discovery/fsm"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"github.com/carbynestack/ephemeral/pkg/ephemeral/network"
	. "github.com/carbynestack/ephemeral/pkg/types"
	"strings"
	"time"

	mb "github.com/vardius/message-bus"
	"go.uber.org/zap"
)

const (
	rawEventsTopic = "rawEvents"
)

// PlayerParams defines parameters of the player.
type PlayerParams struct {
	GameID            string
	Pod               string
	PlayerID, Players int32
	// Address of the frontend gateway (e.g. Istio).
	IP   string
	Name string
}

// NewPlayer returns an fsm based model of the MPC player.
func NewPlayer(ctx context.Context, bus mb.MessageBus, stateTimeout time.Duration, computationTimeout time.Duration, me MPCEngine, playerParams *PlayerParams, errCh chan error, logger *zap.SugaredLogger) (*Player1, error) {
	call := NewCallbacker(bus, playerParams, errCh, logger)
	cbs := []*fsm.Callback{
		fsm.AfterEnter(Registering).Do(call.sendPlayerReady()),
		fsm.AfterEnter(Playing).Do(call.playing(playerParams.Name, me)),
		fsm.AfterEnter(PlayerFinishedWithError).Do(call.finishWithError(playerParams.Name)),
		fsm.AfterEnter(PlayerFinishedWithSuccess).Do(call.finishWithSuccess(playerParams.Name)),
		fsm.AfterEnter(PlayerDone).Do(call.done()),
		fsm.WhenStateTimeout().Do(call.finishWithError(playerParams.Name)),
	}
	trs := []*fsm.Transition{
		fsm.WhenIn(Init).GotEvent(Register).GoTo(Registering),
		fsm.WhenIn(Registering).GotEvent(PlayersReady).GoTo(Playing).WithTimeout(computationTimeout),
		fsm.WhenIn(Playing).GotEvent(PlayerFinishedWithSuccess).GoTo(PlayerFinishedWithSuccess),
		fsm.WhenInAnyState().GotEvent(GameError).GoTo(PlayerFinishedWithError),
		fsm.WhenInAnyState().GotEvent(PlayingError).GoTo(PlayerFinishedWithError),
		fsm.WhenInAnyState().GotEvent(PlayerDone).GoTo(PlayerDone),
		fsm.WhenInAnyState().GotEvent(StateTimeoutError).GoTo(PlayerFinishedWithError),
	}
	callbacks, transitions := fsm.InitCallbacksAndTransitions(cbs, trs)
	f, err := fsm.NewFSM(ctx, "Init", transitions, callbacks, stateTimeout, logger)
	// We can only update publisher's FSM after fsm is created.
	call.pb.Fsm = f
	if err != nil {
		return nil, err
	}

	err = bus.Subscribe(rawEventsTopic, func(e interface{}) {
		// Convert the events from the wire to the format understandable by the FSM.
		ev := e.(*pb.Event)
		call.pb.PublishWithBody(ev.Name, playerParams.Name, ev)
	})
	err = bus.Subscribe(playerParams.Name, func(e interface{}) {
		// All events received on player's name topic are forwarded to the state machine.
		ev := e.(*fsm.Event)
		f.Write(ev)
	})
	return &Player1{
		name:       playerParams.Name,
		fsm:        f,
		messageBus: bus,
		me:         me,
		params:     playerParams,
		call:       call,
		errCh:      errCh,
		logger:     logger,
		ctx:        ctx,
	}, nil
}

// AbstractPlayer is an interface of a player.
type AbstractPlayer interface {
	Init()
	Stop()
	History() *fsm.History
	Bus() mb.MessageBus
	PublishEvent(name, topic string, event *pb.Event)
}

// Player1 stores the FSM of the player, it manages the state and runs callbacks as response on internal/external events.
// The events are communicated via in-memory message bus with 2 topics:
// "discovery" topic is used to transmit events from player to discovery service.
// "playerN" topic name corresponds to the name of the player.
// And is used for events from discovery to the player and from the player to itself.
type Player1 struct {
	name       string
	fsm        *fsm.FSM
	messageBus mb.MessageBus
	logger     *zap.SugaredLogger
	nc         network.NetworkChecker
	me         MPCEngine
	params     *PlayerParams
	call       *Callbacker
	errCh      chan error
	ctx        context.Context
}

// Init starts FSM and triggers the registration of the player.
func (p *Player1) Init() {
	go p.fsm.Run(p.errCh)
	p.call.sendEvent(Register, p.name, struct{}{})
}

// Stop unsubscribes bus listeners.
func (p *Player1) Stop() {
	p.messageBus.Close(rawEventsTopic)
	p.messageBus.Close(p.params.Name)
}

// History returns player's state and event history.
func (p *Player1) History() *fsm.History {
	return p.fsm.History()
}

// Bus returns in internal message bus to comply with FSMWithBus interface.
func (p *Player1) Bus() mb.MessageBus {
	return p.messageBus
}

// PublishEvent publishes an external event into player's state machine.
func (p *Player1) PublishEvent(name, topic string, event *pb.Event) {
	p.call.pb.PublishWithBody(name, topic, event)
}

// NewCallbacker returns a new instance of callbacker
func NewCallbacker(bus mb.MessageBus, playerParams *PlayerParams, errCh chan error, logger *zap.SugaredLogger) *Callbacker {
	return &Callbacker{
		pb:           d.NewPublisher(bus),
		playerParams: playerParams,
		errCh:        errCh,
		logger:       logger,
	}
}

// Callbacker preserves callback functions used in Player FSM.
type Callbacker struct {
	pb           *d.Publisher
	playerParams *PlayerParams
	errCh        chan error
	logger       *zap.SugaredLogger
}

// registration forwards registration request to the discovery service.
func (c *Callbacker) registration() func(e interface{}) error {
	return func(e interface{}) error {
		c.sendEvent(Register, DiscoveryTopic, e)
		return nil
	}
}

// sendPlayerReady notifies discovery service about its readiness.
func (c *Callbacker) sendPlayerReady() func(e interface{}) error {
	return func(e interface{}) error {
		c.sendEvent(PlayerReady, DiscoveryTopic, e)
		return nil
	}
}

// playing triggers the MPC computation and signals itself the state of the execution.
func (c *Callbacker) playing(id string, me MPCEngine) func(e interface{}) error {
	return func(e interface{}) error {
		ev := e.(*fsm.Event)
		err := me.Execute(ev.Meta.TransportMsg)
		if err != nil {
			c.logger.Errorf("Error during code execution: %v", err)
			c.sendEvent(PlayingError, id, e)
			return nil
		}
		c.sendEvent(PlayerFinishedWithSuccess, id, e)
		return nil
	}
}

// finishWithError notifies discovery service about an error.
func (c *Callbacker) finishWithError(id string) func(e interface{}) error {
	return func(e interface{}) error {
		c.sendEvent(GameFinishedWithError, DiscoveryTopic, e)
		c.sendEvent(PlayerDone, id, e)
		event := e.(*fsm.Event)
		msg := fmt.Sprintf("game failed with error: %s", event.Name)
		if event.Meta != nil && event.Meta.FSM != nil && event.Meta.FSM.History() != nil {
			eventDetails := make([]string, len(event.Meta.FSM.History().GetEvents()))
			for _, s := range event.Meta.FSM.History().GetStates() {
				eventDetails = append(eventDetails, s)
			}
			msg = fmt.Sprintf("%s\n\tHistory: %s", msg, strings.Join(eventDetails, " -> "))
		}
		err := errors.New(msg)
		c.logger.Debugf("Player finished with error: %v", err)
		select {
		case c.errCh <- err:
		default:
		}
		return nil
	}
}

// finishWithSuccess notifies discovery service about successful execution.
func (c *Callbacker) finishWithSuccess(id string) func(e interface{}) error {
	return func(e interface{}) error {
		c.sendEvent(GameFinishedWithSuccess, DiscoveryTopic, e)
		c.sendEvent(PlayerDone, id, e)
		return nil
	}
}

func (c *Callbacker) done() func(e interface{}) error {
	return func(e interface{}) error {
		c.pb.Fsm.Stop()
		c.sendEvent(PlayerDone, ServiceEventsTopic, e)
		return nil
	}
}

// sendEvent sends out an event to discovery service through the message bus.
func (c *Callbacker) sendEvent(name, topic string, e interface{}) {
	event := &pb.Event{
		GameID: c.playerParams.GameID,
		Name:   name,
		Players: []*pb.Player{
			&pb.Player{
				Id:      c.playerParams.PlayerID,
				Players: c.playerParams.Players,
				Pod:     c.playerParams.Pod,
				Ip:      c.playerParams.IP,
			},
		},
	}
	c.pb.PublishWithBody(name, topic, event, c.playerParams.GameID)
	c.logger.Debugw("Sending event", "event", event, "topic", topic)
}

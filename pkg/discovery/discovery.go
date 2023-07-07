// Copyright (c) 2021-2023 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package discovery

import (
	"context"
	"errors"
	"github.com/carbynestack/ephemeral/pkg/discovery/fsm"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	t "github.com/carbynestack/ephemeral/pkg/discovery/transport/server"
	. "github.com/carbynestack/ephemeral/pkg/types"
	"sync"
	"time"

	mb "github.com/vardius/message-bus"
	"go.uber.org/zap"
)

const (
	mpcPodNameLabel = "mpc.podName"
)

var (
	// BasePort is the base for the port number that is used by the proxy.
	BasePort        = int32(5000)
	baseNetworkName = "player-network"
	ctx             = context.TODO()
)

// Event is a generic message sent between clients and discovery service.
type Event struct {
	Name   string
	GameID string
}

// PlayerID is the id of the MPC player.
type PlayerID int32

// NewServiceNG returns a new instance of discovery service.
func NewServiceNG(bus mb.MessageBus, pub *Publisher, stateTimeout time.Duration, computationTimeout time.Duration, tr t.Transport, n Networker, frontendAddress string, logger *zap.SugaredLogger, mode string, client DiscoveryClient, playerCount int) *ServiceNG {
	games := map[string]*Game{}
	players := map[string]map[PlayerID]*pb.Player{}
	pods := map[string]int32{}
	networks := map[string]int32{}
	errCh := make(chan error)
	return &ServiceNG{
		bus:                 bus,
		games:               games,
		errCh:               errCh,
		pb:                  pub,
		stateTimeout:        stateTimeout,
		computationTimeout:  computationTimeout,
		transport:           tr,
		players:             players,
		playerCount:         playerCount,
		pods:                pods,
		networks:            networks,
		networker:           n,
		homeFrontendAddress: frontendAddress,
		logger:              logger,
		mode:                mode,
		client:              client,
		startCh:             make(chan struct{}),
	}
}

// ServiceNG is a new generation of discovery service.
type ServiceNG struct {
	bus                 mb.MessageBus
	pb                  *Publisher
	games               map[string]*Game
	players             map[string]map[PlayerID]*pb.Player
	playerCount         int
	pods                map[string]int32
	networks            map[string]int32
	mux                 sync.Mutex
	errCh               chan error
	stateTimeout        time.Duration
	computationTimeout  time.Duration
	transport           t.Transport
	networker           Networker
	homeFrontendAddress string
	logger              *zap.SugaredLogger
	mode                string
	client              DiscoveryClient
	startCh             chan struct{}
}

// Stop stops the service.
func (s *ServiceNG) Stop() {
	s.transport.Stop()
}

// Start starts listening to incoming messages from clients.
func (s *ServiceNG) Start() error {
	s.bus.Subscribe(ServiceEventsTopic, func(e interface{}) error {
		ev := e.(*fsm.Event)
		if ev.Name == DiscoveryServiceStarted {
			s.startCh <- struct{}{}
		}
		return nil
	})
	if s.mode == ModeSlave {
		s.bus.Subscribe(ClientIncomingEventsTopic, s.processInSlave)
		conn, err := s.client.Connect()
		if err != nil {
			return err
		}
		dc := pb.NewDiscoveryClient(conn)
		go s.client.Run(dc)
		s.writeToMaster()
		go s.readFromMaster()
	} else {
		s.bus.Subscribe(ClientIncomingEventsTopic, s.processIn)
	}
	s.bus.Subscribe(DiscoveryTopic, s.processOut)
	go s.transport.Run(func() {
		s.pb.Publish(DiscoveryServiceStarted, ServiceEventsTopic)
	})
	go s.readFromWire()
	s.writeToWire()
	err := <-s.errCh
	return err
}

// WaitUntilReady waits until the service has started until the defined timeout is reached.
func (s *ServiceNG) WaitUntilReady(timeout time.Duration) error {
	select {
	case <-s.startCh:
		return nil
	case <-time.After(timeout):
		return errors.New("timeout while waiting for discovery service to come online")
	}
}

// DeleteCallback is called when the pod is deleted, so we remove it from our bookkeeping.
func (s *ServiceNG) DeleteCallback(name string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	delete(s.networks, name)
	delete(s.pods, name)
}

// readFromWire sends the messages from the discovery clients to the internal message bus.
func (s *ServiceNG) readFromWire() {
	inCh := s.transport.GetIn()
	for {
		event := <-inCh
		s.logger.Debugf("Discovery service received event: %s\n", event.Name)
		s.bus.Publish(ClientIncomingEventsTopic, event)
	}
}

// writeToWire subscribes to the messages from the message bus and sends them back to the discovery clients.
func (s *ServiceNG) writeToWire() {
	outCh := s.transport.GetOut()
	s.bus.Subscribe(ClientOutgoingEventsTopic, func(e interface{}) {
		ev := e.(*pb.Event)
		s.logger.Debugw("Forwarding message from wire to clients", "Event", ev)
		// TODO: do not broadcast to all current games.
		outCh <- ev
	})
}

func (s *ServiceNG) writeToMaster() {
	outCh := s.client.GetOut()
	s.bus.Subscribe(MasterOutgoingEventsTopic, func(e interface{}) {
		ev := e.(*pb.Event)
		s.logger.Debugf("Sending event %s to master", ev.Name)
		outCh <- ev
		s.logger.Debugf("Sent event %s to master", ev.Name)
	})
}

func (s *ServiceNG) readFromMaster() {
	inCh := s.client.GetIn()
	for {
		event := <-inCh
		s.logger.Debugf("Event from Master: %s\n", event.Name)
		s.bus.Publish(ClientOutgoingEventsTopic, event)
	}
}

// registerPlayer creates player's network and registers it in the internal bookkeeping of the discovery service.
func (s *ServiceNG) registerPlayer(pl *pb.Player, gameID string) error {
	defer func() {
		// Set the port of the player every time this message is called.
		pl.Port = s.networks[pl.Pod]
	}()
	s.logger.Debug("Register PLayer", "player", pl, "gameId", gameID)
	p, ok := s.players[gameID]
	// Create a new map for the GameID
	if !ok {
		s.logger.Debug("Create new Player map")
		players := map[PlayerID]*pb.Player{}
		s.players[gameID] = players
	}
	p, _ = s.players[gameID]

	// Do not register the player twice.
	if _, ok := p[PlayerID(pl.Id)]; ok {
		s.logger.Debug("Player already registered")
		return nil
	}

	// Create a new network if it doesn't exist yet.
	_, ok = s.networks[pl.Pod]
	if !ok {
		s.logger.Debug("Create new network")
		port, err := s.createNetwork(pl)
		if err != nil {
			s.logger.Errorf("Error creating network %v", err)
			return err
		}
		s.networks[pl.Pod] = port
	}
	s.pods[pl.Pod] = pl.Id
	p[PlayerID(pl.Id)] = pl
	return nil
}

// createNetwork creates the network if its not a foreign event and update the port of the player.
func (s *ServiceNG) createNetwork(pl *pb.Player) (int32, error) {
	if pl.Ip == s.homeFrontendAddress {
		port, err := s.networker.CreateNetwork(pl)
		if err != nil {
			return 0, err
		}
		return port, err
	}
	s.logger.Debug("Do not create the network for the foreign player")
	return pl.Port, nil
}

// processInSlave registers the player and forwards client events to the master discovery.
func (s *ServiceNG) processInSlave(e interface{}) {
	s.mux.Lock()
	defer s.mux.Unlock()
	ev := e.(*pb.Event)
	player := ev.Players[0]
	s.registerPlayer(player, ev.GameID)
	s.bus.Publish(MasterOutgoingEventsTopic, ev)
}

// processIn takes care of incoming events from the discovery clients.
// It starts the games and converts the events to the required format.
func (s *ServiceNG) processIn(e interface{}) {
	s.mux.Lock()
	defer s.mux.Unlock()
	ev := e.(*pb.Event)
	player := ev.Players[0]
	name := ev.Name
	s.registerPlayer(player, ev.GameID)
	g, ok := s.games[ev.GameID]
	if !ok { // If game does not exist, create it
		g, err := NewGame(ctx, ev.GameID, s.bus, s.stateTimeout, s.computationTimeout, s.logger, s.playerCount)
		if err != nil {
			s.errCh <- err
		}
		gameErrCh := make(chan error, 1)
		go func() {
			// Do not propagate this error to the client.
			// Since should not be related to the client code, but would indicate a bug in the Game FSM.
			if err, open := <-gameErrCh; open {
				s.logger.Errorf("Game error: %s\n", err.Error())
			}
		}()
		g.Init(gameErrCh)
		g.pb.Publish(name, ev.GameID)
		s.games[ev.GameID] = g
	} else if s.verifyGameState(g) {
		g.pb.Publish(name, ev.GameID)
	} else {
		g.pb.Publish(GameProtocolError, DiscoveryTopic, ev.GameID)
	}
}

// processOut converts the internal events to the format understandable by the
// discovery clients.
func (s *ServiceNG) processOut(e interface{}) {
	s.mux.Lock()
	defer s.mux.Unlock()
	ev := e.(*fsm.Event)
	gameID := ev.Meta.SrcTopics[0]
	players, ok := s.players[gameID]
	pls := []*pb.Player{}
	for _, p := range players {
		pls = append(pls, p)
	}
	if !ok {
		s.logger.Errorf("No player registered for the game with id %s", gameID)
	}
	event := &pb.Event{
		Name:    ev.Name,
		GameID:  gameID,
		Players: pls,
	}
	s.pb.PublishExternalEvent(event, ClientOutgoingEventsTopic)
}

// verifyGameState checks whether it is still allowed to join the game.
func (s *ServiceNG) verifyGameState(g *Game) bool {
	if g.fsm.Current() != fsm.Stopped {
		return true
	}
	return false
}

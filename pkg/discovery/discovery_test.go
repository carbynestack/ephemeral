//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package discovery

import (
	"github.com/carbynestack/ephemeral/pkg/discovery/fsm"
	proto "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	t "github.com/carbynestack/ephemeral/pkg/discovery/transport/server"

	"time"

	. "github.com/carbynestack/ephemeral/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	mb "github.com/vardius/message-bus"
	"go.uber.org/zap"
)

var _ = Describe("DiscoveryNG", func() {

	var (
		bus             mb.MessageBus
		timeout         = 1 * time.Second
		done            chan struct{}
		pb              *Publisher
		s               *ServiceNG
		g               *GamesWithBus
		stateTimeout    time.Duration
		tr              t.Transport
		n               *FakeNetworker
		frontendAddress string
		logger          = zap.NewNop().Sugar()
	)

	BeforeEach(func() {
		bus = mb.New(10000)
		done = make(chan struct{})
		pb = &Publisher{
			Bus: bus,
			Fsm: &fsm.FSM{},
		}
		stateTimeout = 10 * time.Second
		tr = &FakeTransport{}
		n = &FakeNetworker{
			FreePorts: []int32{30000, 30001, 30002},
		}
		frontendAddress = "192.168.0.1"
		conf := &FakeDClient{}
		playerCount := 2
		s = NewServiceNG(bus, pb, stateTimeout, tr, n, frontendAddress, logger, ModeMaster, conf, playerCount)
		g = &GamesWithBus{
			Games: s.games,
			Bus:   bus,
		}
	})

	Context("when the game has not started yet", func() {
		var (
			ev *proto.Event
		)
		BeforeEach(func() {
			ev = GenerateEvents(PlayerReady, "0")[0]
		})
		Context("all players are registered", func() {
			It("receives PlayersReady event", func() {
				ready := GenerateEvents(PlayersReady, "0")[0]
				assertExternalEvent(ready, ClientOutgoingEventsTopic, g, done, func(states []string) {})
				go s.Start()
				s.WaitUntilReady(timeout)
				pb.PublishExternalEvent(ev, ClientIncomingEventsTopic)
				pb.PublishExternalEvent(ev, ClientIncomingEventsTopic)
				WaitDoneOrTimeout(done)
			})
			It("receives addresses of the players that joined the game", func() {
				playersReady := GenerateEvents(PlayersReady, "0")[0]
				playerOneIsReady := GenerateEvents(PlayerReady, "0")[0]
				playerTwoIsReady := GenerateEvents(PlayerReady, "0")[0]

				player1 := proto.Player{
					Ip:  frontendAddress,
					Id:  0,
					Pod: "pod1",
				}
				player2 := proto.Player{
					Ip:  frontendAddress,
					Id:  1,
					Pod: "pod2",
				}

				playerOneIsReady.Players[0] = &player1
				playerTwoIsReady.Players[0] = &player2

				assertExternalEventBody(playersReady, ClientOutgoingEventsTopic, g, done, func(event *proto.Event) {
					Expect(len(event.Players)).To(Equal(2))
					Expect(event.Players[0].Ip).To(Equal(frontendAddress))
					Expect(event.Players[0].Port).NotTo(BeZero())
					Expect(event.Players[1].Ip).To(Equal(frontendAddress))
					Expect(event.Players[1].Port).NotTo(BeZero())
					Expect(len(s.pods)).To(Equal(2))
					Expect(s.pods["pod1"]).To(Equal(int32(0)))
					Expect(s.pods["pod2"]).To(Equal(int32(1)))
				})
				go s.Start()
				s.WaitUntilReady(timeout)
				pb.PublishExternalEvent(playerOneIsReady, ClientIncomingEventsTopic)
				pb.PublishExternalEvent(playerTwoIsReady, ClientIncomingEventsTopic)
				WaitDoneOrTimeout(done)
			})

			It("doesn't create the player twice", func() {
				playersReady := GenerateEvents(PlayersReady, "0")[0]
				playerOneIsReady := GenerateEvents(PlayerReady, "0")[0]
				playerTwoIsReady := GenerateEvents(PlayerReady, "0")[0]
				playerOneTCPCheckSuccess := GenerateEvents(TCPCheckSuccess, "0")[0]

				player1 := proto.Player{
					Ip: frontendAddress,
					Id: 0,
				}
				player2 := proto.Player{
					Ip: frontendAddress,
					Id: 1,
				}

				playerOneIsReady.Players[0] = &player1
				playerTwoIsReady.Players[0] = &player2
				playerOneTCPCheckSuccess.Players[0] = &player1

				assertExternalEventBody(playersReady, ClientOutgoingEventsTopic, g, done, func(event *proto.Event) {
					Expect(len(event.Players)).To(Equal(2))
				})
				go s.Start()
				s.WaitUntilReady(timeout)
				pb.PublishExternalEvent(playerOneIsReady, ClientIncomingEventsTopic)
				pb.PublishExternalEvent(playerTwoIsReady, ClientIncomingEventsTopic)
				pb.PublishExternalEvent(playerOneTCPCheckSuccess, ClientIncomingEventsTopic)
				WaitDoneOrTimeout(done)
			})
		})
		Context("a single player sends 2 messages in a row", func() {
			It("doens't create the second network", func() {
				playersReady := GenerateEvents(PlayersReady, "0")[0]
				playerOneIsReady := GenerateEvents(PlayerReady, "0")[0]

				player1 := proto.Player{
					Ip: frontendAddress,
					Id: 0,
				}
				playerOneIsReady.Players[0] = &player1
				assertExternalEventBody(playersReady, ClientOutgoingEventsTopic, g, done, func(event *proto.Event) {
					Expect(len(event.Players)).To(Equal(1))
				})
				go s.Start()
				s.WaitUntilReady(timeout)
				pb.PublishExternalEvent(playerOneIsReady, ClientIncomingEventsTopic)
				pb.PublishExternalEvent(playerOneIsReady, ClientIncomingEventsTopic)
				WaitDoneOrTimeout(done)
			})
		})
		Context("an event from a foreign cluster is received", func() {
			It("doesn't create a network for it", func() {
				playersReady := GenerateEvents(PlayersReady, "0")[0]
				playerOneIsReady := GenerateEvents(PlayerReady, "0")[0]
				playerTwoIsReady := GenerateEvents(PlayerReady, "0")[0]
				foreignFrontendAddress := "192.168.0.2"
				player1 := proto.Player{
					Ip: frontendAddress,
					Id: 0,
				}
				player2 := proto.Player{
					Ip: foreignFrontendAddress,
					Id: 1,
				}
				playerOneIsReady.Players[0] = &player1
				playerTwoIsReady.Players[0] = &player2

				assertExternalEventBody(playersReady, ClientOutgoingEventsTopic, g, done, func(event *proto.Event) {
					// Only a single port was used, 2 are still free.
					Expect(len(n.FreePorts)).To(Equal(2))
				})
				go s.Start()
				s.WaitUntilReady(timeout)
				pb.PublishExternalEvent(playerOneIsReady, ClientIncomingEventsTopic)
				pb.PublishExternalEvent(playerTwoIsReady, ClientIncomingEventsTopic)
				WaitDoneOrTimeout(done)
			})
		})
		Context("an event from the same pod but with different game id comes", func() {
			It("it uses the existing network configuration", func() {
				playersReady := GenerateEvents(PlayersReady, "0")[0]
				playerOneIsReady := GenerateEvents(PlayerReady, "0")[0]
				playerTwoIsReady := GenerateEvents(PlayerReady, "0")[0]
				playersReady1 := GenerateEvents(PlayersReady, "1")[0]
				playerOneIsReady1 := GenerateEvents(PlayerReady, "1")[0]
				playerTwoIsReady1 := GenerateEvents(PlayerReady, "1")[0]
				player1 := proto.Player{
					Ip:  frontendAddress,
					Id:  0,
					Pod: "a",
				}
				player2 := proto.Player{
					Ip:  frontendAddress,
					Id:  1,
					Pod: "b",
				}
				playerOneIsReady.Players[0] = &player1
				playerTwoIsReady.Players[0] = &player2
				playerOneIsReady1.Players[0] = &player1
				playerTwoIsReady1.Players[0] = &player2

				assertExternalEventBody(playersReady, ClientOutgoingEventsTopic, g, done, func(event *proto.Event) {
					pp, _ := s.players["0"]
					p1 := pp[PlayerID(int32(0))]
					p2 := pp[PlayerID(int32(1))]
					Expect(p1.Port).To(Equal(int32(30000)))
					Expect(p2.Port).To(Equal(int32(30001)))
				})
				assertExternalEventBody(playersReady1, ClientOutgoingEventsTopic, g, done, func(event *proto.Event) {
					pp, _ := s.players["1"]
					p1 := pp[PlayerID(int32(0))]
					p2 := pp[PlayerID(int32(1))]
					Expect(p1.Port).To(Equal(int32(30000)))
					Expect(p2.Port).To(Equal(int32(30001)))
				})
				go s.Start()
				s.WaitUntilReady(timeout)
				pb.PublishExternalEvent(playerOneIsReady, ClientIncomingEventsTopic)
				pb.PublishExternalEvent(playerTwoIsReady, ClientIncomingEventsTopic)
				time.Sleep(100 * time.Millisecond)
				pb.PublishExternalEvent(playerOneIsReady1, ClientIncomingEventsTopic)
				pb.PublishExternalEvent(playerTwoIsReady1, ClientIncomingEventsTopic)
				WaitDoneOrTimeout(done)
				WaitDoneOrTimeout(done)
			})
		})
		Context("player from another game joins", func() {
			It("doesn't change the state of the existing game", func() {
				ready := GenerateEvents(PlayerReady, "0", "1")
				gameError := GenerateEvents(GameError, "0", "1")
				assertExternalEvent(gameError[0], ClientOutgoingEventsTopic, g, done, func(states []string) {})
				assertExternalEvent(gameError[1], ClientOutgoingEventsTopic, g, done, func(states []string) {})
				// Make state timeout smaller to cause the error.
				s.timeout = 100 * time.Millisecond
				go s.Start()
				s.WaitUntilReady(timeout)
				pb.PublishExternalEvent(ready[0], ClientIncomingEventsTopic)
				pb.PublishExternalEvent(ready[1], ClientIncomingEventsTopic)
				WaitDoneOrTimeout(done, 2*time.Second)
				WaitDoneOrTimeout(done, 2*time.Second)
			})
		})
		Context("games run in parallel", func() {
			It("all of them register their players and move to the next state", func() {
				ev := GenerateEvents(PlayersReady, "0", "1")
				assertExternalEvent(ev[0], ClientOutgoingEventsTopic, g, done, func(states []string) {
					Expect(states[0]).To(Equal(Init))
					Expect(states[1]).To(Equal(WaitPlayersReady))
					Expect(states[2]).To(Equal(WaitPlayersReady))
				})
				assertExternalEvent(ev[1], ClientOutgoingEventsTopic, g, done, func(states []string) {
					Expect(states[0]).To(Equal(Init))
					Expect(states[1]).To(Equal(WaitPlayersReady))
					Expect(states[2]).To(Equal(WaitPlayersReady))
				})
				go s.Start()
				s.WaitUntilReady(timeout)
				events := GenerateEvents(PlayerReady, "0", "0", "1", "1")
				for _, e := range events {
					pb.PublishExternalEvent(e, ClientIncomingEventsTopic)
				}
				WaitDoneOrTimeout(done)
				WaitDoneOrTimeout(done)
			})
		})
	})
	Context("when the game finishes with success", func() {

		var (
			// Events sent by the clients to discovery service.
			ready, tcpCheckSuccess *proto.Event

			// Events sent by discovery service to the clients
			playersReady, tcpCheckSuccessAll, gameFinishedWithSuccess, gameProtocolError *proto.Event
		)

		BeforeEach(func() {
			// Events sent by clients.
			ready = GenerateEvents(PlayerReady, "0")[0]
			tcpCheckSuccess = GenerateEvents(TCPCheckSuccess, "0")[0]
			gameFinishedWithSuccess = GenerateEvents(GameFinishedWithSuccess, "0")[0]

			playersReady = GenerateEvents(PlayersReady, "0")[0]
			tcpCheckSuccessAll = GenerateEvents(TCPCheckSuccessAll, "0")[0]
			gameProtocolError = GenerateEvents(GameProtocolError, "0")[0]
		})

		It("sends all required events to the clients", func() {
			// Do not test the exact states after each events - it was already covered in the Game unit tests.
			assertExternalEvent(playersReady, ClientOutgoingEventsTopic, g, done, func(states []string) {})
			assertExternalEvent(tcpCheckSuccessAll, ClientOutgoingEventsTopic, g, done, func(states []string) {})

			go s.Start()
			s.WaitUntilReady(timeout)

			pb.PublishExternalEvent(ready, ClientIncomingEventsTopic)
			pb.PublishExternalEvent(ready, ClientIncomingEventsTopic)
			time.Sleep(50 * time.Millisecond)
			pb.PublishExternalEvent(tcpCheckSuccess, ClientIncomingEventsTopic)
			pb.PublishExternalEvent(tcpCheckSuccess, ClientIncomingEventsTopic)
			time.Sleep(50 * time.Millisecond)
			pb.PublishExternalEvent(gameFinishedWithSuccess, ClientIncomingEventsTopic)
			pb.PublishExternalEvent(gameFinishedWithSuccess, ClientIncomingEventsTopic)

			WaitDoneOrTimeout(done)
			WaitDoneOrTimeout(done)
		})

		It("doesn't allow the game with the same id happen twice", func() {
			// Play the whole game until the end.
			pb.Bus.Subscribe(ServiceEventsTopic, func(e interface{}) {
				ev := e.(*fsm.Event)
				if ev.Name == GameDone {
					defer func() {
						done <- struct{}{}
					}()
				}
			})

			go s.Start()
			s.WaitUntilReady(timeout)

			pb.PublishExternalEvent(ready, ClientIncomingEventsTopic)
			pb.PublishExternalEvent(ready, ClientIncomingEventsTopic)
			time.Sleep(50 * time.Millisecond)
			pb.PublishExternalEvent(tcpCheckSuccess, ClientIncomingEventsTopic)
			pb.PublishExternalEvent(tcpCheckSuccess, ClientIncomingEventsTopic)
			time.Sleep(50 * time.Millisecond)
			pb.PublishExternalEvent(gameFinishedWithSuccess, ClientIncomingEventsTopic)
			pb.PublishExternalEvent(gameFinishedWithSuccess, ClientIncomingEventsTopic)

			WaitDoneOrTimeout(done)

			// Try to play the game with the same id again and see that it returns an error.
			assertExternalEvent(gameProtocolError, ClientOutgoingEventsTopic, g, done, func(states []string) {
			})
			pb.PublishExternalEvent(ready, ClientIncomingEventsTopic)

			WaitDoneOrTimeout(done)
		})
	})
	Context("when the service is in slave mode", func() {
		It("registers the player and forwards its events to the master", func() {
			s.mode = ModeSlave
			pb.Bus.Subscribe(MasterOutgoingEventsTopic, func(e interface{}) {
				defer GinkgoRecover()
				ev := e.(*proto.Event)
				if ev.Name == PlayerReady {
					defer func() {
						done <- struct{}{}
					}()
					Expect(len(s.players["0"])).To(Equal(1))
					Expect(s.networks["a"]).To(Equal(int32(30000)))
					Expect(len(s.networks)).To(Equal(1))
					Expect(ev.Players[0].Port).To(Equal(int32(30000)))
				}
			})
			go s.Start()
			s.WaitUntilReady(timeout)

			ready := GenerateEvents(PlayerReady, "0")[0]
			player := &proto.Player{
				Ip:  frontendAddress,
				Id:  0,
				Pod: "a",
			}
			ready.Players[0] = player
			pb.PublishExternalEvent(ready, ClientIncomingEventsTopic)

			WaitDoneOrTimeout(done)
		})
	})
})

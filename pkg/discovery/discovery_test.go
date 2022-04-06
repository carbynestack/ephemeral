//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package discovery

import (
	"fmt"
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
	generateDiscoveryNGTestsWithPlayerCount(2)
	generateDiscoveryNGTestsWithPlayerCount(5)
})

func generateDiscoveryNGTestsWithPlayerCount(playerCount int) {

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
			FreePorts: make([]int32, playerCount),
		}
		for i := range n.FreePorts {
			n.FreePorts[i] = int32(30000 + i)
		}

		frontendAddress = "192.168.0.1"
		conf := &FakeDClient{}
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
				for i := 0; i < playerCount; i++ {
					pb.PublishExternalEvent(ev, ClientIncomingEventsTopic)
				}
				WaitDoneOrTimeout(done)
			})
			It("receives addresses of the players that joined the game", func() {
				playersReady := GenerateEvents(PlayersReady, "0")[0]
				_, allPlayerReadyEvents := createPlayersAndPlayerReadyEvents(playerCount, frontendAddress)
				assertExternalEventBody(playersReady, ClientOutgoingEventsTopic, g, done, func(event *proto.Event) {
					Expect(len(event.Players)).To(Equal(playerCount))
					for i := 0; i < playerCount; i++ {
						Expect(event.Players[i].Ip).To(Equal(frontendAddress))
						Expect(event.Players[i].Port).NotTo(BeZero())
					}
					Expect(len(s.pods)).To(Equal(playerCount))
					for i := 0; i < playerCount; i++ {
						podName := fmt.Sprintf("pod%d", i+1)
						Expect(s.pods[podName]).To(Equal(int32(i)))
					}
				})
				go s.Start()
				s.WaitUntilReady(timeout)
				for _, playerReadyEvent := range allPlayerReadyEvents {
					pb.PublishExternalEvent(playerReadyEvent, ClientIncomingEventsTopic)
				}
				WaitDoneOrTimeout(done)
			})

			It("doesn't create the player twice", func() {
				playersReady := GenerateEvents(PlayersReady, "0")[0]
				allPlayers, allPlayerReadyEvents := createPlayersAndPlayerReadyEvents(playerCount, frontendAddress)
				playerOneTCPCheckSuccess := GenerateEvents(TCPCheckSuccess, "0")[0]
				playerOneTCPCheckSuccess.Players[0] = allPlayers[0]
				assertExternalEventBody(playersReady, ClientOutgoingEventsTopic, g, done, func(event *proto.Event) {
					Expect(len(event.Players)).To(Equal(playerCount))
				})
				go s.Start()
				s.WaitUntilReady(timeout)
				for _, playerReadyEvent := range allPlayerReadyEvents {
					pb.PublishExternalEvent(playerReadyEvent, ClientIncomingEventsTopic)
				}
				pb.PublishExternalEvent(playerOneTCPCheckSuccess, ClientIncomingEventsTopic)
				WaitDoneOrTimeout(done)
			})
		})
		Context("a single player sends 2 messages in a row", func() {
			It("doesn't create the second network", func() {
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
				for i := 0; i < playerCount; i++ {
					// ToDo: Isn't it an error if the same player sends its ReadyMessage multiple times?
					//    Or rather, should we in such a case really go to PlayersReady?
					pb.PublishExternalEvent(playerOneIsReady, ClientIncomingEventsTopic)
				}
				WaitDoneOrTimeout(done)
			})
		})
		Context("an event from a foreign cluster is received", func() {
			It("doesn't create a network for it", func() {
				playersReady := GenerateEvents(PlayersReady, "0")[0]

				foreignFrontendAddress := "192.168.0.2"
				allPlayers, allPlayerReadyEvents := createPlayersAndPlayerReadyEvents(playerCount, foreignFrontendAddress)

				allPlayers[0].Ip = frontendAddress

				assertExternalEventBody(playersReady, ClientOutgoingEventsTopic, g, done, func(event *proto.Event) {
					// Only a single of the ports was used, all others are still free.
					Expect(len(n.FreePorts)).To(Equal(playerCount - 1))
				})
				go s.Start()
				s.WaitUntilReady(timeout)

				for _, playerReadyEvent := range allPlayerReadyEvents {
					pb.PublishExternalEvent(playerReadyEvent, ClientIncomingEventsTopic)
				}
				WaitDoneOrTimeout(done)
			})
		})
		Context("an event from the same pod but with different game id comes", func() {
			It("it uses the existing network configuration", func() {
				playersReady := GenerateEvents(PlayersReady, "0")[0]
				playersReady1 := GenerateEvents(PlayersReady, "1")[0]
				allPlayers, allPlayerReadyEventsInGame0 := createPlayersAndPlayerReadyEvents(playerCount, frontendAddress)
				allPlayerReadyEventsInGame1 := make([]*proto.Event, playerCount)
				for i := 0; i < playerCount; i++ {
					allPlayerReadyEventsInGame1[i] = GenerateEvents(PlayerReady, "1")[0]
					allPlayerReadyEventsInGame1[i].Players[0] = allPlayers[i]
				}
				assertExternalEventBody(playersReady, ClientOutgoingEventsTopic, g, done, func(event *proto.Event) {
					pp, _ := s.players["0"]
					for i := 0; i < playerCount; i++ {
						p := pp[PlayerID(int32(i))]
						Expect(p.Port).To(Equal(int32(30000 + i)))
					}
				})
				assertExternalEventBody(playersReady1, ClientOutgoingEventsTopic, g, done, func(event *proto.Event) {
					pp, _ := s.players["1"]
					for i := 0; i < playerCount; i++ {
						p := pp[PlayerID(int32(i))]
						Expect(p.Port).To(Equal(int32(30000 + i)))
					}
				})
				go s.Start()
				s.WaitUntilReady(timeout)
				for _, playerReadyEventInGame0 := range allPlayerReadyEventsInGame0 {
					pb.PublishExternalEvent(playerReadyEventInGame0, ClientIncomingEventsTopic)
				}
				time.Sleep(100 * time.Millisecond)
				for _, playerReadyEventInGame1 := range allPlayerReadyEventsInGame1 {
					pb.PublishExternalEvent(playerReadyEventInGame1, ClientIncomingEventsTopic)
				}
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
					statesAsserter := NewStatesAsserter(states)
					statesAsserter.ExpectNext().To(Equal(Init))
					for i := 0; i < playerCount; i++ {
						statesAsserter.ExpectNext().To(Equal(WaitPlayersReady))
					}
				})
				assertExternalEvent(ev[1], ClientOutgoingEventsTopic, g, done, func(states []string) {
					statesAsserter := NewStatesAsserter(states)
					statesAsserter.ExpectNext().To(Equal(Init))
					for i := 0; i < playerCount; i++ {
						statesAsserter.ExpectNext().To(Equal(WaitPlayersReady))
					}
				})
				go s.Start()
				s.WaitUntilReady(timeout)
				gameIds := make([]string, 2*playerCount)
				for i := 0; i < playerCount; i++ {
					gameIds[i] = "0"
					gameIds[i+playerCount] = "1"
				}
				events := GenerateEvents(PlayerReady, gameIds...)
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
			for i := 0; i < playerCount; i++ {
				pb.PublishExternalEvent(ready, ClientIncomingEventsTopic)
			}
			time.Sleep(50 * time.Millisecond)
			for i := 0; i < playerCount; i++ {
				pb.PublishExternalEvent(tcpCheckSuccess, ClientIncomingEventsTopic)
			}
			time.Sleep(50 * time.Millisecond)
			for i := 0; i < playerCount; i++ {
				pb.PublishExternalEvent(gameFinishedWithSuccess, ClientIncomingEventsTopic)
			}
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
			for i := 0; i < playerCount; i++ {
				pb.PublishExternalEvent(ready, ClientIncomingEventsTopic)
			}
			time.Sleep(50 * time.Millisecond)
			for i := 0; i < playerCount; i++ {
				pb.PublishExternalEvent(tcpCheckSuccess, ClientIncomingEventsTopic)
			}
			time.Sleep(50 * time.Millisecond)
			for i := 0; i < playerCount; i++ {
				pb.PublishExternalEvent(gameFinishedWithSuccess, ClientIncomingEventsTopic)
			}
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
}

func createPlayersAndPlayerReadyEvents(playerCount int, frontendAddress string) ([]*proto.Player, []*proto.Event) {
	allPlayers := make([]*proto.Player, playerCount)
	allPlayerReadyEvents := make([]*proto.Event, playerCount)
	for i := 0; i < playerCount; i++ {
		allPlayers[i] = &proto.Player{
			Ip:  frontendAddress,
			Id:  int32(i),
			Pod: fmt.Sprintf("pod%d", i+1),
		}
		allPlayerReadyEvents[i] = GenerateEvents(PlayerReady, "0")[0]
		allPlayerReadyEvents[i].Players[0] = allPlayers[i]
	}
	return allPlayers, allPlayerReadyEvents
}

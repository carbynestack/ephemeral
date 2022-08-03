//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package types

import "time"

const (
	// DiscoveryServiceStarted indicates the discovery service has started and ready to processIn events.
	DiscoveryServiceStarted = "DiscoveryServiceStarted"
	// GameProtocolError indicates an error in the protocol as a response to the message that were not delivered to the Game state machine.
	GameProtocolError = "GameProtocolError"
	// serviceEventsTopic represents the internal discovery service events.
	ServiceEventsTopic        = "serviceEvents"
	ClientIncomingEventsTopic = "clientIncomingEvents"
	ClientOutgoingEventsTopic = "clientOutgoingEvents"
	MasterOutgoingEventsTopic = "masterOutgoingEvents"
	DiscoveryTopic            = "discovery"
	// TODO: read this param from the config.
	Timeout = 20 * time.Second

	Init                      = "Init"
	Registering               = "Registering"
	Register                  = "Register"
	Registered                = "Registered"
	WaitPlayersReady          = "WaitPlayersReady"
	TCPCheck                  = "TCPCheck"
	TCPCheckSuccess           = "TCPCheckSuccess"
	TCPCheckFailure           = "TCPCheckFailure"
	Playing                   = "Playing"
	PlayerFinishedWithError   = "PlayerFinishedWithError"
	PlayerFinishedWithSuccess = "PlayerFinishedWithSuccess"
	PlayerReady               = "PlayerReady"
	PlayersReady              = "PlayersReady"
	GameIsReady               = "GameIsReady"
	GameError                 = "GameError"
	GameID                    = "gameID"
	TupleType                 = "TupleType"
	PlayingError              = "PlayingError"
	PlayerDone                = "PlayerDone"
	ModeSlave                 = "slave"
	ModeMaster                = "master"

	GameFinishedWithSuccess = "GameFinishedWithSuccess"
	GameFinishedWithError   = "GameFinishedWithError"
	GameDone                = "GameDone"
	TCPCheckSuccessAll      = "TCPCheckSuccessAll"
	GameSuccess             = "GameSuccess"
	WaitTCPCheck            = "WaitTCPCheck"
	StateTimeoutError       = "StateTimeoutError"
	SecretShare             = "SECRETSHARE"
	PlainText               = "PLAINTEXT"
	AmphoraSecret           = "AMPHORASECRET"
	ConnID                  = "ConnID"
	EventScope              = "EventScope"
	EventScopeAll           = "EventScopeAll"
	EventScopeSelf          = "EventScropeSelf"
)

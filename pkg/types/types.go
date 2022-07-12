//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package types

import (
	"context"
	"github.com/carbynestack/ephemeral/pkg/amphora"
	"github.com/carbynestack/ephemeral/pkg/castor"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"math/big"
	"time"

	mb "github.com/vardius/message-bus"
	"google.golang.org/grpc"
)

// WithBus is a type that contains a message bus.
type WithBus interface {
	Bus() mb.MessageBus
}

// DiscoveryClient is an interface for discovery service client.
type DiscoveryClient interface {
	Connect() (*grpc.ClientConn, error)
	Run(client pb.DiscoveryClient)
	GetIn() chan *pb.Event
	GetOut() chan *pb.Event
}

// DiscoveryConfig represents the condig of discovery service.
type DiscoveryConfig struct {
	FrontendURL  string `json:"frontendURL"`
	MasterHost   string `json:"masterHost"`
	MasterPort   string `json:"masterPort"`
	Slave        bool   `json:"slave"`
	StateTimeout string `json:"stateTimeout"`
	Port         string `json:"port"`
	BusSize      int    `json:"busSize"`
	PortRange    string `json:"portRange"`
	PlayerCount  int    `json:"playerCount"`
}

// Activation is an object that is received as an input from the Ephemeral client.
type Activation struct {
	AmphoraParams []string     `json:"amphoraParams"`
	SecretParams  []string     `json:"secretParams"`
	GameID        string       `json:"gameID"`
	Code          string       `json:"code"`
	Output        OutputConfig `json:"output"`
}

// ProxyConfig is the configuration used by the proxy when the connection between players is established.
type ProxyConfig struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	LocalPort string `json:"localPort"`
}

// CtxConfig contains both execution and platform specific parameters.
type CtxConfig struct {
	Act          *Activation
	Spdz         *SPDZEngineTypedConfig
	ProxyEntries []*ProxyConfig
	ErrCh        chan error
	Context      context.Context
}

// SPDZEngineConfig is the VPC specific configuration.
type SPDZEngineConfig struct {
	RetrySleep        string        `json:"retrySleep"`
	RetryTimeout      string        `json:"retryTimeout"`
	Prime             string        `json:"prime"`
	RInv              string        `json:"rInv"`
	MacKey            string        `json:"macKey"`
	SecurityParameter int32         `json:"securityParameter"`
	AmphoraConfig     AmphoraConfig `json:"amphoraConfig"`
	CastorConfig      CastorConfig  `json:"castorConfig"`
	FrontendURL       string        `json:"frontendURL"`
	PlayerID          int32         `json:"playerID"`
	PlayerCount       int32         `json:"playerCount"`
	MaxBulkSize       int32         `json:"maxBulkSize"`
	DiscoveryAddress  string        `json:"discoveryAddress"`
}

// AmphoraConfig specifies the amphora host parameters.
type AmphoraConfig struct {
	Host   string `json:"host"`
	Scheme string `json:"scheme"`
	Path   string `json:"path"`
}

// CastorConfig specifies the castor host and tuple stock parameters.
type CastorConfig struct {
	Host       string `json:"host"`
	Scheme     string `json:"scheme"`
	Path       string `json:"path"`
	TupleStock int32  `json:"tupleStock"`
}

// OutputConfig defines how the output of the app execution is treated.
type OutputConfig struct {
	Type string `json:"type"`
}

// SPDZEngineTypedConfig reflects SPDZEngineConfig, but it contains the real property types.
// We need this type, since the default json decoder doesn't know how to deserialize big.Int.
type SPDZEngineTypedConfig struct {
	RetrySleep        time.Duration
	RetryTimeout      time.Duration
	Prime             big.Int
	RInv              big.Int
	MacKey            big.Int
	SecurityParameter int32
	AmphoraClient     amphora.AbstractClient
	CastorClient      castor.AbstractClient
	TupleStock        int32
	PlayerID          int32
	PlayerCount       int32
	FrontendURL       string
	MaxBulkSize       int32
	DiscoveryAddress  string
}

type contextKey string

var (
	ActCtx   = contextKey("activation")
	SpdzCtx  = contextKey("spdz")
	ProxyCtx = contextKey("proxy")
)

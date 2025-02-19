// Copyright (c) 2021-2024 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package types

import (
	"context"
	"crypto/tls"
	"github.com/carbynestack/ephemeral/pkg/amphora"
	"github.com/carbynestack/ephemeral/pkg/castor"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"github.com/carbynestack/ephemeral/pkg/opa"
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

// DiscoveryConfig represents the config of discovery service.
type DiscoveryConfig struct {
	FrontendURL        string `json:"frontendURL"`
	MasterHost         string `json:"masterHost"`
	MasterPort         string `json:"masterPort"`
	Slave              bool   `json:"slave"`
	StateTimeout       string `json:"stateTimeout"`
	ComputationTimeout string `json:"computationTimeout"`
	ConnectTimeout     string `json:"connectTimeout"`
	Port               string `json:"port"`
	BusSize            int    `json:"busSize"`
	PortRange          string `json:"portRange"`
	PlayerCount        int    `json:"playerCount"`
}

// DiscoveryTypedConfig reflects DiscoveryConfig, but it contains the real property types
type DiscoveryTypedConfig struct {
	FrontendURL        string
	MasterHost         string
	MasterPort         string
	Slave              bool
	StateTimeout       time.Duration
	ComputationTimeout time.Duration
	ConnectTimeout     time.Duration
	Port               string
	BusSize            int
	PortRange          string
	PlayerCount        int
}

// NetworkControllerConfig represents the config of the network-controller service.
type NetworkControllerConfig struct {
	TlsEnabled bool   `json:"tlsEnabled"`
	TlsSecret  string `json:"tlsSecret"`
}

// NetworkControllerTypedConfig reflects NetworkControllerConfig, but it contains the real property types
type NetworkControllerTypedConfig struct {
	TlsEnabled bool
	TlsSecret  string
}

// Activation is an object that is received as an input from the Ephemeral client.
type Activation struct {
	AmphoraParams []string     `json:"amphoraParams"`
	SecretParams  []string     `json:"secretParams"`
	GameID        string       `json:"gameID"`
	Code          string       `json:"code"`
	Output        OutputConfig `json:"output"`
}

type ActivationInput struct {
	SecretId     string `json:"secretId"`
	Owner        string `json:"owner"`
	AccessPolicy string `json:"accessPolicy"`
}

// ProxyConfig is the configuration used by the proxy when the connection between players is established.
type ProxyConfig struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	LocalPort string `json:"localPort"`
}

// CtxConfig contains both execution and platform specific parameters.
type CtxConfig struct {
	AuthorizedUser string
	Act            *Activation
	Spdz           *SPDZEngineTypedConfig
	ProxyEntries   []*ProxyConfig
	ErrCh          chan error
	Context        context.Context
}

// SPDZEngineConfig is the VPC specific configuration.
type SPDZEngineConfig struct {
	ProgramIdentifier       string `json:"programIdentifier"`
	AuthUserIdField         string `json:"authUserIdField"`
	RetrySleep              string `json:"retrySleep"`
	NetworkEstablishTimeout string `json:"networkEstablishTimeout"`
	Prime                   string `json:"prime"`
	RInv                    string `json:"rInv"`
	GfpMacKey               string `json:"gfpMacKey"`
	Gf2nMacKey              string `json:"gf2nMacKey"`
	Gf2nBitLength           int32  `json:"gf2nBitLength"`
	// Gf2nStorageSize represents the size in bytes for each gf2n element e.g. depending on the 'USE_GF2N_LONG' flag
	// being set when compiling SPDZ where storage size is 16 for USE_GF2N_LONG=1, or 8 if set to 0
	Gf2nStorageSize    int32                 `json:"gf2nStorageSize"`
	PrepFolder         string                `json:"prepFolder"`
	OpaConfig          OpaConfig             `json:"opaConfig"`
	AmphoraConfig      AmphoraConfig         `json:"amphoraConfig"`
	CastorConfig       CastorConfig          `json:"castorConfig"`
	FrontendURL        string                `json:"frontendURL"`
	TlsEnabled         bool                  `json:"tlsEnabled"`
	PlayerID           int32                 `json:"playerID"`
	PlayerCount        int32                 `json:"playerCount"`
	MaxBulkSize        int32                 `json:"maxBulkSize"`
	DiscoveryConfig    DiscoveryClientConfig `json:"discoveryConfig"`
	StateTimeout       string                `json:"stateTimeout"`
	ComputationTimeout string                `json:"computationTimeout"`
}

type OpaConfig struct {
	Endpoint      string `json:"endpoint"`
	PolicyPackage string `json:"policyPackage"`
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

// Config contains TCP connection properties of Carrier.
type DiscoveryClientConfig struct {
	Port           string `json:"port"`
	Host           string `json:"host"`
	ConnectTimeout string `json:"connectTimeout"`
}

// DiscoveryClientTypedConfig reflects DiscoveryClientConfig, but it contains the real property types.
type DiscoveryClientTypedConfig struct {
	Port           string
	Host           string
	ConnectTimeout time.Duration
}

// OutputConfig defines how the output of the app execution is treated.
type OutputConfig struct {
	Type string `json:"type"`
}

// SPDZEngineTypedConfig reflects SPDZEngineConfig, but it contains the real property types.
// We need this type, since the default json decoder doesn't know how to deserialize big.Int.
type SPDZEngineTypedConfig struct {
	ProgramIdentifier       string
	AuthUserIdField         string
	RetrySleep              time.Duration
	NetworkEstablishTimeout time.Duration
	Prime                   big.Int
	RInv                    big.Int
	GfpMacKey               big.Int
	Gf2nMacKey              string
	Gf2nBitLength           int32
	Gf2nStorageSize         int32
	PrepFolder              string
	OpaClient               opa.AbstractClient
	AmphoraClient           amphora.AbstractClient
	CastorClient            castor.AbstractClient
	TupleStock              int32
	PlayerID                int32
	PlayerCount             int32
	FrontendURL             string
	MaxBulkSize             int32
	DiscoveryConfig         DiscoveryClientTypedConfig
	StateTimeout            time.Duration
	ComputationTimeout      time.Duration
	TlsEnabled              bool
	TlsConfig               *tls.Config
}

// Copyright (c) 2021-2025 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"

	"github.com/carbynestack/ephemeral/pkg/amphora"
	"github.com/carbynestack/ephemeral/pkg/castor"
	. "github.com/carbynestack/ephemeral/pkg/ephemeral"
	l "github.com/carbynestack/ephemeral/pkg/logger"
	"github.com/carbynestack/ephemeral/pkg/opa"
	"github.com/carbynestack/ephemeral/pkg/utils"
	"os"

	"math/big"
	"net/http"
	"net/url"
	"time"

	. "github.com/carbynestack/ephemeral/pkg/types"

	"go.uber.org/zap"
)

const (
	defaultConfig    = "/etc/config/config.json"
	defaultTlsConfig = "/etc/tls"
	defaultPort      = "8080"
)

func main() {
	logger, err := l.NewDevelopmentLogger()
	if err != nil {
		panic(err)
	}
	config, err := ParseConfig(defaultConfig)
	if err != nil {
		panic(err)
	}
	logger.Debugf("Starting with the config:\n%+v", config)
	if err != nil {
		panic(err)
	}
	handler, err := GetHandlerChain(config, logger)
	if err != nil {
		panic(err)
	}
	http.Handle("/", handler)
	logger.Info("Starting http server")
	err = http.ListenAndServe("localhost:"+defaultPort, nil)
	if err != nil {
		panic(err)
	}
}

// GetHandlerChain returns a chain of handlers that are used to process HTTP requests.
func GetHandlerChain(conf *SPDZEngineConfig, logger *zap.SugaredLogger) (http.Handler, error) {
	typedConfig, err := InitTypedConfig(conf, logger)
	if err != nil {
		return nil, err
	}
	spdzClient, err := NewSPDZEngine(logger, utils.NewCommander(), typedConfig)
	if err != nil {
		return nil, err
	}
	server := NewServer(conf.AuthUserIdField, spdzClient.Compile, spdzClient.Activate, logger, typedConfig)
	activationHandler := http.HandlerFunc(server.ActivationHandler)
	// Apply in Order:
	// 1) MethodFilter: Check that only POST Requests can go through
	// 2) RequestFilter: Check that Request Body is set properly and Sets the CtxConfig to the request
	// 3) CompilationHandler: Compiles the script if ?compile=true
	// 4) ActivationHandler: Runs the script
	filterChain := server.MethodFilter(server.RequestFilter(server.CompilationHandler(activationHandler)))
	return filterChain, nil
}

// ParseConfig reads the configuration file content.
func ParseConfig(path string) (*SPDZEngineConfig, error) {
	bytes, err := utils.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var conf SPDZEngineConfig
	err = json.Unmarshal(bytes, &conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}

// InitTypedConfig converts the string parameters that were parsed by standard json parser to
// the parameters which are used internally, e.g. string -> time.Duration.
func InitTypedConfig(conf *SPDZEngineConfig, logger *zap.SugaredLogger) (*SPDZEngineTypedConfig, error) {
	retrySleep, err := time.ParseDuration(conf.RetrySleep)
	if err != nil {
		return nil, err
	}
	var p, rInv, gfpMacKey big.Int
	_, ok := p.SetString(conf.Prime, 10)
	if !ok {
		return nil, errors.New("wrong prime number format")
	}
	_, ok = rInv.SetString(conf.RInv, 10)
	if !ok {
		return nil, errors.New("wrong rInv format")
	}
	_, ok = gfpMacKey.SetString(conf.GfpMacKey, 10)
	if !ok {
		return nil, errors.New("wrong gfpMacKey format")
	}
	stateTimeout, err := time.ParseDuration(conf.StateTimeout)
	if err != nil {
		return nil, err
	}
	computationTimeout, err := time.ParseDuration(conf.ComputationTimeout)
	if err != nil {
		return nil, err
	}
	connectTimeout, err := time.ParseDuration(conf.DiscoveryConfig.ConnectTimeout)
	if err != nil {
		return nil, err
	}
	networkEstablishTimeout, err := time.ParseDuration(conf.NetworkEstablishTimeout)
	if err != nil {
		return nil, err
	}
	programIdentifier, ok := os.LookupEnv("EPHEMERAL_PROGRAM_IDENTIFIER")
	if !ok {
		programIdentifier = conf.ProgramIdentifier
	}

	policyPackage, ok := os.LookupEnv("EPHEMERAL_OPA_POLICY_PACKAGE")
	if !ok {
		policyPackage = conf.OpaConfig.PolicyPackage
	}
	opaClient, err := opa.NewClient(logger, conf.OpaConfig.Endpoint, policyPackage)
	if err != nil {
		return nil, err
	}

	amphoraURL := url.URL{
		Host:   conf.AmphoraConfig.Host,
		Scheme: conf.AmphoraConfig.Scheme,
		Path:   conf.AmphoraConfig.Path,
	}
	amphoraClient, err := amphora.NewClient(amphoraURL)
	if err != nil {
		return nil, err
	}

	castorURL := url.URL{
		Host:   conf.CastorConfig.Host,
		Scheme: conf.CastorConfig.Scheme,
		Path:   conf.CastorConfig.Path,
	}
	castorClient, err := castor.NewClient(castorURL)
	if err != nil {
		return nil, err
	}

	var tlsConfig *tls.Config
	if conf.TlsEnabled {
		var err error
		tlsConfig, err = utils.CreateTLSConfig(defaultTlsConfig)
		if err != nil {
			return nil, err
		}
	}

	return &SPDZEngineTypedConfig{
		ProgramIdentifier:       programIdentifier,
		NetworkEstablishTimeout: networkEstablishTimeout,
		RetrySleep:              retrySleep,
		Prime:                   p,
		RInv:                    rInv,
		GfpMacKey:               gfpMacKey,
		Gf2nMacKey:              conf.Gf2nMacKey,
		Gf2nBitLength:           conf.Gf2nBitLength,
		Gf2nStorageSize:         conf.Gf2nStorageSize,
		PrepFolder:              conf.PrepFolder,
		OpaClient:               opaClient,
		AmphoraClient:           amphoraClient,
		CastorClient:            castorClient,
		TupleStock:              conf.CastorConfig.TupleStock,
		PlayerID:                conf.PlayerID,
		PlayerCount:             conf.PlayerCount,
		FrontendURL:             conf.FrontendURL,
		MaxBulkSize:             conf.MaxBulkSize,
		DiscoveryConfig: DiscoveryClientTypedConfig{
			Host:           conf.DiscoveryConfig.Host,
			Port:           conf.DiscoveryConfig.Port,
			ConnectTimeout: connectTimeout,
		},
		StateTimeout:       stateTimeout,
		ComputationTimeout: computationTimeout,
		TlsConfig:          tlsConfig,
	}, nil
}

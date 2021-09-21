//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package main

import (
	"encoding/json"
	"errors"
	"github.com/carbynestack/ephemeral/pkg/amphora"
	. "github.com/carbynestack/ephemeral/pkg/ephemeral"
	l "github.com/carbynestack/ephemeral/pkg/logger"
	"github.com/carbynestack/ephemeral/pkg/utils"

	. "github.com/carbynestack/ephemeral/pkg/types"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

const (
	defaultConfig = "/etc/config/config.json"
	defaultPort   = "8080"
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
	typedConfig, err := InitTypedConfig(conf)
	if err != nil {
		return nil, err
	}
	spdzClient := NewSPDZEngine(logger, utils.NewCommander(), typedConfig)
	server := NewServer(spdzClient.Compile, spdzClient.Activate, logger, typedConfig)
	activationHandler := http.HandlerFunc(server.ActivationHandler)
	filterChain := server.MethodFilter(server.BodyFilter(server.CompilationHandler(activationHandler)))
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
func InitTypedConfig(conf *SPDZEngineConfig) (*SPDZEngineTypedConfig, error) {
	retryTimeout, err := time.ParseDuration(conf.RetryTimeout)
	if err != nil {
		return nil, err
	}
	retrySleep, err := time.ParseDuration(conf.RetrySleep)
	if err != nil {
		return nil, err
	}
	var p, rInv big.Int
	_, ok := p.SetString(conf.Prime, 10)
	if !ok {
		return nil, errors.New("wrong prime number format")
	}
	_, ok = rInv.SetString(conf.RInv, 10)
	if !ok {
		return nil, errors.New("wrong rInv format")
	}

	amphoraURL := url.URL{
		Host:   conf.AmphoraConfig.Host,
		Scheme: conf.AmphoraConfig.Scheme,
		Path:   conf.AmphoraConfig.Path,
	}
	client, err := amphora.NewAmphoraClient(amphoraURL)
	if err != nil {
		return nil, err
	}

	return &SPDZEngineTypedConfig{
		RetryTimeout:     retryTimeout,
		RetrySleep:       retrySleep,
		Prime:            p,
		RInv:             rInv,
		AmphoraClient:    client,
		PlayerID:         conf.PlayerID,
		FrontendURL:      conf.FrontendURL,
		MaxBulkSize:      conf.MaxBulkSize,
		DiscoveryAddress: conf.DiscoveryAddress,
	}, nil
}

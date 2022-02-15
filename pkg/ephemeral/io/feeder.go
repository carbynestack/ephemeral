//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package io

import (
	"encoding/json"
	"fmt"
	"github.com/carbynestack/ephemeral/pkg/amphora"
	"github.com/carbynestack/ephemeral/pkg/ephemeral/network"
	. "github.com/carbynestack/ephemeral/pkg/types"
	"strings"

	"go.uber.org/zap"
)

// Feeder is an interface.
type Feeder interface {
	LoadByTagsAndSecretStoreAndFeed(act *Activation, feedPort string, ctx *CtxConfig) ([]byte, error)
	LoadFromSecretStoreAndFeed(act *Activation, feedPort string, ctx *CtxConfig) ([]byte, error)
	LoadFromRequestAndFeed(act *Activation, feedPort string, ctx *CtxConfig) ([]byte, error)
	Close() error
}

// NewAmphoraFeeder returns a new instance of amphora feeder.
func NewAmphoraFeeder(l *zap.SugaredLogger, conf *SPDZEngineTypedConfig) *AmphoraFeeder {
	dialer := network.RetryingDialerWithContext(conf.RetrySleep, conf.RetryTimeout, func() {
		l.Debug(fmt.Sprintf("retrying to connect to SPDZ socket after %s", conf.RetrySleep))
	})

	carrier := &Carrier{
		Dialer: dialer,
		Packer: &SPDZPacker{
			MaxBulkSize: conf.MaxBulkSize,
		},
	}
	return &AmphoraFeeder{
		logger:  l,
		conf:    conf,
		carrier: carrier,
	}
}

// AmphoraFeeder provides parameters to the SPDZ execution based on the given activation.
type AmphoraFeeder struct {
	logger  *zap.SugaredLogger
	conf    *SPDZEngineTypedConfig
	carrier AbstractCarrier
}

// LoadByTagsAndSecretStoreAndFeed loads input parameters from Amphora based on tag filter.
func (f *AmphoraFeeder) LoadByTagsAndSecretStoreAndFeed(act *Activation, feedPort string, ctx *CtxConfig) ([]byte, error) {

	f.logger.Debug("Called LoadByTagsAndSecretStoreAndFeed()")

	var secretShareIds []string
	client := f.conf.AmphoraClient

	// TODO implement pagination
	params := &amphora.ObjectListRequestParams{
		Filter:        strings.Join(act.TagFilterParams, ","),
		PageNumber:    0,
		PageSize:      0,
		SortProperty:  "",
		SortDirection: "",
	}

	f.logger.Debugw("Resolving tags to secretIds", "params", params, GameID, ctx.Act.GameID)
	metadataPage, err := client.GetObjectList(params)
	if err != nil {
		return nil, err
	}

	for _, metadata := range metadataPage.Content {
		secretShareIds = append(secretShareIds, metadata.SecretID)
	}

	// TODO below here it's duplicated code from LoadFromSecretStoreAndFeed()
	// using secretShareIds instead act.AmphoraParams
	f.logger.Debugw(fmt.Sprintf("Fetching secret shares for secretIds=%s", secretShareIds), GameID, ctx.Act.GameID)
	var data []string
	for i := range secretShareIds {
		osh, err := client.GetSecretShare(secretShareIds[i])
		if err != nil {
			return nil, err
		}
		data = append(data, osh.Data)
	}
	resp, err := f.feedAndRead(data, feedPort, ctx)
	if err != nil {
		return nil, err
	}
	// Write to amphora if required and return amphora secret ids.
	if act.Output.Type == AmphoraSecret {
		ids, err := f.writeToAmphora(act, *resp)
		if err != nil {
			return nil, err
		}
		resp.Response = ids
	}
	return json.Marshal(&resp)
}

func (f *AmphoraFeeder) LoadFromSecretStoreAndFeed(act *Activation, feedPort string, ctx *CtxConfig) ([]byte, error) {

	f.logger.Debug("Called LoadFromSecretStoreAndFeed()")

	var data []string
	client := f.conf.AmphoraClient
	for i := range act.AmphoraParams {
		osh, err := client.GetSecretShare(act.AmphoraParams[i])
		if err != nil {
			return nil, err
		}
		data = append(data, osh.Data)
	}
	resp, err := f.feedAndRead(data, feedPort, ctx)
	if err != nil {
		return nil, err
	}
	// Write to amphora if required and return amphora secret ids.
	if act.Output.Type == AmphoraSecret {
		ids, err := f.writeToAmphora(act, *resp)
		if err != nil {
			return nil, err
		}
		resp.Response = ids
	}
	return json.Marshal(&resp)
}

// LoadFromRequestAndFeed loads input parameteters from the request body.
func (f *AmphoraFeeder) LoadFromRequestAndFeed(act *Activation, feedPort string, ctx *CtxConfig) ([]byte, error) {
	resp, err := f.feedAndRead(act.SecretParams, feedPort, ctx)
	if err != nil {
		return nil, err
	}
	// Write to amphora if required and return amphora secret ids.
	if act.Output.Type == AmphoraSecret {
		ids, err := f.writeToAmphora(act, *resp)
		if err != nil {
			return nil, err
		}
		resp.Response = ids
	}
	return json.Marshal(&resp)
}

// Close closes the underlying socket connection.
func (f *AmphoraFeeder) Close() error {
	return f.carrier.Close()
}

// feedAndRead takes a slice of base64 encoded secret shared parameters along with the port where SPDZ runtime is listening for the input. The base64 input params are converted into a form digestable by SPDZ and sent to the socket. The runtime must send back a response for this function to finish without an error.
func (f *AmphoraFeeder) feedAndRead(params []string, port string, ctx *CtxConfig) (*Result, error) {
	var conv ResponseConverter
	f.logger.Debugw(fmt.Sprintf("Received secret shared parameters %s", params), GameID, ctx.Act.GameID)
	isBulk := false
	// It must be defined in the Activation whether plaintext or secret shared output is expected.
	switch strings.ToUpper(ctx.Act.Output.Type) {
	case PlainText:
		mpcParams := []interface{}{&f.conf.RInv, &f.conf.Prime}
		conv = &PlaintextConverter{
			Params: mpcParams,
		}
	case SecretShare:
		conv = &SecretSharesConverter{}
	case AmphoraSecret:
		conv = &SecretSharesConverter{}
		isBulk = true
	default:
		return nil, fmt.Errorf("no output config is given, either %s, %s or %s must be defined", PlainText, SecretShare, AmphoraSecret)
	}
	err := f.carrier.Connect(ctx.Context, ctx.Spdz.PlayerID, "localhost", port)
	defer f.carrier.Close()
	if err != nil {
		return nil, err
	}
	secrets := []amphora.SecretShare{}
	for i := range params {
		secret := amphora.SecretShare{
			Data: params[i],
		}
		secrets = append(secrets, secret)
	}
	err = f.carrier.Send(secrets)
	if err != nil {
		return nil, err
	}
	return f.carrier.Read(conv, isBulk)
}

func (f *AmphoraFeeder) writeToAmphora(act *Activation, resp Result) ([]string, error) {
	client := f.conf.AmphoraClient
	os := amphora.SecretShare{
		SecretID: act.GameID,
		// When writing to Amphora, the slice has exactly 1 element.
		Data: resp.Response[0],
		Tags: []amphora.Tag{
			amphora.Tag{
				ValueType: "STRING",
				Key:       "gameID",
				Value:     act.GameID,
			},
		},
	}
	err := client.CreateSecretShare(&os)
	f.logger.Infow(fmt.Sprintf("Created secret share with id %s", os.SecretID), GameID, act.GameID)
	if err != nil {
		return nil, err
	}
	return []string{act.GameID}, nil
}

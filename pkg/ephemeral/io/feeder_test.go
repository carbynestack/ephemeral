// Copyright (c) 2021-2024 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package io

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/carbynestack/ephemeral/pkg/amphora"
	. "github.com/carbynestack/ephemeral/pkg/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var _ = Describe("Feeder", func() {
	var (
		carrier *FakeCarrier
		act     *Activation
		f       AmphoraFeeder
		conf    *CtxConfig
	)
	BeforeEach(func() {
		carrier = &FakeCarrier{}
		act = &Activation{
			AmphoraParams: []string{"a"},
			GameID:        "71b2a100-f3f6-11e9-81b4-2a2ae2dbcce4",
			Output: OutputConfig{
				Type: PlainText,
			},
		}
		f = AmphoraFeeder{
			conf: &SPDZEngineTypedConfig{
				AmphoraClient: &FakeAmphoraClient{},
				OpaClient:     &FakeOpaClient{},
			},
			carrier: carrier,
			logger:  zap.NewNop().Sugar(),
		}
		conf = &CtxConfig{
			Act:     act,
			Context: context.TODO(),
			Spdz:    &SPDZEngineTypedConfig{PlayerCount: 2},
		}
	})

	Context("when reading objects", func() {
		Context("when reading objects from amphora", func() {
			Context("when output type is plaintext", func() {
				It("responds with the result", func() {
					res, err := f.LoadFromSecretStoreAndFeed(act, "", conf)
					Expect(err).NotTo(HaveOccurred())
					var response Result
					json.Unmarshal(res, &response)
					Expect(response.Response[0]).To(Equal("yay"))
					Expect(carrier.isBulk).To(BeFalse())
				})
			})
			Context("when output type is secret share", func() {
				It("responds with the result", func() {
					act.Output.Type = SecretShare
					res, err := f.LoadFromSecretStoreAndFeed(act, "", conf)
					Expect(err).NotTo(HaveOccurred())
					var response Result
					json.Unmarshal(res, &response)
					Expect(response.Response[0]).To(Equal("yay"))
					Expect(carrier.isBulk).To(BeFalse())
				})
			})
			Context("when output type is amphora secret", func() {
				It("responds with the secretID=gameID", func() {
					act.Output.Type = AmphoraSecret
					res, err := f.LoadFromSecretStoreAndFeed(act, "", conf)
					Expect(err).NotTo(HaveOccurred())
					var response Result
					json.Unmarshal(res, &response)
					Expect(response.Response[0]).To(Equal(act.GameID))
					Expect(carrier.isBulk).To(BeTrue())
				})
			})
			Context("when no output type is given", func() {
				It("returns an error", func() {
					act.Output.Type = ""
					res, err := f.LoadFromSecretStoreAndFeed(act, "", conf)
					Expect(err).To(HaveOccurred())
					Expect(res).To(BeNil())
				})
			})
			Context("when getting an object fails", func() {
				It("returns an error", func() {
					f.conf.AmphoraClient = &BrokenReadFakeAmphoraClient{}
					res, err := f.LoadFromSecretStoreAndFeed(act, "", conf)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("amphora read error"))
					Expect(res).To(BeNil())
				})
			})
			Context("when writing an object fails", func() {
				It("returns an error", func() {
					f.conf.AmphoraClient = &BrokenWriteFakeAmphoraClient{}
					act.Output.Type = AmphoraSecret
					res, err := f.LoadFromSecretStoreAndFeed(act, "", conf)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("amphora create error"))
					Expect(res).To(BeNil())
				})
			})
		})
		Context("when reading parameters from the body", func() {
			Context("when output is to be written in the http response", func() {
				It("responds with the result", func() {
					act.Output.Type = SecretShare
					res, err := f.LoadFromRequestAndFeed(act, "", conf)
					Expect(err).NotTo(HaveOccurred())
					var response Result
					json.Unmarshal(res, &response)
					Expect(response.Response[0]).To(Equal("yay"))
					Expect(carrier.isBulk).To(BeFalse())
				})
			})
			Context("when output is to be written to amphora", func() {
				It("responds with the secretID=gameID", func() {
					act.Output.Type = AmphoraSecret
					res, err := f.LoadFromRequestAndFeed(act, "", conf)
					Expect(err).NotTo(HaveOccurred())
					var response Result
					json.Unmarshal(res, &response)
					Expect(response.Response[0]).To(Equal(act.GameID))
					Expect(carrier.isBulk).To(BeTrue())
				})
			})
			Context("when creating an object fails", func() {
				It("returns an error", func() {
					f.conf.AmphoraClient = &BrokenWriteFakeAmphoraClient{}
					act.Output.Type = AmphoraSecret
					res, err := f.LoadFromRequestAndFeed(act, "", conf)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("amphora create error"))
					Expect(res).To(BeNil())
				})
			})
			Context("when connection to spdz port doesn't succeed", func() {
				It("returns an error", func() {
					f.carrier = &BrokenConnectFakeCarrier{}
					act.Output.Type = AmphoraSecret
					res, err := f.LoadFromRequestAndFeed(act, "", conf)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("carrier connect error"))
					Expect(res).To(BeNil())
				})
			})
			Context("when sending through the carrier fails", func() {
				It("returns an error", func() {
					f.carrier = &BrokenSendFakeCarrier{}
					act.Output.Type = AmphoraSecret
					res, err := f.LoadFromRequestAndFeed(act, "", conf)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("carrier send error"))
					Expect(res).To(BeNil())
				})
			})
		})
	})

	Context("when creating a new instance of feeder", func() {
		It("sets required parameters and returns a new instance", func() {
			l := zap.NewNop().Sugar()
			conf := &SPDZEngineTypedConfig{
				PlayerID: 0,
			}
			f := NewAmphoraFeeder(l, conf)
			Expect(f.conf.PlayerID).To(Equal(int32(0)))
		})
	})
})

type FakeOpaClient struct {
}

func (f *FakeOpaClient) GenerateTags(interface{}) ([]amphora.Tag, error) {
	return []amphora.Tag{}, nil
}

func (f *FakeOpaClient) CanExecute(interface{}) (bool, error) {
	return true, nil
}

type FakeAmphoraClient struct {
}

func (f *FakeAmphoraClient) GetSecretShare(string, string) (amphora.SecretShare, error) {
	return amphora.SecretShare{}, nil
}
func (f *FakeAmphoraClient) CreateSecretShare(*amphora.SecretShare) error {
	return nil
}

type BrokenReadFakeAmphoraClient struct {
}

func (f *BrokenReadFakeAmphoraClient) GetSecretShare(string, string) (amphora.SecretShare, error) {
	return amphora.SecretShare{}, errors.New("amphora read error")
}
func (f *BrokenReadFakeAmphoraClient) CreateSecretShare(*amphora.SecretShare) error {
	return nil
}

type BrokenWriteFakeAmphoraClient struct {
}

func (f *BrokenWriteFakeAmphoraClient) GetSecretShare(string, string) (amphora.SecretShare, error) {
	return amphora.SecretShare{}, nil
}
func (f *BrokenWriteFakeAmphoraClient) CreateSecretShare(*amphora.SecretShare) error {
	return errors.New("amphora create error")
}

type FakeCarrier struct {
	isBulk bool
}

func (f *FakeCarrier) Connect(context.Context, int32, string, string) error {
	return nil
}

func (f *FakeCarrier) Read(conv ResponseConverter, isBulk bool) (*Result, error) {
	f.isBulk = isBulk
	return &Result{Response: []string{"yay"}}, nil
}

func (f *FakeCarrier) Close() error {
	return nil
}

func (f *FakeCarrier) Send([]amphora.SecretShare) error {
	return nil
}

type BrokenConnectFakeCarrier struct {
	isBulk bool
}

func (f *BrokenConnectFakeCarrier) Connect(context.Context, int32, string, string) error {
	return errors.New("carrier connect error")
}

func (f *BrokenConnectFakeCarrier) Read(conv ResponseConverter, isBulk bool) (*Result, error) {
	f.isBulk = isBulk
	return &Result{Response: []string{"yay"}}, nil
}

func (f *BrokenConnectFakeCarrier) Close() error {
	return nil
}

func (f *BrokenConnectFakeCarrier) Send([]amphora.SecretShare) error {
	return nil
}

type BrokenSendFakeCarrier struct {
	isBulk bool
}

func (f *BrokenSendFakeCarrier) Connect(context.Context, int32, string, string) error {
	return nil
}

func (f *BrokenSendFakeCarrier) Read(conv ResponseConverter, isBulk bool) (*Result, error) {
	f.isBulk = isBulk
	return &Result{Response: []string{"yay"}}, nil
}

func (f *BrokenSendFakeCarrier) Close() error {
	return nil
}

func (f *BrokenSendFakeCarrier) Send([]amphora.SecretShare) error {
	return errors.New("carrier send error")
}

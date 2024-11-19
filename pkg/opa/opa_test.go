// Copyright (c) 2024 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0

package opa

import (
	"github.com/carbynestack/ephemeral/pkg/amphora"
	"net/http"
	"net/http/httptest"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("OpaClient", func() {
	Context("when creating a new client", func() {
		It("returns an error when the endpoint is invalid", func() {
			client, err := NewClient("invalid-url", "valid.policy.package")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid URL"))
			Expect(client).To(BeNil())
		})
		It("returns an error when the policy package is empty", func() {
			client, err := NewClient("http://valid-url.com", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("policy package is required"))
			Expect(client).To(BeNil())
		})
		It("returns a new client when the endpoint and policy package are valid", func() {
			client, err := NewClient("http://valid-url.com", "valid.policy.package")
			Expect(err).NotTo(HaveOccurred())
			Expect(client.URL.String()).To(Equal("http://valid-url.com"))
			Expect(client.PolicyPackage).To(Equal("valid.policy.package"))
		})
	})

	Context("when generating tags", func() {
		It("returns tags when the response is valid", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"result": [{"key": "tag1", "value": "value1", "valueType": "STRING"}, {"key": "tag2", "value": "1", "valueType": "LONG"}]}`))
			}))
			defer server.Close()

			u, _ := url.Parse(server.URL)
			client := &Client{
				URL:           *u,
				PolicyPackage: "test.policy.package",
				HttpClient:    http.Client{},
			}

			tags, err := client.GenerateTags(map[string]interface{}{"key": "value"})
			Expect(err).NotTo(HaveOccurred())
			Expect(tags).To(Equal([]amphora.Tag{
				{Key: "tag1", Value: "value1", ValueType: "STRING"},
				{Key: "tag2", Value: "1", ValueType: "LONG"}}))
		})
		It("returns an error when the response code is not 200", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"result": []}`))
			}))
			defer server.Close()

			u, _ := url.Parse(server.URL)
			client := &Client{
				URL:           *u,
				PolicyPackage: "test.policy.package",
				HttpClient:    http.Client{},
			}

			_, err := client.GenerateTags(map[string]interface{}{"key": "value"})
			Expect(err).To(HaveOccurred())
		})
		It("returns an error when the response body is invalid", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`invalid json`))
			}))
			defer server.Close()

			u, _ := url.Parse(server.URL)
			client := &Client{
				URL:           *u,
				PolicyPackage: "test.policy.package",
				HttpClient:    http.Client{},
			}

			_, err := client.GenerateTags(map[string]interface{}{"key": "value"})
			Expect(err).To(HaveOccurred())
		})
	})
})

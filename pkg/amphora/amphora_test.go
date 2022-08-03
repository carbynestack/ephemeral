//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package amphora_test

import (
	"encoding/json"
	. "github.com/carbynestack/ephemeral/pkg/utils"
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/carbynestack/ephemeral/pkg/amphora"
)

var _ = Describe("Amphora", func() {

	var (
		share SecretShare
		js    []byte
	)

	BeforeEach(func() {
		share = SecretShare{SecretID: "xyz"}
		js, _ = json.Marshal(&share)
	})
	Context("when retrieving a shared secret", func() {
		It("returns a shared object when it exists in amphora", func() {
			rt := MockedRoundTripper{ExpectedPath: "/intra-vcp/secret-shares/xyz", ReturnJSON: js, ExpectedResponseCode: http.StatusOK}
			HTTPClient := http.Client{Transport: &rt}
			client := Client{HTTPClient: HTTPClient, URL: url.URL{Host: "test", Scheme: "http"}}

			secret, err := client.GetSecretShare("xyz")
			Expect(secret.SecretID).To(Equal("xyz"))
			Expect(err).NotTo(HaveOccurred())
		})
		It("returns an error when shared secret does not exist", func() {
			rt := MockedRoundTripper{ExpectedPath: "/intra-vcp/secret-shares/xxx", ReturnJSON: js, ExpectedResponseCode: http.StatusOK}
			HTTPClient := http.Client{Transport: &rt}
			client := Client{HTTPClient: HTTPClient, URL: url.URL{Host: "test", Scheme: "http"}}

			_, err := client.GetSecretShare("xyz")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when creating a shared object", func() {
		It("returns no error when shared object is successfully created", func() {
			rt := MockedRoundTripper{ExpectedPath: "/intra-vcp/secret-shares", ExpectedResponseCode: http.StatusCreated}
			HTTPClient := http.Client{Transport: &rt}
			client := Client{HTTPClient: HTTPClient, URL: url.URL{Host: "test", Scheme: "http"}}

			err := client.CreateSecretShare(&share)
			Expect(err).NotTo(HaveOccurred())
		})
		It("returns an error when shared object cannot be created", func() {
			rt := MockedRoundTripper{ExpectedPath: "/another-url"}
			HTTPClient := http.Client{Transport: &rt}
			client := Client{HTTPClient: HTTPClient, URL: url.URL{Host: "test", Scheme: "http"}}

			err := client.CreateSecretShare(&share)
			Expect(err).To(HaveOccurred())
		})
	})
})

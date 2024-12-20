//
// Copyright (c) 2022 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/castor.
//
// SPDX-License-Identifier: Apache-2.0
//

package castor_test

import (
	"encoding/json"
	. "github.com/carbynestack/ephemeral/pkg/utils"
	"github.com/google/uuid"
	"net/http"
	"net/url"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/carbynestack/ephemeral/pkg/castor"
)

var _ = Describe("Castor", func() {

	Context("passing an URL to constructor", func() {
		Context("when url is invalid", func() {
			It("responds with error", func() {
				invalidURL := url.URL{Host: "host:8080", Scheme: "invalidScheme"}
				_, err := NewClient(invalidURL)
				Expect(err).To(HaveOccurred())
			})
		})
		Context("when url is valid", func() {
			It("returns a new client", func() {
				validURL := url.URL{Host: "host:8080", Scheme: "http"}
				client, err := NewClient(validURL)
				Expect(err).NotTo(HaveOccurred())
				Expect(client).NotTo(BeNil())
				Expect(client.HTTPClient).NotTo(BeNil())
				Expect(client.URL).To(Equal(validURL))
			})
		})
	})

	Context("requesting tuples from castor", func() {
		var (
			tupleList *TupleList
			jsn       []byte
			myURL     url.URL
		)
		BeforeEach(func() {
			shares := []Share{Share{Value: "val", Mac: "mac"}}
			tuples := []Tuple{Tuple{Shares: shares}}
			tupleList = &TupleList{Tuples: tuples}
			jsn, _ = json.Marshal(tupleList)
			myURL = url.URL{Host: "host:8080", Scheme: "http"}
		})
		Context("when the path is correct", func() {
			It("returns tuples", func() {
				tbytelist := []byte{1, 2, 1, 2}
				mockedRT := MockedRoundTripper{ExpectedPath: "/intra-vcp/tuples", ReturnJSON: tbytelist, ExpectedResponseCode: http.StatusOK}
				httpClient := &http.Client{Transport: &mockedRT}

				client := Client{URL: myURL, HTTPClient: httpClient}
				tuples, err := client.GetTuples(2, BitGfp, uuid.MustParse("acc23dc8-7855-4a2f-bc89-494ba30a74d2"))

				Expect(tuples).To(Equal(tbytelist))
				Expect(err).NotTo(HaveOccurred())
			})
		})
		Context("when castor returns a non-200 HTTP response code", func() {
			It("returns an error", func() {
				mockedRT := MockedRoundTripper{ExpectedPath: "/wrongpath", ReturnJSON: jsn, ExpectedResponseCode: http.StatusOK}
				httpClient := &http.Client{Transport: &mockedRT}

				client := Client{URL: myURL, HTTPClient: httpClient}
				_, err := client.GetTuples(0, BitGfp, uuid.MustParse("acc23dc8-7855-4a2f-bc89-494ba30a74d2"))

				Expect(checkHTTPError(err.Error(), "getting tuples failed")).To(BeTrue())
			})
		})
		Context("when request to castor fails", func() {
			It("returns an error", func() {
				rt := MockedBrokenRoundTripper{}
				httpClient := &http.Client{Transport: &rt}

				client := Client{URL: myURL, HTTPClient: httpClient}
				_, err := client.GetTuples(0, BitGfp, uuid.MustParse("acc23dc8-7855-4a2f-bc89-494ba30a74d2"))

				Expect(checkHTTPError(err.Error(), "communication with castor failed")).To(BeTrue())
			})
		})

	})

})

func checkHTTPError(actual, expected string) bool {
	return strings.Contains(actual, expected)
}

//
// Copyright (c) 2022 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/castor.
//
// SPDX-License-Identifier: Apache-2.0
//
package castor_test

import (
	"encoding/json"
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
				invalidUrl := url.URL{Host: "host:8080", Scheme: "invalidScheme"}
				_, err := NewCastorClient(invalidUrl)
				Expect(err).To(HaveOccurred())
			})
		})
		Context("when url is valid", func() {
			It("returns a new client", func() {
				validUrl := url.URL{Host: "host:8080", Scheme: "http"}
				client, err := NewCastorClient(validUrl)
				Expect(err).NotTo(HaveOccurred())
				Expect(client).NotTo(BeNil())
			})
		})
	})

	Context("requesting tuples from castor", func() {
		var (
			tupleList TupleList
			js        []byte
			myUrl     url.URL
		)
		BeforeEach(func() {
			tupleList = TupleList{TupleCls: "cls"}
			js, _ = json.Marshal(&tupleList)
			myUrl = url.URL{Host: "host:8080", Scheme: "http"}
		})
		Context("when the path is correct", func() {
			It("returns tuples", func() {

				mockedRT := MockedRoundTripper{ExpectedPath: "/intra-vcp/tuples", ReturnJson: js}
				httpClient := http.Client{Transport: &mockedRT}

				client := Client{Url: myUrl, HttpClient: httpClient}
				tuples, err := client.GetTuples(0, BitGfp, "1")

				Expect(tuples.TupleCls).To(Equal(tupleList.TupleCls))
				Expect(err).NotTo(HaveOccurred())
			})
		})
		Context("when castor returns a non-200 HTTP response code", func() {
			It("returns an error", func() {
				mockedRT := MockedRoundTripper{ExpectedPath: "/wrongpath", ReturnJson: js}
				httpClient := http.Client{Transport: &mockedRT}

				client := Client{Url: myUrl, HttpClient: httpClient}
				_, err := client.GetTuples(0, BitGfp, "1")

				Expect(checkHttpError(err.Error(), "getting tuples failed")).To(BeTrue())
			})
		})
		Context("when request to castor fails", func() {
			It("returns an error", func() {
				rt := MockedBrokenRoundTripper{}
				httpClient := http.Client{Transport: &rt}

				client := Client{Url: myUrl, HttpClient: httpClient}
				_, err := client.GetTuples(0, BitGfp, "1")

				Expect(checkHttpError(err.Error(), "communication with castor failed")).To(BeTrue())
			})
		})
		Context("when castor returns invalid json body", func() {
			It("returns an error", func() {
				js = []byte("invalid JSON String")
				mockedRT := MockedRoundTripper{ExpectedPath: "/intra-vcp/tuples", ReturnJson: js}
				httpClient := http.Client{Transport: &mockedRT}

				client := Client{Url: myUrl, HttpClient: httpClient}
				_, err := client.GetTuples(0, BitGfp, "1")

				Expect(checkHttpError(err.Error(), "castor has returned an invalid response body")).To(BeTrue())
			})
		})

	})

})

func checkHttpError(actual, expected string) bool {
	return strings.Contains(actual, expected)
}

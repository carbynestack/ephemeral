//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package network

import (
	"crypto/tls"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"net"
	"os"
)

const (
	keyFileClient = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC3zPV+S49jvQql
z2PIgJLL1iP9E7VuhA8W6RnFD4DivTBgE71JwktQ2Fm+zSaYnvctNEl5NWYSTktw
Ymx/C2FtPVLU3RWLRvCjut/AXZUJ9J2X7oH8M+LVrkkTI0vHqHqmpQpGk1czuQn3
7qR73w9/+pc0492saClJMBo0jYvGWhbaH4+YuzwYqz0j5A+RFgkVwsxcsVuHBjEJ
f7k+PZ0t6fOC6cgAvo0BedZzGYBusnxwU9xaN+NkofmFBEU5+X4Y0ZSMRZvi8bzs
YbPJjSLu+S/PRDZvvth/KJqsEBpwbMOBIHGy3ilHNq1HDmxAyrszWLVExWD6uPPv
fbQKx6QxAgMBAAECggEAIUowFKXe3LO6n/mGGySecejhL89IBzJIAWBK2JRMRcT6
ZAxvNlLIjWYCKzrBCNeR8VANFrUDPcGMjFhnSkNna/+1ZvR8GHPK1fzc1dydR+ZU
PNZoGKPVK9qbRaoY6ZqsTE6MI+g/3RBgq9U/WWg3SHi8tkmnNrjO8YCS3n3cmRod
enw0KfHb8GaOAhXY3mLF7R9Wfqtl84SaqF5YIElvKSHkNUSKM8qSUrJMhMVe61Ih
o9mBNPCqwks8uwr7sYW9oE7Vj62reAzxKb0qnaIOnow9QKyeIXzLnBhYeKnKkfw/
sn2b02KptYxJ6Do1Ca85JjCWxJaKB/rc9uLIfYEtOQKBgQDe2fzfnuRyEqyB7ShQ
Vwei3GPYuceQ6cjR4QuDLW4Vjorkt/RFLJ8iwFZjZOgw/8QGJ/W/9dyA897IF2ex
G67HGlgrPQIdLsPWZznoSYtkyl1BQHji+To3g34oqso7ihNkXUs2D1TZcuKeYi6F
0fzHtzHfOT3b8VQgu+x4EAtxZwKBgQDTI/GLOPyTtlDkhbb7Tq6XklQ4OB2rEEQY
yRs3Flde2UcL4Lm9I3j1a5ysOl06Mk496DKF2E3CvOTbzR3KwhVrbBxfzyEXqbBa
wF8Kt5eKoMXy2frBid2BFn7/Sj/6VnrOqx2w8RSnbg7L+25jLbWxHU5IdDr8l8J9
2EIY0a3GpwKBgQDA19JDkLQPIqm1JQyluSoafKzKdrmDZUsqk5vqv/1rGhaHJchz
s9FhuR8Ik+F5xVpUGXBH1PIjhOVcMSTB1jrAgMObZwfVSQqfFmS95iaB6bwZIzl4
8EK4l0ks195491sglrrm5Q1/vjLs6/lmQ/iCuryldltZYNR0HyraGshMMQKBgBoV
NqGcSJd2zkdsvU4OSkMvMHhBdmjLeZ4WOeZ0PBbbgItXF5rl5utqf9BG5X1q+X9s
T9F5ByInc54zmJqTn1HF6TtsuwnRTJfpa9RHGdFmSw3VH8UI4vQvc0DWS1EBneop
+WECZyrHzcwlI13dJ7TZifIpaaAKn1wsev3V6UHBAoGAcqkpuxh3Wc8NMVVpSwi1
UYUrhvw5SwbQBW4zuJ1TCmdBTInWIzs6g8OLivKsb7//mnREnzXbMU77dGVAPvtD
7LVVf8g2pQo9Mc6pQ+jSe8XP4myxZs6zyYv/GKSufhVj1HHVEsmTR5z7h4uxJQ43
BCdfT4PYg8e49YT++3T+2bk=
-----END PRIVATE KEY-----
`

	pemFileClient = `-----BEGIN CERTIFICATE-----
MIIC+zCCAeOgAwIBAgIUdBsxMVKucoi0AcgnSQ6NfJ6Rak4wDQYJKoZIhvcNAQEL
BQAwDTELMAkGA1UEAwwCQzAwHhcNMjIwNDEzMTE1OTAzWhcNMzIwNDEwMTE1OTAz
WjANMQswCQYDVQQDDAJDMDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
ALfM9X5Lj2O9CqXPY8iAksvWI/0TtW6EDxbpGcUPgOK9MGATvUnCS1DYWb7NJpie
9y00SXk1ZhJOS3BibH8LYW09UtTdFYtG8KO638BdlQn0nZfugfwz4tWuSRMjS8eo
eqalCkaTVzO5CffupHvfD3/6lzTj3axoKUkwGjSNi8ZaFtofj5i7PBirPSPkD5EW
CRXCzFyxW4cGMQl/uT49nS3p84LpyAC+jQF51nMZgG6yfHBT3Fo342Sh+YUERTn5
fhjRlIxFm+LxvOxhs8mNIu75L89ENm++2H8omqwQGnBsw4EgcbLeKUc2rUcObEDK
uzNYtUTFYPq48+99tArHpDECAwEAAaNTMFEwHQYDVR0OBBYEFDy1mR2JzTRo+F5u
DYYw4cNPpEIPMB8GA1UdIwQYMBaAFDy1mR2JzTRo+F5uDYYw4cNPpEIPMA8GA1Ud
EwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEBAFN1k5YdRHIFbHJSXy+deFxb
chW72ncyv/04YLMwL8oF/YWjxTzp2/g5rFxxM0hD5Z3oP6eQ2XS2FLVxbjXXBrlu
WcLfQZek1Z5QA0KIFPuI7fk30dxfP0BhdeuYue5WIPw16UWv94cSMdMOpz/fs/lD
5wtSAo1OVjSUJCuOXj4k92hJd2tbsxoK1wQgGggBmN3dHqoi86BBtXITIPwUuow/
jcrFmLQ3fEvorWsv2idztv7vXvnmRZSxN+tMsB662gvbgXutkj7pXGGpwQJnS/zn
XredMwJy4AeFFPw0A2nkntEw5Y6FXd1LYNmHUfJ9nyrXe4mjOuLKKf1Kh1Iydxg=
-----END CERTIFICATE-----
`

	keyFileServer = `-----BEGIN PRIVATE KEY-----
MIIEvwIBADANBgkqhkiG9w0BAQEFAASCBKkwggSlAgEAAoIBAQDIQG38Cnp2zMkS
mnUbQiXxRCl0QGbFU76k2dVS00QS/MSAQchnleZoISD2NoxnXQa2zgqY9FprYM2R
pSMI2pAicSyH661cXpGL29j2oU0Ue3zWtPYxf8CXhhOvHd7cOBFInGZcMhv2W7ZU
EhlwHw8Znvg1KSKyQ85IgtOjbAP40HrACJskAd6kGAHZJUVqqPn38qpi1bO7l2fr
W2h9AAsh8A3lLdn1Mod2yCTvN3GJUjROWCnGYrEs//o+j0Ia0KzMtWb9kxLylMgk
HxLUr50Vs5fQ8PO83waTHljqI3ItlecX7W2fLQHwTkhRMmSVw5SyDOJHFpmtWD8p
J/SR+vK7AgMBAAECggEBAJPLQaFgVmwxzkElsEKS+o/rn7DGC1Od8DmY8CG1/SsK
VTjX1EHnV2sI8FvnfI6ZEOiAfz/OMKHJi07wE0BolzJkVtpmLcfboA4aDzJPcCUq
0sNgQcfcotbyRLrdD+t2kgMGM2HeNdcIbzPzO8UNl0Zwln4dwxbQhoHr1Klrgi7y
2TNnecPg6xnIkXjvXZIiUErr0Bo9amqnmzl57oDu4Zs8xNqcwd8PwX62JsVl3Ar9
eML3PSgUuY6BuMCv9PKVLDRSrAKSI2jAb5YT1tw6NrOQfNrL5TSbYV52aCBjsDF+
V7Hcv2c3fmsW28gRVNij+DdNoW3tR6/DSWkpZ8ypNqkCgYEA5iW8rKjehvlaUGlO
SUYmVDI0kLze/LQnEMR4E02hkSKX0keEgw8EcMsOzCZXchU6vZKAdw6zZ+kO9/jc
mOMwcKLShH5FiU3SzUpYr/SX+Ru95+ZB8ACqIrzgMyJUIEf93ezCA/akf3vjQbnz
zajxqcXY9yPDz9GWtjpSYsF99tcCgYEA3r7+AyhSBhEHRoHC9HMjdQE3Kf87Z5zO
ydEPx87/nm/KTKwLLT85uWFRCsYAxzbxZ9vuj8uBnNG1cwkDklWWQm7KuNkg/q+M
3cvwDjlz90CH5kwTMEEXXMkHOQElZCEJvfWEA/iNpTgUl5wU1L5LFpicj5J/OOnu
CjK6svOJOr0CgYEA1uR7lFglV7AyZQy+vWpT1Z//Nvoz1487PsvENnnxFzxOuFhw
4ZK/GbZwPay7T9mEvIezjfdbCvYxNNbY26SekT1nBbGFqhvRbkAyKTFgSYhevM5h
2QA13DOxv+0Y0f+GipZL3jmJBUQfQTqo6+oIo/YJjVGGv2A6sjIoxO9Yd4cCgYA1
uX1Mx6nY+rx1fhDGowq3St7CS2RJnmGl/b2/pKa00SPLEGf1tt02YEmKvq0rX44k
TcChgCU37MDGCTOKVQhT56MPqJcztqXUTT8OPz9AMJlWq5ypM9ntsDMExcj9+JX/
8jqwNn/7jKYy1xuTIH696XtBicUTtiCK5yduyByeRQKBgQCGxnyeMQVraJrj1MMe
px2HHPnZ3intEjCbFXRRSNLbnUH2BkN0iBqAlg17ODwZKRe3RzIt33ily0XLX9eO
nMTahi3D1qvE4mA0fCUIZ1MeCErfVfpkpw6WAGbW/k9dSSPUXvw/8y/k1n+LG4NH
ATB42I5DxjquYktYfgOqMrfe9A==
-----END PRIVATE KEY-----
`

	pemFileServer = `-----BEGIN CERTIFICATE-----
MIIC+zCCAeOgAwIBAgIUZnenCFm9buzi9J+xE0Z/8hAP6j0wDQYJKoZIhvcNAQEL
BQAwDTELMAkGA1UEAwwCUDAwHhcNMjIwNDEzMTE1NzI2WhcNMzIwNDEwMTE1NzI2
WjANMQswCQYDVQQDDAJQMDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AMhAbfwKenbMyRKadRtCJfFEKXRAZsVTvqTZ1VLTRBL8xIBByGeV5mghIPY2jGdd
BrbOCpj0WmtgzZGlIwjakCJxLIfrrVxekYvb2PahTRR7fNa09jF/wJeGE68d3tw4
EUicZlwyG/ZbtlQSGXAfDxme+DUpIrJDzkiC06NsA/jQesAImyQB3qQYAdklRWqo
+ffyqmLVs7uXZ+tbaH0ACyHwDeUt2fUyh3bIJO83cYlSNE5YKcZisSz/+j6PQhrQ
rMy1Zv2TEvKUyCQfEtSvnRWzl9Dw87zfBpMeWOojci2V5xftbZ8tAfBOSFEyZJXD
lLIM4kcWma1YPykn9JH68rsCAwEAAaNTMFEwHQYDVR0OBBYEFFjYZyR+nDmEaUIC
wqZRo2M+/aO4MB8GA1UdIwQYMBaAFFjYZyR+nDmEaUICwqZRo2M+/aO4MA8GA1Ud
EwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEBAHXEnPbWUxzGSJfCC5R2Sjgd
wOB8op5RKqJcarpgg7dfipcwihLPcBitSuVMUKIzsCGlPJk3wRMPz6N0WCEgBFKS
FyPUh+I0ptkXbilmpfTsmbnkb5YbG8BrNLiyWwRKltQbYPQlCdEU50FUigMJy/6T
BWdj1UqyuxVLMcsPQVf8J2/BSmd7I1Xn4+rNGlw3Wg+F5sZ/ETVWNxbU3t9C0M7V
yZchSo1aNHWYFkxsKJVIhSTWwdIzRoaMmdIq0TJ3NCz/pu42tBPqv1DVN9dQkvUP
guY82ncsl79+Nh3HqoE6Tr6tIpuwpuFIjAlsGJrOFQJrx6JrzgQHf+Fcxe3VBEQ=
-----END CERTIFICATE-----
`
)

var _ = Describe("TLSConnector", func() {
	var testDataFolder string
	var certificateFolder string
	var playerID = int32(0)

	BeforeEach(func() {
		var err error
		testDataFolder, err = ioutil.TempDir("", "testData")
		certificateFolder = testDataFolder + "/Player-Data"
		err = os.Mkdir(certificateFolder, os.ModeDir|os.ModePerm)
		if err != nil {
			panic(err)
		}

		err = ioutil.WriteFile(fmt.Sprintf("%s/C%d.pem", certificateFolder, playerID), []byte(pemFileClient), os.ModePerm)
		if err != nil {
			panic(err)
		}

		err = ioutil.WriteFile(fmt.Sprintf("%s/C%d.key", certificateFolder, playerID), []byte(keyFileClient), os.ModePerm)
		if err != nil {
			panic(err)
		}

		err = ioutil.WriteFile(fmt.Sprintf("%s/P%d.pem", certificateFolder, playerID), []byte(pemFileServer), os.ModePerm)
		if err != nil {
			panic(err)
		}

		err = ioutil.WriteFile(fmt.Sprintf("%s/P%d.key", certificateFolder, playerID), []byte(keyFileServer), os.ModePerm)
		if err != nil {
			panic(err)
		}
	})

	AfterEach(func() {
		err := os.RemoveAll(testDataFolder)
		if err != nil {
			panic(err)
		}
	})

	Context("when trying to upgrade to a TLS connection", func() {
		var (
			tlsConnector   func(conn net.Conn, playerID int32) (net.Conn, error)
			client, server net.Conn
		)

		BeforeEach(func() {
			tlsConnector = NewTLSConnectorWithPath(certificateFolder)
			client, server = net.Pipe()
		})

		It("establishes a TLS Connection and allows to send something over the connection", func() {
			// Arrange
			serverPemFileLocation := fmt.Sprintf("%s/P%d.pem", certificateFolder, playerID)
			serverKeyFileLocation := fmt.Sprintf("%s/P%d.key", certificateFolder, playerID)
			serverCertificate, err := tls.LoadX509KeyPair(serverPemFileLocation, serverKeyFileLocation)
			if err != nil {
				panic(err)
			}
			serverConfig := &tls.Config{
				Certificates: []tls.Certificate{serverCertificate},
			}
			serverTLSConnection := tls.Server(server, serverConfig)
			go serverTLSConnection.Handshake()

			// Act
			tlsConnection, err := tlsConnector(client, playerID)
			contentToSend := []byte{byte(1)}
			go tlsConnection.Write(contentToSend)

			contentToReceive := make([]byte, 1)
			serverTLSConnection.Read(contentToReceive)

			// Assert
			Expect(err).NotTo(HaveOccurred())
			Expect(tlsConnection).ToNot(BeNil())
			Expect(contentToReceive).To(Equal(contentToSend))
		})

		Context("and no certificate files for the playerID exist", func() {
			playerID := int32(1)

			It("errors when trying to load the certificate key pair", func() {
				// Act
				tlsConnection, err := tlsConnector(client, playerID)

				// Assert
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("C1.pem"))
				Expect(tlsConnection).To(BeNil())
			})
		})

		Context("and the server does not have the matching certificate", func() {
			playerID := int32(0)
			It("will throw a TLS Error", func() {
				// Arrange
				serverConfig := &tls.Config{
					//No Server Certificates -> Client certificate won't match
					Certificates: []tls.Certificate{},
				}
				serverTLSConnection := tls.Server(server, serverConfig)
				go serverTLSConnection.Handshake()

				// Act
				tlsConnection, err := tlsConnector(client, playerID)

				// Assert
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("remote error: tls: unrecognized name"))
				Expect(tlsConnection).To(BeNil())
			})
		})
	})
})

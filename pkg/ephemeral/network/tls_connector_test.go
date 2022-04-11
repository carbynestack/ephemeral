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
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQCvx2eeVDXG5R+l
GlslnYNHlJgmmkLeXn5MT18qTbq3MCpB6o4rd8I2a1D/uFUht13Ourj7zilKz/5W
jcTnoVG7fiCLcBj3tXvCL5ymOGxxmQeN5siJcefpB8kcSB4RkrON9y6HCpZSIOMv
vfSVMrVMrQj/rjqsO2/Vv1A+4nETJm3GqKfwSikhgNsVcqiHYGkg0d1/3zP8CTAQ
+lp92LijeJAMCyNyHm/A+Wya3g8heRbm6lPtZWUcPOfyn3FGQ+Pu9MbBrQcPbXPW
0sjtGoBweNLYYyns3yViSp7gyOnZWaAwnQtA1T7PPNGkOYp5ehI3gA4bhhCWxbkN
ZVy0qajhAgMBAAECggEAJdsJ4706/6SklggBDS7I8Qd9ZQLf18f95y1Iz3GB/qWu
1BdRmublupaOESR/oQ0+dKEd6YzSs7vriHRrrX6+fWSCWcVAe0hoaL+cOuf34tcU
G2lSUtdnHHaCx0Z4w0wWw0IykP6ktPdENinwnJkZFnRFddrt493BDgVvoLtfosHO
Q+CcX6SmjfS3i0GSsDbI1sBAtH9vP+cCJeXWYtVcPRX9zoX3oYY9zBxuuiarcZku
3mcx22WFi4t30o2jCFwshhjY3W5mxZ3icCZ/mO/BS8FOYk4+BJUQtlxhDvJSjg/u
jCmmFi6WwtceKEhSL6IyiRFLzec60ITlR9U9YB/UqQKBgQDl6sMr/++hzQvOv58c
zoOfBKejHao7Bx9MkFLtQ4KXf4Ypc2uZh/XenziBb+tKRJ5mSXV8NLHs/zrdxPeY
ps0AYkWl9xVR1hKYlnQ75DCbs6zkIEKbKZ1xq5X1TfAmIyHmUcttD5BvQLAeQyG3
+iNo2yFUgg6BywS4E6biL40zkwKBgQDDuF3FW2K5Ms5ntw/o/d55scinx05C74D6
Oy+HesRs6bg77R07fr9Xqgnawqpn2Jk9TRFL5yVJTEHcXH9xMzHgNQ128SGNnDtC
T5/jfalj92hjdmt/gwdGK6PN+IDgb3h3vMnQZszK4zhXP78nte1QGUx2W7TZ7ZrP
C+iulm3iOwKBgQDbkkQqNRYpM6VfIWlXHXJd3xgpkx8LmFWvzPUlWh/RhxwdYfkU
et+4Z96S3suZ9cZAcU8d+0UgzO7u9DhxNHr7Lt7NDRbzPLottyHyQI6bZBBtHNH/
VNLjx7ZCutfp1At/5gWcdgy98s0/WWVOSjie3wcJqdso4TX0hfAOetMiuQKBgDri
C+wla1U2kNypObMqNbW9JBY+IzCGJ/KgvdLvv4rY4iG9W68bmeuA78gOCwCFLM1B
k3OXjiM4OxRWC819zoKa03s2XpbhKv7vP7ZMhxrZQ2GxLfRF8nlNBdIg8n0TbFXx
yXHWi8R6iefN+O+0jzoq8lMlkgqCrrGd7pogDd0jAoGBALK43xm6ZIx5f6Ko94Vk
quXurZhmfbwiU52hBOdej6T+w2axs+mne83/HpcnWNtsmQDPN7vsfnKH/Ny4dG87
G0iQcIEfW6OCGn1N6mr9ch7+2ihszOlKomOBxLurzw3Y7b3z0k9i1+NXeVY9agwF
U5QapxH75EeTq2YKGRjcN100
-----END PRIVATE KEY-----
`
	pemFileClient = `-----BEGIN CERTIFICATE-----
MIIC+zCCAeOgAwIBAgIUWOrYZliAZd4NDKJBNkYsOqSCj5owDQYJKoZIhvcNAQEL
BQAwDTELMAkGA1UEAwwCQzAwHhcNMjIwMTI2MTI0NjQ5WhcNMjIwMjI1MTI0NjQ5
WjANMQswCQYDVQQDDAJDMDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AK/HZ55UNcblH6UaWyWdg0eUmCaaQt5efkxPXypNurcwKkHqjit3wjZrUP+4VSG3
Xc66uPvOKUrP/laNxOehUbt+IItwGPe1e8IvnKY4bHGZB43myIlx5+kHyRxIHhGS
s433LocKllIg4y+99JUytUytCP+uOqw7b9W/UD7icRMmbcaop/BKKSGA2xVyqIdg
aSDR3X/fM/wJMBD6Wn3YuKN4kAwLI3Ieb8D5bJreDyF5FubqU+1lZRw85/KfcUZD
4+70xsGtBw9tc9bSyO0agHB40thjKezfJWJKnuDI6dlZoDCdC0DVPs880aQ5inl6
EjeADhuGEJbFuQ1lXLSpqOECAwEAAaNTMFEwHQYDVR0OBBYEFCXac7qi2TG+j/CQ
fVyvM6W3JfONMB8GA1UdIwQYMBaAFCXac7qi2TG+j/CQfVyvM6W3JfONMA8GA1Ud
EwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEBAE0xk3rMO3xmpq1mwWGGQ/B2
J9Xlqf5qwr63MNz6aIcKrlyk2+OLLaDm8RrF7wFNQ+uvMKKg6bLF7jW7MAX9WMO7
giiT5ySjxddDT0cbSA3HcG3Ria9P6c02VZVt057M1FzXweR/FiJA1Tocn43lXrBT
n2sAiRtO4sxbfhUdIJI1Vh7UUhyAJLe3lVcG/AMMmPG/IedguhMbdalm5/gEaIIc
LjHyQLPWzHQTiUvj+AjpTmCN+3ZbBS/8r4g7XJ7/zvawXxi1Lk9fvSGWGkQLwHJ0
DupEw8GWmc9H0cyY93qtEqKLQPvEDDdvhPoENcf/P6/BD1Z8lMmSMvZ+s6M7VfQ=
-----END CERTIFICATE-----
`

	keyFileServer = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCwdCqmEjiMVBPK
m31IG+I+xwgX+EnEpnrBnlOa0WhFzaMrwqXpijgMA+dLNR8a1zpyhglWBsqm8dpN
7tEV19piizOmxZtZee7h1Hdso/+4U106NqzX5HKwuqZVSOjVN29SFKq0sNricIX1
HabE5LYyBQJtzMzxAZwclb+e7uGBfHJDsOk3hOhs3bkJyV3eRa0uHH2Bu4CPH6L9
bcisFCmHiykZeZaY+BpRkS0c5+h7umLrKSGUe37/vf9UY9niLDUNolHePS/iQnmb
Hv1l/mDl2LvNy5OSCSvOE6L0GMCUDnYmYf6F999LLdQgC7gcZCp3rujZ4MYUsqR2
Nqp46LdbAgMBAAECggEBAJ6ViM8AiTn1RmRNImdwSAHLtwZz6ziFtsXUmacGlQRH
MGLf6WTfCEgkKfd5op7o2Gqc9D8Qk4k+y8hG3jsXZ/owyRcVee0MnRjxbvOA4Q60
PZFYGjdd5YXX+i2j/T3DOJU4ZcNHPzFLl9kX8Q37z5Nc1TYBXh8sJzW5kCIy5xEL
XAKNcwGTZF1ml3jkWkFl3LukS3DP8fF1qDvD987YGuc9oVliYW1F0oKL9VGyS7nB
BtQWslFdP8MbPXG1hjkFydCBiE4teqrFen6hvLdIQk7XJ88Q9UmBoOPJr6+gHuDf
vk33nVGpBVQ1UHFPnDzZyQKtlDBVEUJ8XhqzEkm4asECgYEA5sTgxJt/nJCL9Lh1
61jFbVD21SVFEv7IWIV6YjBxzJhzGVJa6ZhrOnRTrkTAkraJ1wUd9FyIdEsL/Nvy
/z8hOAXbty1zXdpOo/BV0J6zRwJ0Cj8WVTeCUr5KGgw/pzbQdltJ+1J8jnAGbZjN
Ri/QUdryqZTQz3rD8sDVDFvLojMCgYEAw78HQ+y/gL5Z/IJww0lUYHjqcm1G5taY
3Ht6qRvkqdCmW8qC2wpKFKl9lCJfo+H1jidjhM5RTPFSlCxiWtxLAamMvfv0f3d6
q5gPjcjak275bnmU1e0blkLEdeXQljRXH+oDmur95udzh0DrdTDJ/lqbf3uui8Uc
VApAcSbR/jkCgYAWUT/zg55Jw+jlF9m/kuw08DmOz3Xoql8xwGbfjBPVV4D6F+7W
3HiyRIG7Psbo6WJXOxV0hmZj6MYWBCdx6+cIhfiDtI+Nqgkk7Z8+97oaye/y9brx
LtcZrXF5J2oYf8KVT6rN9WI6XDci7j4b5Y/d+rCxGcU/6317wo5YDaCZ5QKBgET3
4qRxHwxKhUQt5XM5PAx9rgVBMXEV/Wf57b71v/yBMow27yIkHvPmwANYlSAV9kHu
6OabFxQoFvN0K/ddlOPyDE/IHV5oB4W8HwbS1QiLWkEtf15cm5K21afAoFy79lKd
TkXgNDOOKytlmVCCLzl6TT1+o4JFofSOZCQ6DFUpAoGAAQdaCjX5UCeWb5e/Vbiu
SQL1RKIHkgm6gj1UjlQ981r6y+hVkBygtIr/eW0wSkFAkUrOdefHNVOQW18ESF06
YqBL4gD7aEij9kGd0PrievimgcYYaBHOcO1RouQOURTMmWqjIPu1fyWDv+rFk+S5
2uCuYndpzOgCiEhjDGCuSug=
-----END PRIVATE KEY-----`

	pemFileServer = `-----BEGIN CERTIFICATE-----
MIIC+zCCAeOgAwIBAgIUNI9WRun2Y+ICmpzjYRpVcJ/BBE4wDQYJKoZIhvcNAQEL
BQAwDTELMAkGA1UEAwwCUDAwHhcNMjIwMTI2MTI0NjQ5WhcNMjIwMjI1MTI0NjQ5
WjANMQswCQYDVQQDDAJQMDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
ALB0KqYSOIxUE8qbfUgb4j7HCBf4ScSmesGeU5rRaEXNoyvCpemKOAwD50s1HxrX
OnKGCVYGyqbx2k3u0RXX2mKLM6bFm1l57uHUd2yj/7hTXTo2rNfkcrC6plVI6NU3
b1IUqrSw2uJwhfUdpsTktjIFAm3MzPEBnByVv57u4YF8ckOw6TeE6GzduQnJXd5F
rS4cfYG7gI8fov1tyKwUKYeLKRl5lpj4GlGRLRzn6Hu6YuspIZR7fv+9/1Rj2eIs
NQ2iUd49L+JCeZse/WX+YOXYu83Lk5IJK84TovQYwJQOdiZh/oX330st1CALuBxk
Kneu6NngxhSypHY2qnjot1sCAwEAAaNTMFEwHQYDVR0OBBYEFDjtm5a7RbAFeYuQ
QfFYci+eTOeXMB8GA1UdIwQYMBaAFDjtm5a7RbAFeYuQQfFYci+eTOeXMA8GA1Ud
EwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEBAGo1n03gEYMsBLLaOcY7dDwn
behhLE7UP3eWRw2gpmbKfilk+dljYWsOdiQeXktE/LxyFiuBNwefI7JrypFifzio
udqYyQAJ2pvMogij+TPajaDhJxmMWqRizcAo/6cXekSCufnRbbTBENUG2ZNHRuyn
zsYFZtpxDO9LF0uutE2P6NJQpKKrCo/NGMV4AF0vy1tKp6h2fBU3K9Yn+1RihIyS
Y+sLoNiorJloqZ8qn2cULbax/xi/IcccdRJfoIjmIuSl9wUwl+lkeGB9Rlwm5iFJ
LO+mQ15hUEpbjrXF3IdY+4MjDqFOETC0KuI72yjUGPZqWe+WAhBcni3VNzs2Ik4=
-----END CERTIFICATE-----`
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
				serverTlsConnection := tls.Server(server, serverConfig)
				go serverTlsConnection.Handshake()

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

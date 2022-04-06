package network

import (
	"crypto/tls"
	"fmt"
	"net"
)

func NewTlsConnector() func(conn net.Conn, playerID int32) (net.Conn, error) {
	return NewTlsConnectorWithPath("Player-Data")
}

func NewTlsConnectorWithPath(folderPath string) func(conn net.Conn, playerID int32) (net.Conn, error) {
	return func(conn net.Conn, playerID int32) (net.Conn, error) {
		tlsConfig, err := getTlsConfig(playerID, folderPath)
		if err != nil {
			return nil, err
		}

		tlsClient := tls.Client(conn, tlsConfig)
		err = tlsClient.Handshake()
		if err != nil {
			return nil, err
		}

		return net.Conn(tlsClient), nil
	}
}

func getTlsConfig(playerID int32, folder string) (*tls.Config, error) {
	certFile := fmt.Sprintf("%s/C%d.pem", folder, playerID)
	keyFile := fmt.Sprintf("%s/C%d.key", folder, playerID)
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}
	return tlsConfig, nil
}

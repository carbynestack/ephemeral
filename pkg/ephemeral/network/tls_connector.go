package network

import (
	"crypto/tls"
	"fmt"
	"net"
)

// NewTLSConnector creates a TLS connector Function in the default Path "Player-Data"
// Simply delegates to NewTLSConnectorWithPath
func NewTLSConnector() func(conn net.Conn, playerID int32) (net.Conn, error) {
	return NewTLSConnectorWithPath("Player-Data")
}

// NewTLSConnectorWithPath creates a new TLS connector Function.
// The function will accept the Socket Connection and the PlayerID and upgrade it to a TLS encrypted one.
// Will search for Certificates in the provided folder Path.
// Certificates must be named in the format that MP-SPDZ uses (<Folder>/C<PlayerID>.pem and .key)
func NewTLSConnectorWithPath(folderPath string) func(conn net.Conn, playerID int32) (net.Conn, error) {
	return func(conn net.Conn, playerID int32) (net.Conn, error) {
		tlsConfig, err := getTLSConfig(playerID, folderPath)
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

// getTLSConfig Loads the TLS Config for the provided PlayerId located in the given folder
// Certificates must be named in the format that MP-SPDZ uses (<Folder>/C<PlayerID>.pem and .key)
func getTLSConfig(playerID int32, folder string) (*tls.Config, error) {
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

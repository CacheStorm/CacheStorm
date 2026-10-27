package server

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"github.com/cachestorm/cachestorm/internal/config"
)

func tlsBase() *tls.Config {
	return &tls.Config{
		MinVersion:       tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
	}
}

func loadCAPool(file string) (*x509.CertPool, error) {
	pemData, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemData) {
		return nil, fmt.Errorf("no certificates in %s", file)
	}
	return pool, nil
}

// serverTLSConfig is the listener's TLS configuration, with client
// certificates checked as tls-auth-clients asks.
func serverTLSConfig(sc *config.ServerConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(sc.TLSCertFile, sc.TLSKeyFile)
	if err != nil {
		return nil, err
	}
	cfg := tlsBase()
	cfg.Certificates = []tls.Certificate{cert}
	mode := sc.TLSAuthClients
	if mode == "" && sc.TLSCACertFile != "" {
		mode = "yes"
	}
	switch mode {
	case "yes", "optional":
		if sc.TLSCACertFile == "" {
			return nil, errors.New("tls_auth_clients needs tls_ca_cert_file")
		}
		pool, err := loadCAPool(sc.TLSCACertFile)
		if err != nil {
			return nil, err
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		if mode == "optional" {
			cfg.ClientAuth = tls.VerifyClientCertIfGiven
		}
	}
	return cfg, nil
}

// replicaTLSConfig is what a replica dials its master with: its own
// certificate when it has one, and the configured CA (or the system roots)
// to verify the master.
func replicaTLSConfig(sc *config.ServerConfig) (*tls.Config, error) {
	cfg := tlsBase()
	if sc.TLSCertFile != "" && sc.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(sc.TLSCertFile, sc.TLSKeyFile)
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	if sc.TLSCACertFile != "" {
		pool, err := loadCAPool(sc.TLSCACertFile)
		if err != nil {
			return nil, err
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

package server

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

type testPKI struct {
	dir                  string
	ca                   string
	serverCert, serverKy string
	clientCert, clientKy string
}

func writePEM(t *testing.T, path, typ string, der []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
}

func newTestPKI(t *testing.T) testPKI {
	t.Helper()
	dir := t.TempDir()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test CA"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, _ := x509.ParseCertificate(caDER)
	p := testPKI{dir: dir, ca: filepath.Join(dir, "ca.pem")}
	writePEM(t, p.ca, "CERTIFICATE", caDER)
	issue := func(name string, serial int64, usage x509.ExtKeyUsage) (string, string) {
		key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name},
			NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage, x509.ExtKeyUsageClientAuth},
			IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, DNSNames: []string{"localhost"},
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
		if err != nil {
			t.Fatal(err)
		}
		keyDER, _ := x509.MarshalECPrivateKey(key)
		cert, keyFile := filepath.Join(dir, name+".pem"), filepath.Join(dir, name+"-key.pem")
		writePEM(t, cert, "CERTIFICATE", der)
		writePEM(t, keyFile, "EC PRIVATE KEY", keyDER)
		return cert, keyFile
	}
	p.serverCert, p.serverKy = issue("server", 2, x509.ExtKeyUsageServerAuth)
	p.clientCert, p.clientKy = issue("client", 3, x509.ExtKeyUsageClientAuth)
	return p
}

func (p testPKI) clientTLS(t *testing.T, withCert bool) *tls.Config {
	t.Helper()
	pool, err := loadCAPool(p.ca)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &tls.Config{RootCAs: pool, ServerName: "127.0.0.1"}
	if withCert {
		cert, err := tls.LoadX509KeyPair(p.clientCert, p.clientKy)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg
}

func tlsPing(port int, cfg *tls.Config) error {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 2 * time.Second}, "tcp", "127.0.0.1:"+strconv.Itoa(port), cfg)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(encodeArgs("PING")); err != nil {
		return err
	}
	v, err := resp.NewReader(bufio.NewReader(conn)).ReadValue()
	if err != nil {
		return err
	}
	if v.Str != "PONG" {
		return &net.AddrError{Err: "unexpected reply " + v.Str}
	}
	return nil
}

func TestTLSAuthClients(t *testing.T) {
	p := newTestPKI(t)
	for _, tc := range []struct {
		mode              string
		withCert, without bool
	}{
		{"yes", true, false},
		{"optional", true, true},
		{"no", true, true},
	} {
		s := startReplServer(t, func(cfg *config.Config) {
			cfg.Server.TLSCertFile, cfg.Server.TLSKeyFile = p.serverCert, p.serverKy
			cfg.Server.TLSCACertFile, cfg.Server.TLSAuthClients = p.ca, tc.mode
		})
		if err := tlsPing(s.testPort(), p.clientTLS(t, true)); (err == nil) != tc.withCert {
			t.Fatalf("tls_auth_clients %s with a client certificate: %v", tc.mode, err)
		}
		if err := tlsPing(s.testPort(), p.clientTLS(t, false)); (err == nil) != tc.without {
			t.Fatalf("tls_auth_clients %s without a client certificate: %v", tc.mode, err)
		}
	}
}

func TestReplicationOverTLS(t *testing.T) {
	p := newTestPKI(t)
	tlsCfg := func(cfg *config.Config) {
		cfg.Server.TLSCertFile, cfg.Server.TLSKeyFile = p.serverCert, p.serverKy
		cfg.Server.TLSCACertFile = p.ca
	}
	master := startReplServer(t, tlsCfg)
	replica := startReplServer(t, func(cfg *config.Config) {
		tlsCfg(cfg)
		cfg.Replication.Role = "replica"
		cfg.Replication.MasterHost = "127.0.0.1"
		cfg.Replication.MasterPort = master.testPort()
		cfg.Replication.TLS = true
	})
	conn, err := tls.Dial("tcp", "127.0.0.1:"+strconv.Itoa(master.testPort()), p.clientTLS(t, true))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c := &replClient{t: t, conn: conn, rd: resp.NewReader(bufio.NewReader(conn))}
	if got := c.do("SET", "secure", "yes"); got.Str != "OK" {
		t.Fatalf("SET = %+v", got)
	}
	eventually(t, 10*time.Second, "the replica received the write over TLS", func() bool {
		entry, ok := replica.store.Get("secure")
		return ok && entry != nil
	})
}

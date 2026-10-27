package server

import (
	"bufio"
	"crypto/tls"
	"strconv"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

func startTLSBusNode(t *testing.T, p testPKI) *busNode {
	t.Helper()
	s := startReplServerCfg(t, clusterTestConfig(t, t.TempDir(), func(cfg *config.Config) {
		cfg.Cluster.NodeTimeout = busTestTimeout
		cfg.Cluster.TLS = true
		cfg.Server.TLSCertFile, cfg.Server.TLSKeyFile, cfg.Server.TLSCACertFile = p.serverCert, p.serverKy, p.ca
	}))
	conn, err := tls.Dial("tcp", "127.0.0.1:"+strconv.Itoa(s.testPort()), p.clientTLS(t, true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	c := &replClient{t: t, conn: conn, rd: resp.NewReader(bufio.NewReader(conn))}
	return &busNode{t: t, s: s, c: c, id: c.str("CLUSTER", "MYID"), port: s.testPort(), bus: s.testBusPort()}
}

func TestClusterOverTLS(t *testing.T) {
	p := newTestPKI(t)
	nodes := []*busNode{startTLSBusNode(t, p), startTLSBusNode(t, p), startTLSBusNode(t, p)}
	formMesh(t, nodes)
	assignThirds(t, nodes)

	replica := startTLSBusNode(t, p)
	replica.meet(nodes[0])
	eventually(t, 10*time.Second, "the replica to join", func() bool { return len(replica.nodes()) == 4 })
	if got := replica.c.str("CLUSTER", "REPLICATE", nodes[0].id); got != "OK" {
		t.Fatalf("CLUSTER REPLICATE = %s", got)
	}
	if got := nodes[0].c.str("SET", "{bar}x", "1"); got != "OK" {
		t.Fatalf("SET = %s", got)
	}
	replica.c.str("READONLY")
	eventually(t, 10*time.Second, "the replica to sync over TLS", func() bool {
		return replica.c.str("GET", "{bar}x") == "1"
	})

	slot := nodes[0].c.do("CLUSTER", "KEYSLOT", "{bar}x").Int
	nodes[1].c.str("CLUSTER", "SETSLOT", strconv.FormatInt(slot, 10), "IMPORTING", nodes[0].id)
	nodes[0].c.str("CLUSTER", "SETSLOT", strconv.FormatInt(slot, 10), "MIGRATING", nodes[1].id)
	if got := nodes[0].c.str("MIGRATE", "127.0.0.1", strconv.Itoa(nodes[1].port), "", "0", "5000", "KEYS", "{bar}x"); got != "OK" {
		t.Fatalf("MIGRATE over TLS = %s", got)
	}
}

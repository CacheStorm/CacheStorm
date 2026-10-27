package server

import (
	"strconv"
	"testing"
	"time"
)

func TestFailoverCommand(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	mc := dialRepl(t, master)
	if got := mc.str("FAILOVER"); got != "ERR:ERR FAILOVER requires connected replicas." {
		t.Fatalf("FAILOVER without replicas = %s", got)
	}
	mc.str("SET", "before", "1")
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	rport := strconv.Itoa(replica.testPort())

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"FAILOVER", "ABORT"}, "ERR:ERR No failover in progress."},
		{[]string{"FAILOVER", "FORCE"}, "ERR:ERR FAILOVER with force option requires both a timeout and target HOST and IP."},
		{[]string{"FAILOVER", "TIMEOUT", "0"}, "ERR:ERR FAILOVER timeout must be greater than 0"},
		{[]string{"FAILOVER", "TO", "10.9.9.9", "1"}, "ERR:ERR FAILOVER target HOST and PORT is not a replica."},
		{[]string{"FAILOVER", "BOGUS"}, "ERR:ERR syntax error"},
	} {
		if got := mc.str(tc.args...); got != tc.want {
			t.Fatalf("%v = %s, want %s", tc.args, got, tc.want)
		}
	}
	if got := rc.str("FAILOVER"); got != "ERR:ERR FAILOVER is not valid when server is a replica." {
		t.Fatalf("FAILOVER on the replica = %s", got)
	}

	if got := mc.str("FAILOVER", "TO", "127.0.0.1", rport, "TIMEOUT", "5000"); got != "OK" {
		t.Fatalf("FAILOVER = %s", got)
	}
	eventually(t, 10*time.Second, "the roles to swap", func() bool {
		mi, ri := mc.info("replication"), rc.info("replication")
		return ri["role"] == "master" && mi["role"] == "slave" && mi["master_port"] == rport &&
			mi["master_link_status"] == "up" && mi["master_failover_state"] == "no-failover"
	})
	if got := rc.str("SET", "after", "2"); got != "OK" {
		t.Fatalf("SET on the new master = %s", got)
	}
	eventually(t, 5*time.Second, "the old master to follow", func() bool { return mc.str("GET", "after") == "2" })
	if got := mc.str("GET", "before"); got != "1" {
		t.Fatalf("GET before on the old master = %s", got)
	}
	if got := mc.str("SET", "x", "1"); got != "ERR:READONLY You can't write against a read only replica." {
		t.Fatalf("SET on the old master = %s", got)
	}

	mport := strconv.Itoa(master.testPort())
	eventually(t, 5*time.Second, "the old master to be an online replica", func() bool {
		return rc.info("replication")["connected_slaves"] == "1"
	})
	if got := rc.str("FAILOVER"); got != "OK" {
		t.Fatalf("FAILOVER back = %s", got)
	}
	eventually(t, 10*time.Second, "the roles to swap back", func() bool {
		mi, ri := mc.info("replication"), rc.info("replication")
		return mi["role"] == "master" && ri["role"] == "slave" && ri["master_port"] == mport && ri["master_link_status"] == "up"
	})
	if got := mc.str("GET", "after"); got != "2" {
		t.Fatalf("GET after the second failover = %s", got)
	}
}

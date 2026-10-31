package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseRedisArgs(t *testing.T) {
	ds, err := ParseRedisArgs([]string{"--port", "7000", "--save", "60", "1000", "--save", "", "--appendonly", "yes"})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(ds))
	for i, d := range ds {
		got[i] = d.Name + "=" + strings.Join(d.Args, ",")
	}
	if want := []string{"port=7000", "save=60,1000", "save=", "appendonly=yes"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("directives %v, want %v", got, want)
	}
	if _, err := ParseRedisArgs([]string{"7000"}); err == nil {
		t.Fatal("a value without an option was accepted")
	}
}

func TestParseRedisConf(t *testing.T) {
	ds, err := ParseRedisConf("# comment\n\nbind 127.0.0.1 -::1\nlogfile \"\"\nproc-title-template \"{title} {listen-addr}\"\n", "redis.conf")
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 3 || ds[0].Name != "bind" || !reflect.DeepEqual(ds[1].Args, []string{""}) || ds[2].Args[0] != "{title} {listen-addr}" || ds[2].Source != "redis.conf:5" {
		t.Fatalf("directives %+v", ds)
	}
	if _, err := ParseRedisConf("save \"60 1\n", "bad.conf"); err == nil {
		t.Fatal("an unbalanced quote was accepted")
	}
}

func TestApplyDirectives(t *testing.T) {
	c := Default()
	rest, err := c.ApplyDirectives([]Directive{
		{Name: "port", Args: []string{"7001"}},
		{Name: "bind", Args: []string{"*", "-::*"}},
		{Name: "requirepass", Args: []string{"pw"}},
		{Name: "maxmemory", Args: []string{"1gb"}},
		{Name: "appendonly", Args: []string{"yes"}},
		{Name: "save", Args: []string{"900", "1"}},
		{Name: "save", Args: []string{"300 10"}},
		{Name: "dir", Args: []string{"/data"}},
		{Name: "slaveof", Args: []string{"10.0.0.1", "6379"}},
		{Name: "loglevel", Args: []string{"warning"}},
		{Name: "cluster-enabled", Args: []string{"yes"}},
		{Name: "hash-max-listpack-entries", Args: []string{"256"}},
		{Name: "user", Args: []string{"app", "on", ">pw", "~*", "+@all"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Port != 7001 || c.Server.Bind != "0.0.0.0" || c.Server.RequirePass != "pw" || c.Memory.MaxMemory != "1gb" {
		t.Fatalf("server and memory settings: %+v %+v", c.Server, c.Memory)
	}
	if !c.Persistence.Enabled || !c.Persistence.AOF || c.Persistence.SnapshotInterval != "300s" || c.Persistence.DataDir != "/data" {
		t.Fatalf("persistence settings: %+v", c.Persistence)
	}
	if !c.Replication.IsReplica() || c.Replication.MasterHost != "10.0.0.1" || c.Replication.MasterPort != 6379 {
		t.Fatalf("replication settings: %+v", c.Replication)
	}
	if c.Logging.Level != "warn" || !c.Cluster.Enabled {
		t.Fatalf("logging %q, cluster %v", c.Logging.Level, c.Cluster.Enabled)
	}
	if len(rest) != 2 || rest[0].Name != "hash-max-listpack-entries" || rest[1].Name != "user" {
		t.Fatalf("directives left for run time: %+v", rest)
	}

	c = Default()
	if _, err := c.ApplyDirectives([]Directive{{Name: "save", Args: []string{"60", "1"}}, {Name: "save", Args: []string{""}}}); err != nil {
		t.Fatal(err)
	}
	if c.Persistence.Enabled || c.Persistence.SnapshotInterval != "0" {
		t.Fatalf("an empty save rule kept snapshots on: %+v", c.Persistence)
	}

	for _, bad := range []Directive{
		{Name: "port", Args: []string{"x"}},
		{Name: "appendonly", Args: []string{"maybe"}},
		{Name: "daemonize", Args: []string{"yes"}},
		{Name: "dbfilename", Args: []string{"other.rdb"}},
		{Name: "rename-command", Args: []string{"FLUSHALL", ""}},
		{Name: "replicaof", Args: []string{"host"}},
	} {
		if _, err := Default().ApplyDirectives([]Directive{bad}); err == nil {
			t.Errorf("%s %v was accepted", bad.Name, bad.Args)
		}
	}
}

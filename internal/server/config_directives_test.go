package server

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

func TestApplyConfigDirectives(t *testing.T) {
	s := startReplServer(t, nil)
	err := s.ApplyConfigDirectives([]config.Directive{
		{Name: "hash-max-listpack-entries", Args: []string{"256"}, Source: "t:1"},
		{Name: "client-output-buffer-limit", Args: []string{"pubsub", "16mb", "4mb", "30"}, Source: "t:2"},
		{Name: "tcp-backlog", Args: []string{"511"}, Source: "t:3"},
		{Name: "user", Args: []string{"app", "on", ">secret", "~app:*", "+get"}, Source: "t:4"},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := dialRepl(t, s)
	if got := c.str("CONFIG", "GET", "hash-max-listpack-entries"); got != "[hash-max-listpack-entries 256]" {
		t.Fatalf("CONFIG GET hash-max-listpack-entries = %s", got)
	}
	if got := c.str("CONFIG", "GET", "client-output-buffer-limit"); !strings.Contains(got, "pubsub 16777216 4194304 30") {
		t.Fatalf("CONFIG GET client-output-buffer-limit = %s", got)
	}
	if got := c.str("AUTH", "app", "secret"); got != "OK" {
		t.Fatalf("AUTH as the user from the user directive: %s", got)
	}

	err = s.ApplyConfigDirectives([]config.Directive{{Name: "no-such-directive", Args: []string{"1"}, Source: "t:9"}})
	if err == nil || !strings.Contains(err.Error(), "t:9: no-such-directive") {
		t.Fatalf("an unknown directive: %v", err)
	}
	err = s.ApplyConfigDirectives([]config.Directive{{Name: "hash-max-listpack-entries", Args: []string{"x"}, Source: "t:10"}})
	if err == nil {
		t.Fatal("an invalid value was accepted")
	}
}

func TestConfigSetSaveChangesTheSnapshotInterval(t *testing.T) {
	dir := t.TempDir()
	s := startReplServer(t, func(cfg *config.Config) {
		cfg.Persistence.Enabled = true
		cfg.Persistence.AOF = false
		cfg.Persistence.DataDir = dir
		cfg.Persistence.SnapshotInterval = "300s"
	})
	c := dialRepl(t, s)
	if got := c.str("CONFIG", "GET", "save"); got != "[save 300 1]" {
		t.Fatalf("CONFIG GET save = %s", got)
	}
	if got := c.str("CONFIG", "SET", "save", "3600 1 1 1"); got != "OK" {
		t.Fatalf("CONFIG SET save: %s", got)
	}
	if got := c.str("CONFIG", "GET", "save"); got != "[save 3600 1 1 1]" {
		t.Fatalf("CONFIG GET save after CONFIG SET = %s", got)
	}
	c.str("SET", "k", "v")
	deadline := time.Now().Add(5 * time.Second)
	for c.do("LASTSAVE").Int == 0 || !fileExists(filepath.Join(dir, "dump.rdb")) {
		if time.Now().After(deadline) {
			t.Fatal("no snapshot within 5s of save 1 1")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got := c.str("CONFIG", "SET", "save", ""); got != "OK" {
		t.Fatalf("CONFIG SET save \"\": %s", got)
	}
	if got := c.str("CONFIG", "GET", "save"); got != "[save ]" {
		t.Fatalf("CONFIG GET save after clearing = %s", got)
	}
	if got := c.str("CONFIG", "SET", "save", "60"); !strings.Contains(got, "Invalid save parameters") {
		t.Fatalf("CONFIG SET save 60: %s", got)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestSaveRulesNeedChangesAndTime(t *testing.T) {
	s := &Server{}
	rules, err := parseSaveRules("3600 1 60 10000")
	if err != nil {
		t.Fatal(err)
	}
	s.saveRules.Store(&rules)
	now := time.Now()
	s.snapshotLastAt = now.Add(-2 * time.Minute)
	if s.saveDue(now, 9999) || !s.saveDue(now, 10000) {
		t.Fatal("the 60 10000 rule must wait for 10000 changes")
	}
	s.snapshotLastAt = now.Add(-2 * time.Hour)
	if s.saveDue(now, 0) || !s.saveDue(now, 1) {
		t.Fatal("the 3600 1 rule must fire after an hour with one change")
	}
	s.snapshotLastError, s.snapshotStarted = errors.New("disk full"), now.Add(-time.Second)
	if s.saveDue(now, 1) {
		t.Fatal("a failed save must block the next one for 5 seconds")
	}
	for _, bad := range []string{"60", "0 1", "60 -1", "x 1"} {
		if _, err := parseSaveRules(bad); err == nil {
			t.Fatalf("parseSaveRules(%q) accepted", bad)
		}
	}
}

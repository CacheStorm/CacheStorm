package server

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/resp"
)

var (
	stressKeys = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}
	stressTags = []string{"t0", "t1", "t2", "t3", "t4", "t5"}
)

func stressCommand(rng *rand.Rand) []string {
	key := stressKeys[rng.Intn(len(stressKeys))]
	other := stressKeys[rng.Intn(len(stressKeys))]
	tag := func() string { return stressTags[rng.Intn(len(stressTags))] }
	switch rng.Intn(20) {
	case 0, 1, 2:
		return []string{"SET", key, strconv.Itoa(rng.Intn(9)), "TAGS", tag(), tag()}
	case 3:
		return []string{"SETTAG", key, "v", tag()}
	case 4, 5:
		return []string{"ADDTAG", key, tag(), tag()}
	case 6:
		return []string{"REMTAG", key, tag()}
	case 7, 8:
		return []string{"INVALIDATE", tag()}
	case 9:
		return []string{"INVALIDATE", tag(), tag(), "CASCADE"}
	case 10:
		return []string{"TAGLINK", tag(), tag()}
	case 11:
		return []string{"TAGUNLINK", tag(), tag()}
	case 12:
		return []string{"RENAME", key, other}
	case 13:
		return []string{"COPY", key, other, "REPLACE"}
	case 14:
		return []string{"MOVE", key, "1"}
	case 15:
		return []string{"PEXPIRE", key, "5"}
	case 16:
		return []string{"INCR", key}
	case 17:
		return []string{"DEL", key}
	case 18:
		return []string{"SET", key, "plain"}
	}
	return []string{"RPUSH", key, "x"}
}

// tagIndexState checks that, in each database, every key's tags and every
// tag's keys agree, and returns the state for comparing servers.
func tagIndexState(t *testing.T, c *replClient, who string) string {
	t.Helper()
	var b strings.Builder
	total := int64(0)
	for db := 0; db < 2; db++ {
		c.str("SELECT", strconv.Itoa(db))
		byTag := map[string][]string{}
		for _, k := range stressKeys {
			if c.do("EXISTS", k).Int == 0 {
				if tags := c.do("TAGS", k); len(tags.Array) != 0 {
					t.Fatalf("%s db%d: missing key %s has tags %s", who, db, k, str(tags))
				}
				continue
			}
			var tags []string
			for _, v := range c.do("TAGS", k).Array {
				tags = append(tags, string(v.Bulk))
				byTag[string(v.Bulk)] = append(byTag[string(v.Bulk)], k)
			}
			sort.Strings(tags)
			fmt.Fprintf(&b, "db%d %s=%s[%s] ", db, k, c.str("TYPE", k), strings.Join(tags, ","))
		}
		for _, tag := range stressTags {
			var keys []string
			for _, v := range c.do("TAGKEYS", tag).Array {
				keys = append(keys, string(v.Bulk))
			}
			sort.Strings(keys)
			want := byTag[tag]
			sort.Strings(want)
			if strings.Join(keys, ",") != strings.Join(want, ",") {
				t.Fatalf("%s db%d: TAGKEYS %s = %v, keys with the tag %v", who, db, tag, keys, want)
			}
			if n := c.do("TAGCOUNT", tag).Int; n != int64(len(want)) {
				t.Fatalf("%s db%d: TAGCOUNT %s = %d, want %d", who, db, tag, n, len(want))
			}
			total += int64(len(want))
			fmt.Fprintf(&b, "db%d %s>%s ", db, tag, str(c.do("TAGCHILDREN", tag)))
		}
	}
	c.str("SELECT", "0")
	if m := c.info("tags")["tag_memberships"]; m != strconv.FormatInt(total, 10) {
		t.Fatalf("%s: tag_memberships %s, the tags list %d", who, m, total)
	}
	return b.String()
}

// Clients writing, retagging, moving and invalidating the same keys at once
// leave a tag index that agrees with the keys, on the master, its replica
// and after an AOF restart.
func TestConcurrentTagCommandsKeepTheIndexConsistentX(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), func(c *config.Config) { c.Persistence.AOFRewritePercentage = 0 })
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	replicate(t, replica, master)

	const clients, rounds = 8, 60
	var wg sync.WaitGroup
	errs := make(chan error, clients)
	for id := 0; id < clients; id++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(id)))
			c := dialRepl(t, master)
			for r := 0; r < rounds; r++ {
				// Pipelines of commands, sometimes in a transaction and
				// sometimes in the other database.
				var cmds [][]string
				if rng.Intn(4) == 0 {
					cmds = append(cmds, []string{"SELECT", "1"})
				}
				multi := rng.Intn(5) == 0
				if multi {
					cmds = append(cmds, []string{"MULTI"})
				}
				for i := 0; i < 8; i++ {
					cmds = append(cmds, stressCommand(rng))
				}
				if multi {
					cmds = append(cmds, []string{"EXEC"})
				}
				cmds = append(cmds, []string{"SELECT", "0"})
				if rng.Intn(30) == 0 {
					cmds = append(cmds, []string{"SWAPDB", "0", "1"})
				}
				var buf []byte
				for _, cmd := range cmds {
					buf = append(buf, encodeArgs(cmd...)...)
				}
				_ = c.conn.SetDeadline(time.Now().Add(20 * time.Second))
				if _, err := c.conn.Write(buf); err != nil {
					errs <- err
					return
				}
				for range cmds {
					if _, err := c.rd.ReadValue(); err != nil {
						errs <- err
						return
					}
				}
			}
		}(id)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	// Let the short expiries pass and be noticed.
	time.Sleep(50 * time.Millisecond)
	c := dialRepl(t, master)
	for db := 0; db < 2; db++ {
		c.str("SELECT", strconv.Itoa(db))
		for _, k := range stressKeys {
			c.do("EXISTS", k)
		}
	}
	c.str("SELECT", "0")

	want := tagIndexState(t, c, "master")
	waitSynced(t, replica, master)
	if got := tagIndexState(t, dialRepl(t, replica), "replica"); got != want {
		t.Fatalf("replica differs\nmaster:  %s\nreplica: %s", want, got)
	}
	stopReplServer(master)
	if s, err := newOwnedTestServer(t, cfg); err != nil {
		logAOFTail(t, filepath.Join(cfg.Persistence.DataDir, "appendonly.aof"), err)
		t.Fatal(err)
	} else {
		stopReplServer(s)
	}
	restarted := startReplServerCfg(t, cfg)
	if got := tagIndexState(t, dialRepl(t, restarted), "restarted"); got != want {
		t.Fatalf("after an AOF restart\nbefore: %s\nafter:  %s", want, got)
	}
}

// logAOFTail logs, for a replay that failed at AOF command n, the commands
// before it with the database each ran in, so a failure seen once (in CI)
// can be followed.
func logAOFTail(t *testing.T, path string, replayErr error) {
	t.Helper()
	var n int
	msg := replayErr.Error()
	if i := strings.Index(msg, "AOF command "); i < 0 {
		return
	} else if _, err := fmt.Sscanf(msg[i:], "AOF command %d", &n); err != nil {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	br := bufio.NewReader(f)
	rd := resp.NewReader(br)
	db := "0"
	var lines []string
	for i := 1; i <= n; i++ {
		for {
			if b, _ := br.Peek(1); len(b) == 1 && b[0] == '#' {
				_, _ = br.ReadString('\n')
				continue
			}
			break
		}
		v, err := rd.ReadValue()
		if err != nil {
			break
		}
		args := make([]string, 0, len(v.Array))
		for _, a := range v.Array {
			args = append(args, string(a.Bulk))
		}
		if len(args) > 1 && strings.EqualFold(args[0], "SELECT") {
			db = args[1]
		}
		lines = append(lines, fmt.Sprintf("%5d db%s %s", i, db, strings.Join(args, " ")))
	}
	if len(lines) > 60 {
		lines = lines[len(lines)-60:]
	}
	t.Logf("AOF before the failing command:\n%s", strings.Join(lines, "\n"))
}

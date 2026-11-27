package server

import (
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

// mixedCommand is a write to a hash with field expiry, a set, a sorted
// set, a string, a bitmap, a HyperLogLog, a geo set or a stream with a
// consumer group, or a command moving keys between them. The keys of one
// type are now and then renamed, copied or overwritten into another, so
// some commands meet the wrong type.
func mixedCommand(rng *rand.Rand) []string {
	n := strconv.Itoa(rng.Intn(1000))
	i := strconv.Itoa(rng.Intn(3))
	j := strconv.Itoa(rng.Intn(3))
	field := "f" + strconv.Itoa(rng.Intn(6))
	ms := strconv.Itoa(1 + rng.Intn(6))
	switch rng.Intn(56) {
	// Hashes and their fields' expiry.
	case 0, 1:
		return []string{"HSET", "h" + i, field, n, "f" + strconv.Itoa(rng.Intn(6)), n}
	case 2:
		return []string{"HPEXPIRE", "h" + i, ms, "FIELDS", "2", field, "f" + strconv.Itoa(rng.Intn(6))}
	case 3:
		return []string{"HEXPIRE", "h" + i, "100", "FIELDS", "2", field, "f" + strconv.Itoa(rng.Intn(6))}
	case 4:
		return []string{"HPERSIST", "h" + i, "FIELDS", "1", field}
	case 5:
		return []string{"HGETDEL", "h" + i, "FIELDS", "1", field}
	case 6:
		if rng.Intn(2) == 0 {
			return []string{"HGETEX", "h" + i, "EX", "100", "FIELDS", "1", field}
		}
		return []string{"HGETEX", "h" + i, "PX", ms, "FIELDS", "1", field}
	case 7:
		return []string{"HSETEX", "h" + i, "PX", ms, "FIELDS", "1", field, n}
	case 8:
		return []string{"HINCRBY", "h" + i, field, "1"}
	case 9:
		return []string{"HDEL", "h" + i, field}
	// Sets.
	case 10, 11:
		return []string{"SADD", "s" + i, "m" + strconv.Itoa(rng.Intn(10)), "m" + strconv.Itoa(rng.Intn(10))}
	case 12:
		return []string{"SREM", "s" + i, "m" + strconv.Itoa(rng.Intn(10))}
	case 13:
		return []string{"SPOP", "s" + i, strconv.Itoa(1 + rng.Intn(2))}
	case 14:
		return []string{"SMOVE", "s" + i, "s" + j, "m" + strconv.Itoa(rng.Intn(10))}
	case 15:
		return []string{"SINTERSTORE", "s" + i, "s" + j, "s" + strconv.Itoa(rng.Intn(3))}
	case 16:
		return []string{"SDIFFSTORE", "s" + i, "s" + j}
	// Sorted sets.
	case 17, 18:
		return []string{"ZADD", "z" + i, strconv.Itoa(rng.Intn(20)), "m" + strconv.Itoa(rng.Intn(10))}
	case 19:
		return []string{"ZINCRBY", "z" + i, "1.5", "m" + strconv.Itoa(rng.Intn(10))}
	case 20:
		return []string{"ZREMRANGEBYSCORE", "z" + i, "5", "8"}
	case 21:
		return []string{"ZUNIONSTORE", "z" + i, "2", "z" + j, "s" + i, "WEIGHTS", "2", "1"}
	case 22:
		return []string{"ZRANGESTORE", "z" + i, "z" + j, "0", "3"}
	case 23:
		return []string{"ZPOPMIN", "z" + i}
	case 24:
		return []string{"ZADD", "z" + i, "GT", "CH", "10", "m" + strconv.Itoa(rng.Intn(10))}
	// Strings, bitmaps and HyperLogLogs.
	case 25:
		return []string{"APPEND", "t" + i, n}
	case 26:
		return []string{"SETRANGE", "t" + i, strconv.Itoa(rng.Intn(8)), "xy"}
	case 27:
		return []string{"INCRBYFLOAT", "c" + i, "0.25"}
	case 28:
		return []string{"GETEX", "t" + i, "PX", ms}
	case 29:
		return []string{"SET", "t" + i, n, "PX", strconv.Itoa(1 + rng.Intn(20)), "GET"}
	case 30:
		return []string{"SETBIT", "b" + i, strconv.Itoa(rng.Intn(64)), strconv.Itoa(rng.Intn(2))}
	case 31:
		return []string{"BITFIELD", "b" + i, "INCRBY", "u8", strconv.Itoa(8 * rng.Intn(4)), "7", "OVERFLOW", "SAT", "INCRBY", "i4", "0", "3"}
	case 32:
		return []string{"BITOP", "XOR", "b" + i, "b" + j, "b" + strconv.Itoa(rng.Intn(3))}
	case 33:
		return []string{"PFADD", "p" + i, "e" + n, "e" + strconv.Itoa(rng.Intn(1000))}
	case 34:
		return []string{"PFMERGE", "p" + i, "p" + j}
	case 35:
		return []string{"PFCOUNT", "p" + i}
	// Geo sets.
	case 36:
		return []string{"GEOADD", "g" + i, strconv.Itoa(rng.Intn(10)), strconv.Itoa(rng.Intn(10)), "m" + strconv.Itoa(rng.Intn(10))}
	case 37:
		return []string{"GEOSEARCHSTORE", "g" + i, "g" + j, "FROMLONLAT", "5", "5", "BYRADIUS", "400", "km", "ASC", "COUNT", "3", "STOREDIST"}
	// A stream and its consumer group.
	case 38, 39:
		return []string{"XADD", "x0", "MAXLEN", "~", "30", "*", "f", n}
	case 40:
		return []string{"XGROUP", "CREATE", "x0", "g", "0", "MKSTREAM"}
	case 41:
		return []string{"XREADGROUP", "GROUP", "g", "c" + i, "COUNT", "2", "STREAMS", "x0", ">"}
	case 42:
		return []string{"XAUTOCLAIM", "x0", "g", "c" + i, "0", "0-0", "COUNT", "2"}
	case 43:
		return []string{"XTRIM", "x0", "MAXLEN", "10"}
	case 44:
		return []string{"XGROUP", "SETID", "x0", "g", "0"}
	case 45:
		return []string{"XGROUP", "DELCONSUMER", "x0", "g", "c" + i}
	case 46:
		return []string{"EVAL", "local r = redis.call('XREADGROUP', 'GROUP', 'g', 'c9', 'COUNT', 1, 'STREAMS', KEYS[1], '>') if r then redis.call('XACK', KEYS[1], 'g', r[1][2][1][1]) end return 1", "1", "x0"}
	// Moving keys around, between types too.
	case 47:
		return []string{"COPY", "h" + i, "h" + j, "REPLACE"}
	case 48:
		return []string{"RENAME", "z" + i, "z" + j}
	case 49:
		return []string{"MOVE", "s" + i, "1"}
	case 50:
		return []string{"SORT", "s" + i, "ALPHA", "STORE", "l" + i}
	case 51:
		return []string{"PEXPIRE", []string{"h", "s", "z", "t", "b", "p"}[rng.Intn(6)] + i, ms}
	case 52:
		return []string{"RENAME", "t" + i, "s" + j}
	case 53:
		return []string{"DEL", "h" + i, "z" + j}
	case 54:
		return []string{"EVAL", "redis.call('HSET', KEYS[1], 'f0', ARGV[1]) redis.call('HPEXPIRE', KEYS[1], 2, 'FIELDS', 1, 'f0') return redis.call('ZINCRBY', KEYS[2], 1, 'm')", "2", "h" + i, "z" + j, n}
	}
	return []string{"PERSIST", []string{"h", "s", "z", "t"}[rng.Intn(4)] + i}
}

// fieldTTLState is which fields of which hashes have an expiry, in the
// first two databases, as dumpAll leaves field expiry out.
func fieldTTLState(c *replClient) string {
	var out []string
	for db := 0; db < 2; db++ {
		c.str("SELECT", strconv.Itoa(db))
		for _, k := range c.do("KEYS", "*").Array {
			key := string(k.Bulk)
			if c.str("TYPE", key) != "hash" {
				continue
			}
			fields := c.do("HKEYS", key).Array
			args := []string{"HPTTL", key, "FIELDS", strconv.Itoa(len(fields))}
			for _, f := range fields {
				args = append(args, string(f.Bulk))
			}
			if len(fields) == 0 {
				continue
			}
			ttls := c.do(args...).Array
			for i, f := range fields {
				if i < len(ttls) && ttls[i].Int > 0 {
					out = append(out, strconv.Itoa(db)+"/"+key+"."+string(f.Bulk))
				}
			}
		}
	}
	c.str("SELECT", "0")
	sort.Strings(out)
	return strings.Join(out, " ")
}

// mixedGroupState is the consumer group of x0: its position and each
// consumer's pending entries.
func mixedGroupState(c *replClient) string {
	var b strings.Builder
	for db := 0; db < 2; db++ {
		c.str("SELECT", strconv.Itoa(db))
		if c.str("TYPE", "x0") != "stream" {
			continue
		}
		b.WriteString("db" + strconv.Itoa(db) + " " + str(c.do("XINFO", "GROUPS", "x0")) + " ")
		for _, g := range c.do("XINFO", "GROUPS", "x0").Array {
			if len(g.Array) < 2 {
				continue
			}
			// Each pending entry without its idle time: id, consumer and
			// delivery count.
			for _, e := range c.do("XPENDING", "x0", string(g.Array[1].Bulk), "-", "+", "1000").Array {
				if len(e.Array) == 4 {
					b.WriteString(str(e.Array[0]) + "/" + str(e.Array[1]) + "/" + str(e.Array[3]) + " ")
				}
			}
			b.WriteString("; ")
		}
	}
	c.str("SELECT", "0")
	return dropIdle(b.String())
}

// dropIdle removes the idle times and lag figures XINFO and XPENDING print,
// which run on as the comparison runs, keeping the rest.
func dropIdle(s string) string {
	f := strings.Fields(s)
	for i := range f {
		if i > 0 && (strings.Contains(f[i-1], "idle") || strings.Contains(f[i-1], "inactive")) {
			f[i] = "_"
		}
	}
	return strings.Join(f, " ")
}

// Clients send pipelines of writes to every data type at once, some in
// transactions, scripts, database 1 or followed by a SWAPDB, while short
// key and field expiries run out; a second replica attaches with a full
// sync and the AOF is rewritten meanwhile. Every replica and the AOF end
// with the master's keys, values, field expiries and consumer group.
func TestConcurrentMixedCommandsReplicateX(t *testing.T) {
	cfg := aofRewriteConfig(t, t.TempDir(), func(c *config.Config) { c.Persistence.AOFRewritePercentage = 0 })
	master := startReplServerCfg(t, cfg)
	replica := startReplServer(t, nil)
	replicate(t, replica, master)
	late := startReplServer(t, nil)

	admin := dialRepl(t, master)
	runTagLoad(t, master, 8, 120, mixedCommand, func() {
		time.Sleep(30 * time.Millisecond)
		admin.do("BGREWRITEAOF")
		replicate(t, late, master)
		waitAOFRewrites(t, master, 1)
	})
	// Let the short field expiries run out and be reaped.
	time.Sleep(30 * time.Millisecond)

	mc := dialRepl(t, master)
	_ = fieldTTLState(mc)
	want := dumpAll(t, mc)
	wantTTL := fieldTTLState(mc)
	wantGroup := mixedGroupState(mc)
	for _, r := range []struct {
		name string
		s    *Server
	}{{"replica", replica}, {"late replica", late}} {
		waitSynced(t, r.s, master)
		c := dialRepl(t, r.s)
		if got := dumpAll(t, c); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s keys differ\n%s", r.name, mapDiff(want, got))
		}
		if got := fieldTTLState(c); got != wantTTL {
			t.Fatalf("%s field expiries differ\nmaster: %s\n%s: %s", r.name, wantTTL, r.name, got)
		}
		if got := mixedGroupState(c); got != wantGroup {
			t.Fatalf("%s group differs\nmaster: %s\n%s: %s", r.name, wantGroup, r.name, got)
		}
	}
	if got := mc.do("SAVE"); got.Str != "OK" {
		t.Fatalf("SAVE = %s", str(got))
	}
	restarted := checkAOFRestart(t, master, cfg)
	rc := dialRepl(t, restarted)
	if got := dumpAll(t, rc); !reflect.DeepEqual(got, want) {
		t.Fatalf("keys after an AOF load differ\n%s", mapDiff(want, got))
	}
	if got := fieldTTLState(rc); got != wantTTL {
		t.Fatalf("field expiries after an AOF load\nbefore: %s\nafter:  %s", wantTTL, got)
	}
	if got := mixedGroupState(rc); got != wantGroup {
		t.Fatalf("group after an AOF load\nbefore: %s\nafter:  %s", wantGroup, got)
	}

	// The snapshot SAVE wrote, loaded alone.
	stopReplServer(restarted)
	fromRDB := dialRepl(t, startReplServerCfg(t, snapshotConfig(t, cfg.Persistence.DataDir)))
	if got := dumpAll(t, fromRDB); !reflect.DeepEqual(got, want) {
		t.Fatalf("keys after a snapshot load differ\n%s", mapDiff(want, got))
	}
	if got := fieldTTLState(fromRDB); got != wantTTL {
		t.Fatalf("field expiries after a snapshot load\nbefore: %s\nafter:  %s", wantTTL, got)
	}
	if got := mixedGroupState(fromRDB); got != wantGroup {
		t.Fatalf("group after a snapshot load\nbefore: %s\nafter:  %s", wantGroup, got)
	}
}

// mapDiff lists the keys whose values differ between want and got.
func mapDiff(want, got map[string]string) string {
	var out []string
	for k, v := range want {
		if got[k] != v {
			out = append(out, k+": want "+v+" got "+got[k])
		}
	}
	for k, v := range got {
		if _, ok := want[k]; !ok {
			out = append(out, k+": unexpected "+v)
		}
	}
	sort.Strings(out)
	return strings.Join(out, "\n")
}

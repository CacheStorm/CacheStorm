//go:build compat

package compat

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDumpRestoreInteroperability(t *testing.T) {
	redisAddr, csAddr := startRedis(t), startCacheStorm(t)
	redis := &Target{Name: "redis", Addr: redisAddr, conns: map[int]*Conn{}}
	cs := &Target{Name: "cachestorm", Addr: csAddr, conns: map[int]*Conn{}}
	defer redis.resetConns()
	defer cs.resetConns()
	must := func(t *testing.T, target *Target, args ...string) *Val {
		t.Helper()
		r := target.adminDo(args...)
		if r.Err != "" || r.Val == nil || r.Val.Kind == '-' {
			t.Fatalf("%s %s: %s", target.Name, args[0], r.Display())
		}
		return r.Val
	}
	for _, direction := range []struct{ src, dst *Target }{{redis, cs}, {cs, redis}} {
		for _, tc := range []struct {
			name  string
			setup []string
			read  []string
			norm  Norm
		}{
			{"string-binary", []string{"SET", "k", "a\x00b\r\n,=&"}, []string{"GET", "k"}, NormExact},
			{"string-int8", []string{"SET", "k", "-123"}, []string{"GET", "k"}, NormExact},
			{"string-int16", []string{"SET", "k", "-12345"}, []string{"GET", "k"}, NormExact},
			{"string-int32", []string{"SET", "k", "123456789"}, []string{"GET", "k"}, NormExact},
			{"string-lzf", []string{"SET", "k", strings.Repeat("compress\x00", 10000)}, []string{"GET", "k"}, NormExact},
			{"string-raw32", []string{"SET", "k", strings.Repeat("abcdefghijklmnopqrstuvwxyz", 2600)}, []string{"GET", "k"}, NormExact},
			{"list-packed", []string{"RPUSH", "k", "", "a,\x00", "0", "-1", "4095", "-4096", "32767", "-32768", "8388607", "-8388608", "2147483647", "-2147483648", "9223372036854775807"}, []string{"LRANGE", "k", "0", "-1"}, NormExact},
			{"list-plain", []string{"RPUSH", "k", strings.Repeat("large-node", 1000), "last"}, []string{"LRANGE", "k", "0", "-1"}, NormExact},
			{"hash-packed", []string{"HSET", "k", "f&=\x00", "v&=\x00", "", "", "1", "-2"}, []string{"HGETALL", "k"}, NormSortedPairs},
			{"hash-table", []string{"HSET", "k", "field", strings.Repeat("large", 150), "", ""}, []string{"HGETALL", "k"}, NormSortedPairs},
			{"set-packed", []string{"SADD", "k", "", "a,b\x00", "plain"}, []string{"SMEMBERS", "k"}, NormSorted},
			{"set-intset", []string{"SADD", "k", "-9223372036854775808", "32767", "-32768", "2147483647", "9223372036854775807"}, []string{"SMEMBERS", "k"}, NormSorted},
			{"zset-packed", []string{"ZADD", "k", "1.23456789012345", "m,:\x00", "-inf", "", "inf", "infinite"}, []string{"ZRANGE", "k", "0", "-1", "WITHSCORES"}, NormExact},
			{"zset-table", []string{"ZADD", "k", "-2", strings.Repeat("member", 30), "3", "second"}, []string{"ZRANGE", "k", "0", "-1", "WITHSCORES"}, NormExact},
		} {
			t.Run(direction.src.Name+"-to-"+direction.dst.Name+"/"+tc.name, func(t *testing.T) {
				must(t, redis, "FLUSHALL")
				must(t, cs, "FLUSHALL")
				must(t, direction.src, tc.setup...)
				if direction.src == redis && tc.name == "string-raw32" {
					must(t, redis, "CONFIG", "SET", "rdbcompression", "no")
				}
				payload := must(t, direction.src, "DUMP", "k")
				if direction.src == redis && tc.name == "string-raw32" {
					must(t, redis, "CONFIG", "SET", "rdbcompression", "yes")
				}
				if payload.Kind != '$' || payload.Null {
					t.Fatal("missing dump")
				}
				if direction.src == redis && len(payload.Str) < 300 {
					t.Logf("reference payload: %s", hex.EncodeToString([]byte(payload.Str)))
				}
				must(t, direction.dst, "RESTORE", "k", "0", payload.Str)
				expected, actual := must(t, direction.src, tc.read...), must(t, direction.dst, tc.read...)
				if !bytes.Equal(normalize(Step{Norm: tc.norm}, Reply{Val: expected}), normalize(Step{Norm: tc.norm}, Reply{Val: actual})) {
					t.Fatalf("restored value differs: %s / %s", fmt.Sprint(expected), fmt.Sprint(actual))
				}
			})
		}
	}
	must(t, redis, "FLUSHALL")
	must(t, cs, "FLUSHALL")
	for _, target := range []*Target{redis, cs} {
		must(t, target, "SET", "source", "v")
		payload := must(t, target, "DUMP", "source").Str
		must(t, target, "RESTORE", "relative", "60000", payload)
		result := must(t, target, "PTTL", "relative")
		remaining, err := strconv.ParseInt(result.Str, 10, 64)
		if err != nil || result.Kind != ':' || remaining < 59000 || remaining > 60000 {
			t.Fatalf("%s relative ttl=%s", target.Name, result.Str)
		}
		must(t, target, "RESTORE", "past", "1", payload, "ABSTTL")
		if must(t, target, "EXISTS", "past").Str != "0" {
			t.Fatal("past key revived")
		}
		must(t, target, "RESTORE", "idle", "0", payload, "IDLETIME", "30")
		idle := must(t, target, "OBJECT", "IDLETIME", "idle")
		if idle.Str != "30" {
			t.Fatalf("%s idle=%s", target.Name, idle.Str)
		}
	}
	t.Logf("bidirectional dump/restore verified at %s", time.Now().UTC().Format(time.RFC3339))
}

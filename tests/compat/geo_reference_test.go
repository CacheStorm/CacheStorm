//go:build compat

package compat

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
)

func TestGeoReferenceScoresAndTransfer(t *testing.T) {
	redis := &Target{Name: "redis", Addr: startRedis(t), conns: map[int]*Conn{}}
	cs := &Target{Name: "cachestorm", Addr: startCacheStorm(t), conns: map[int]*Conn{}}
	defer redis.resetConns()
	defer cs.resetConns()
	compare := func(args ...string) {
		t.Helper()
		a, b := redis.adminDo(args...), cs.adminDo(args...)
		if a.Err != "" || b.Err != "" || a.Val == nil || b.Val == nil || !bytes.Equal(a.Val.Encode(), b.Val.Encode()) {
			t.Fatalf("%v: %s / %s", args, a.Display(), b.Display())
		}
	}
	compare("FLUSHALL")
	rng := rand.New(rand.NewSource(73141))
	for i := 0; i < 256; i++ {
		member := fmt.Sprintf("p:%d\x00", i)
		lon, lat := fmt.Sprintf("%.17g", rng.Float64()*360-180), fmt.Sprintf("%.17g", rng.Float64()*170-85)
		compare("GEOADD", "geo", lon, lat, member)
		compare("ZSCORE", "geo", member)
		compare("GEOPOS", "geo", member)
		compare("GEOHASH", "geo", member)
	}
	for _, score := range []string{"0", "1.25", "-1", "inf", "-inf", "4503599627370496", "9007199254740991", "18446744073709551616"} {
		t.Logf("raw score %s", score)
		compare("ZADD", "raw", score, "point")
		compare("GEOPOS", "raw", "point")
		compare("GEOHASH", "raw", "point")
	}
	for _, direction := range []struct{ src, dst *Target }{{redis, cs}, {cs, redis}} {
		payload := direction.src.adminDo("DUMP", "geo")
		if payload.Err != "" || payload.Val == nil || payload.Val.Kind != '$' {
			t.Fatalf("DUMP: %s", payload.Display())
		}
		if got := direction.dst.adminDo("RESTORE", "transfer", "0", payload.Val.Str, "REPLACE"); got.Err != "" || got.Val == nil || got.Val.Kind == '-' {
			t.Fatalf("RESTORE: %s", got.Display())
		}
		before, after := direction.src.adminDo("ZRANGE", "geo", "0", "-1", "WITHSCORES"), direction.dst.adminDo("ZRANGE", "transfer", "0", "-1", "WITHSCORES")
		if before.Val == nil || after.Val == nil || !bytes.Equal(before.Val.Encode(), after.Val.Encode()) {
			t.Fatal("geo DUMP/RESTORE lost scores or members")
		}
	}
}

package server

import (
	"reflect"
	"testing"

	"github.com/cachestorm/cachestorm/internal/replication"
	"github.com/cachestorm/cachestorm/internal/resp"
)

func TestGeoZsetPersistenceAndEffectiveStore(t *testing.T) {
	for _, mode := range []string{"snapshot", "aof", "rewrite"} {
		t.Run(mode, func(t *testing.T) {
			cfg := snapshotConfig(t, t.TempDir())
			cfg.Persistence.AOF = mode != "snapshot"
			s := startReplServerCfg(t, cfg)
			c := dialRepl(t, s)
			if got := c.do("GEOADD", "geo", "13.361389", "38.115556", "Palermo", "15.087269", "37.502669", "Catania"); got.Int != 2 {
				t.Fatalf("seed=%+v", got)
			}
			c.do("PEXPIREAT", "geo", "4102444800000")
			if got := c.do("GEOSEARCHSTORE", "copy", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "COUNT", "1"); got.Type != resp.TypeInteger || got.Int != 1 {
				t.Fatalf("store=%+v", got)
			}
			commands := [][]string{{"TYPE", "geo"}, {"ZRANGE", "geo", "0", "-1", "WITHSCORES"}, {"GEOPOS", "geo", "Palermo", "Catania", "absent"}, {"GEOHASH", "geo", "Palermo"}, {"PEXPIRETIME", "geo"}, {"ZRANGE", "copy", "0", "-1", "WITHSCORES"}}
			before := make([]*resp.Value, len(commands))
			for i, args := range commands {
				before[i] = c.do(args...)
			}
			switch mode {
			case "snapshot":
				if got := c.do("SAVE"); got.Str != "OK" {
					t.Fatalf("save=%+v", got)
				}
			case "rewrite":
				if err := s.aof.Rewrite(replication.DBSource{DBs: s.dbs}); err != nil {
					t.Fatal(err)
				}
			}
			stopReplServer(s)
			restored := startReplServerCfg(t, cfg)
			rc := dialRepl(t, restored)
			for i, args := range commands {
				if got := rc.do(args...); !reflect.DeepEqual(got, before[i]) {
					t.Fatalf("%v changed across %s: %+v / %+v", args, mode, before[i], got)
				}
			}
		})
	}
}

func TestGeoZsetFullSync(t *testing.T) {
	master := startReplServer(t, nil)
	mc := dialRepl(t, master)
	mc.do("GEOADD", "geo", "13.361389", "38.115556", "Palermo")
	replica := startReplServer(t, nil)
	rc := replicate(t, replica, master)
	waitSynced(t, replica, master)
	for _, args := range [][]string{{"ZSCORE", "geo", "Palermo"}, {"GEOPOS", "geo", "Palermo"}, {"TYPE", "geo"}, {"GEORADIUS_RO", "geo", "13", "38", "200", "km"}, {"GEORADIUSBYMEMBER_RO", "geo", "Palermo", "200", "km"}} {
		if a, b := mc.do(args...), rc.do(args...); !reflect.DeepEqual(a, b) {
			t.Fatalf("full sync changed %v: %+v / %+v", args, a, b)
		}
	}
}

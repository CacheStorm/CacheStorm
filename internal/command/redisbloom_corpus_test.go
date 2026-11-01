package command

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// testdata/redisbloom_r8.jsonl holds commands and the replies Redis 8.2
// (RedisBloom 8.2.16) gave them: Bloom, cuckoo, count-min, top-k (decay 1,
// so without randomness) and t-digest, in RESP2 and then RESP3. The
// structures must be bit-for-bit the same as RedisBloom's for every false
// positive, eviction and centroid to agree.
func TestSketchesReplayRedisBloomCorpus(t *testing.T) {
	f, err := os.Open("testdata/redisbloom_r8.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	router := NewRouter()
	RegisterProbabilisticCommands(router)
	RegisterStatsCommands(router)
	st := store.NewStore()
	resp3 := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for n := 1; sc.Scan(); n++ {
		var c struct {
			Cmd  []string `json:"cmd"`
			Want *string  `json:"want"`
		}
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			t.Fatal(err)
		}
		switch strings.ToUpper(c.Cmd[0]) {
		case "FLUSHALL":
			st = store.NewStore()
			continue
		case "HELLO":
			resp3 = c.Cmd[1] == "3"
			continue
		}
		var out bytes.Buffer
		w := resp.NewWriter(&out)
		w.SetRESP3(resp3)
		ctx := NewContext(strings.ToUpper(c.Cmd[0]), bytesArgs(c.Cmd[1:]...), st, w)
		if err := router.Execute(ctx); err != nil {
			t.Fatalf("line %d %v: %v", n, c.Cmd, err)
		}
		ctx.Writer.Flush()
		if got := out.String(); c.Want != nil && got != *c.Want {
			t.Errorf("line %d %v:\n got %q\nwant %q", n, c.Cmd, got, *c.Want)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
}

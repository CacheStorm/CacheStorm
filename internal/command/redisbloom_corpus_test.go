package command

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/search"
	"github.com/cachestorm/cachestorm/internal/store"
)

// testdata/redisbloom_r8.jsonl holds commands and the replies Redis 8.2
// (RedisBloom 8.2.16) gave them: Bloom, cuckoo, count-min, top-k (decay 1,
// so without randomness) and t-digest, in RESP2 and then RESP3. The
// structures must be bit-for-bit the same as RedisBloom's for every false
// positive, eviction and centroid to agree.
func TestSketchesReplayRedisBloomCorpus(t *testing.T) {
	router := NewRouter()
	RegisterProbabilisticCommands(router)
	RegisterStatsCommands(router)
	replayRedisCorpus(t, router, "testdata/redisbloom_r8.jsonl")
}

// testdata/redisjson_r8.jsonl holds random JSON commands, and probes of
// paths that select nothing, overlap or name missing keys, with Redis 8.2's
// (RedisJSON 8.2.9) replies. JSON.GET with several paths is left out: its
// object's key order is a hash order that varies between Redis calls.
func TestJSONReplaysRedisJSONCorpus(t *testing.T) {
	router := NewRouter()
	RegisterJSONCommands(router)
	RegisterStringCommands(router)
	RegisterKeyCommands(router)
	replayRedisCorpus(t, router, "testdata/redisjson_r8.jsonl")
}

// testdata/redisearch_r8.jsonl holds FT commands and Redis 8.2's
// (RediSearch 8.2.13) replies: random documents and queries, the parse
// trees FT.EXPLAIN prints for random DIALECT 1 queries and their syntax
// errors, in RESP2 and RESP3. Replies left without "want" differ by
// design: hash-ordered lists and groups, FT.INFO's statistics, and scores
// that depend on the order RediSearch's iterators visit a union's terms.
func TestSearchReplaysRediSearchCorpus(t *testing.T) {
	router := NewRouter()
	RegisterSearchCommands(router)
	RegisterHashCommands(router)
	RegisterJSONCommands(router)
	RegisterStringCommands(router)
	RegisterKeyCommands(router)
	search.GetIndexManager().Reset()
	t.Cleanup(search.GetIndexManager().Reset)
	replayRedisCorpus(t, router, "testdata/redisearch_r8.jsonl")
}

// testdata/redisearch_vectors_r8.jsonl holds random vector indexes (FLAT
// and HNSW, single and double precision, L2, IP and cosine, 2 to 40
// dimensions) and KNN, hybrid and range queries with Redis 8.2's replies;
// vector blobs are written in hex. Redis ran on an x86 CPU with AVX and
// without AVX-512; with the same kernels the distances agree to the last
// bit.
func TestSearchReplaysVectorCorpus(t *testing.T) {
	t.Cleanup(search.SetVectorKernels(true, false))
	router := NewRouter()
	RegisterSearchCommands(router)
	RegisterHashCommands(router)
	RegisterKeyCommands(router)
	search.GetIndexManager().Reset()
	t.Cleanup(search.GetIndexManager().Reset)
	replayRedisCorpus(t, router, "testdata/redisearch_vectors_r8.jsonl")
}

// testdata/redisearch_synonyms_r8.jsonl holds random synonym groups,
// documents and single-term queries with their scores, FT.AGGREGATE
// pipelines that sort, page and filter in one arrange step, and the
// properties each step may read, with Redis 8.2's replies.
func TestSearchReplaysSynonymCorpus(t *testing.T) {
	router := NewRouter()
	RegisterSearchCommands(router)
	RegisterHashCommands(router)
	RegisterStringCommands(router)
	RegisterKeyCommands(router)
	search.GetIndexManager().Reset()
	t.Cleanup(search.GetIndexManager().Reset)
	replayRedisCorpus(t, router, "testdata/redisearch_synonyms_r8.jsonl")
}

// testdata/redisearch_suggest_r8.jsonl holds auto-complete dictionaries
// built and queried at random (scores, INCR, payloads, deletes, fuzzy and
// case-folded prefixes) with Redis 8.2's replies. TRIM is left out where it
// would drop results: RediSearch then frees results it still replies with.
func TestSuggestReplaysCorpus(t *testing.T) {
	router := NewRouter()
	RegisterSuggestCommands(router)
	RegisterStringCommands(router)
	RegisterKeyCommands(router)
	replayRedisCorpus(t, router, "testdata/redisearch_suggest_r8.jsonl")
}

func replayRedisCorpus(t *testing.T, router *Router, file string) {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	st := store.NewStore()
	resp3 := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for n := 1; sc.Scan(); n++ {
		var c struct {
			Cmd  []string `json:"cmd"`
			Hex  []int    `json:"hex"`
			Want *string  `json:"want"`
		}
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			t.Fatal(err)
		}
		switch strings.ToUpper(c.Cmd[0]) {
		case "FLUSHALL":
			st = store.NewStore()
			search.GetIndexManager().Reset()
			continue
		case "HELLO":
			resp3 = c.Cmd[1] == "3"
			continue
		}
		var out bytes.Buffer
		w := resp.NewWriter(&out)
		w.SetRESP3(resp3)
		args := bytesArgs(c.Cmd[1:]...)
		for _, i := range c.Hex {
			b, err := hex.DecodeString(c.Cmd[i])
			if err != nil {
				t.Fatal(err)
			}
			args[i-1] = b
		}
		ctx := NewContext(strings.ToUpper(c.Cmd[0]), args, st, w)
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

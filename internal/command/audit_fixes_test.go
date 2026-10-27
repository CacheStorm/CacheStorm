package command

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

func auditRouter() (*Router, *store.Store) {
	router := NewRouter()
	for _, register := range []func(*Router){
		RegisterMoreCommands, RegisterResilienceCommands, RegisterMVCCCommands,
		RegisterDataStructuresCommands, RegisterActorCommands, RegisterAdvancedCommands2,
		RegisterExtraCommands, RegisterIntegrationCommands, RegisterTemplateCommands,
		RegisterEncodingCommands, RegisterMLCommands, RegisterExtendedCommands,
		RegisterCacheWarmingCommands,
	} {
		register(router)
	}
	return router, store.NewStore()
}

func auditRun(t *testing.T, router *Router, s *store.Store, cmd string, args ...string) string {
	t.Helper()
	reply, err := registryReply(router, s, cmd, args...)
	if err != nil {
		t.Fatalf("%s %v: unexpected error %v", cmd, args, err)
	}
	return reply
}

func auditExpect(t *testing.T, router *Router, s *store.Store, want string, cmd string, args ...string) {
	t.Helper()
	if got := auditRun(t, router, s, cmd, args...); got != want {
		t.Fatalf("%s %v = %q, want %q", cmd, args, got, want)
	}
}

func auditExpectError(t *testing.T, router *Router, s *store.Store, cmd string, args ...string) {
	t.Helper()
	if got := auditRun(t, router, s, cmd, args...); !strings.HasPrefix(got, "-ERR") {
		t.Fatalf("%s %v = %q, want an ERR reply", cmd, args, got)
	}
}

func TestAuditCreateRejectsNonPositiveSizes(t *testing.T) {
	router, s := auditRouter()
	for _, c := range [][]string{
		{"BUCKETX.CREATE", "b", "10", "1", "0"},
		{"BUCKETX.CREATE", "b", "10", "1", "-5"},
		{"BLOOMX.CREATE", "b", "0", "3"},
		{"BLOOMX.CREATE", "b", "-1", "3"},
		{"SKETCH.CREATE", "s", "0", "4"},
		{"SKETCH.CREATE", "s", "4", "-1"},
		{"RINGBUFFER.CREATE", "r", "0"},
		{"PARTITION.CREATE", "p", "0"},
		{"SLIDING.CREATE", "s", "0", "1000"},
		{"PARTITIONX.CREATE", "p", "-2"},
		{"ROLLUP.CREATE", "r", "0"},
		{"SLIDINGWINDOW.CREATE", "w", "0", "10"},
		{"RATELIMIT.CREATE", "l", "10", "0"},
		{"BUFFERX.CREATE", "b", "-1"},
	} {
		auditExpectError(t, router, s, c[0], c[1:]...)
	}
	for _, c := range [][]string{
		{"BUCKETX.TAKE", "b", "1"},
		{"SLIDING.CHECK", "s"},
		{"ROLLUP.ADD", "r", "1", "1"},
		{"SLIDINGWINDOW.INCR", "w", "k"},
	} {
		auditExpectError(t, router, s, c[0], c[1:]...)
	}
}

func TestAuditSlidingWindowIncrIsPerKey(t *testing.T) {
	router, s := auditRouter()
	auditExpect(t, router, s, "+OK\r\n", "SLIDINGWINDOW.CREATE", "w", "60000", "2")
	auditExpect(t, router, s, "*2\r\n:1\r\n:1\r\n", "SLIDINGWINDOW.INCR", "w", "a")
	auditExpect(t, router, s, "*2\r\n:1\r\n:2\r\n", "SLIDINGWINDOW.INCR", "w", "a")
	auditExpect(t, router, s, "*2\r\n:0\r\n:2\r\n", "SLIDINGWINDOW.INCR", "w", "a")
	auditExpect(t, router, s, "*2\r\n:1\r\n:1\r\n", "SLIDINGWINDOW.INCR", "w", "b")
	auditExpect(t, router, s, ":2\r\n", "SLIDINGWINDOW.COUNT", "w", "a")
	auditExpect(t, router, s, ":3\r\n", "SLIDINGWINDOW.COUNT", "w")
	auditExpect(t, router, s, "+OK\r\n", "SLIDINGWINDOW.RESET", "w")
	auditExpect(t, router, s, ":0\r\n", "SLIDINGWINDOW.COUNT", "w")
}

func TestAuditTrieDelete(t *testing.T) {
	router, s := auditRouter()
	auditRun(t, router, s, "TRIE.ADD", "t", "hello")
	auditRun(t, router, s, "TRIE.ADD", "t", "help")
	auditExpect(t, router, s, ":0\r\n", "TRIE.DELETE", "t", "hel")
	auditExpect(t, router, s, ":0\r\n", "TRIE.DELETE", "t", "world")
	auditExpect(t, router, s, ":0\r\n", "TRIE.DELETE", "missing", "hello")
	auditExpect(t, router, s, ":1\r\n", "TRIE.DELETE", "t", "hello")
	auditExpect(t, router, s, ":0\r\n", "TRIE.DELETE", "t", "hello")
	auditExpect(t, router, s, ":0\r\n", "TRIE.SEARCH", "t", "hello")
	auditExpect(t, router, s, ":1\r\n", "TRIE.SEARCH", "t", "help")
}

func TestAuditValidateUsesRegexp(t *testing.T) {
	router, s := auditRouter()
	auditExpectError(t, router, s, "VALIDATE.CREATE", "bad", "a(")
	auditExpect(t, router, s, "+OK\r\n", "VALIDATE.CREATE", "digits", `^[0-9]+$`)
	auditExpect(t, router, s, ":1\r\n", "VALIDATE.CHECK", "digits", "12345")
	auditExpect(t, router, s, ":0\r\n", "VALIDATE.CHECK", "digits", "12a45")
}

func TestAuditVectorClockCompareDirection(t *testing.T) {
	router, s := auditRouter()
	auditRun(t, router, s, "VECTOR_CLOCK.CREATE", "a", "n1")
	auditRun(t, router, s, "VECTOR_CLOCK.CREATE", "b", "n1")
	auditExpect(t, router, s, "$5\r\nequal\r\n", "VECTOR_CLOCK.COMPARE", "a", "b")
	auditRun(t, router, s, "VECTOR_CLOCK.INCREMENT", "b", "n1")
	auditExpect(t, router, s, "$6\r\nbefore\r\n", "VECTOR_CLOCK.COMPARE", "a", "b")
	auditExpect(t, router, s, "$5\r\nafter\r\n", "VECTOR_CLOCK.COMPARE", "b", "a")
	auditRun(t, router, s, "VECTOR_CLOCK.INCREMENT", "a", "n2")
	auditExpect(t, router, s, "$10\r\nconcurrent\r\n", "VECTOR_CLOCK.COMPARE", "a", "b")
}

func TestAuditTimerStatusStopsRunning(t *testing.T) {
	router, s := auditRouter()
	id := registryBulkValue(auditRun(t, router, s, "TIMER.CREATE", "t", "1"))
	time.Sleep(10 * time.Millisecond)
	got := auditRun(t, router, s, "TIMER.STATUS", id)
	if !strings.Contains(got, "running\r\n$5\r\nfalse") || !strings.Contains(got, ":0\r\n") {
		t.Fatalf("expired timer status = %q, want running=false remaining=0", got)
	}
	id = registryBulkValue(auditRun(t, router, s, "TIMER.CREATE", "t2", "60000"))
	if got := auditRun(t, router, s, "TIMER.STATUS", id); !strings.Contains(got, "running\r\n$4\r\ntrue") {
		t.Fatalf("live timer status = %q, want running=true", got)
	}
}

func TestAuditBufferXStartsEmpty(t *testing.T) {
	router, s := auditRouter()
	auditRun(t, router, s, "BUFFERX.CREATE", "b", "16")
	auditRun(t, router, s, "BUFFERX.WRITE", "b", "hi")
	auditExpect(t, router, s, "$2\r\nhi\r\n", "BUFFERX.READ", "b")
}

func TestAuditPermitRevokeUngranted(t *testing.T) {
	router, s := auditRouter()
	auditRun(t, router, s, "PERMIT.GRANT", "u", "doc", "read")
	auditExpect(t, router, s, ":0\r\n", "PERMIT.REVOKE", "u", "doc", "write")
	auditExpect(t, router, s, ":1\r\n", "PERMIT.REVOKE", "u", "doc", "read")
	auditExpect(t, router, s, ":0\r\n", "PERMIT.REVOKE", "u", "doc", "read")
}

func TestAuditLockXStatusExpires(t *testing.T) {
	router, s := auditRouter()
	auditRun(t, router, s, "LOCKX.ACQUIRE", "k", "h", "1")
	time.Sleep(10 * time.Millisecond)
	auditExpect(t, router, s, "*2\r\n$6\r\nlocked\r\n:0\r\n", "LOCKX.STATUS", "k")
	auditRun(t, router, s, "LOCKX.ACQUIRE", "k2", "h", "60000")
	if got := auditRun(t, router, s, "LOCKX.STATUS", "k2"); !strings.HasPrefix(got, "*6\r\n$6\r\nlocked\r\n:1\r\n") {
		t.Fatalf("held lock status = %q", got)
	}
}

func TestAuditShuffleGetIsRandom(t *testing.T) {
	router, s := auditRouter()
	firstAlwaysA := true
	for trial := 0; trial < 40; trial++ {
		auditRun(t, router, s, "SHUFFLE.CREATE", "s")
		for _, v := range []string{"a", "b", "c"} {
			auditRun(t, router, s, "SHUFFLE.ADD", "s", v)
		}
		seen := map[string]bool{}
		for i := 0; i < 3; i++ {
			v := registryBulkValue(auditRun(t, router, s, "SHUFFLE.GET", "s"))
			if i == 0 && v != "a" {
				firstAlwaysA = false
			}
			seen[v] = true
		}
		if len(seen) != 3 {
			t.Fatalf("SHUFFLE.GET returned %v, want each element once", seen)
		}
		auditExpect(t, router, s, "$-1\r\n", "SHUFFLE.GET", "s")
	}
	if firstAlwaysA {
		t.Fatal("SHUFFLE.GET popped the first element in 40 of 40 trials; it is FIFO, not random")
	}
}

func TestAuditRateLimitCheckIsPerIdentifier(t *testing.T) {
	router, s := auditRouter()
	auditRun(t, router, s, "RATELIMIT.CREATE", "l", "1", "60000")
	if got := auditRun(t, router, s, "RATELIMIT.CHECK", "l", "alice"); !strings.HasPrefix(got, "*3\r\n:1\r\n") {
		t.Fatalf("first alice check = %q", got)
	}
	if got := auditRun(t, router, s, "RATELIMIT.CHECK", "l", "alice"); !strings.HasPrefix(got, "*3\r\n:0\r\n") {
		t.Fatalf("second alice check = %q, want refused", got)
	}
	if got := auditRun(t, router, s, "RATELIMIT.CHECK", "l", "bob"); !strings.HasPrefix(got, "*3\r\n:1\r\n") {
		t.Fatalf("bob check = %q, want allowed", got)
	}
}

func TestAuditGeohashRoundTrip(t *testing.T) {
	if got := encodeGeohash(57.64911, 10.40744); !strings.HasPrefix(got, "u4pruydqqvj") || len(got) != 12 {
		t.Fatalf("encodeGeohash = %q, want prefix u4pruydqqvj", got)
	}
	points := [][2]float64{{57.64911, 10.40744}, {-33.8688, 151.2093}, {40.7128, -74.0060}, {0, 0}, {-89.9, -179.9}}
	for _, p := range points {
		full := encodeGeohash(p[0], p[1])
		for n := 1; n <= 12; n++ {
			lat, lon := decodeGeohash(full[:n])
			lonBits := (5*n + 1) / 2
			latBits := 5 * n / 2
			if math.Abs(lat-p[0]) > 90/math.Pow(2, float64(latBits)) || math.Abs(lon-p[1]) > 180/math.Pow(2, float64(lonBits)) {
				t.Fatalf("decode(%q) = %f,%f, not within the cell of %f,%f", full[:n], lat, lon, p[0], p[1])
			}
		}
	}
}

func TestAuditMathFixes(t *testing.T) {
	router, s := auditRouter()
	auditExpect(t, router, s, ":-3\r\n", "MATH.FLOOR", "-2.5")
	auditExpect(t, router, s, ":2\r\n", "MATH.FLOOR", "2.5")
	auditExpect(t, router, s, ":-3\r\n", "MATH.ROUND", "-2.5")
	auditExpect(t, router, s, ":-2\r\n", "MATH.ROUND", "-2.4")
	auditExpect(t, router, s, ":3\r\n", "MATH.ROUND", "2.5")
	auditExpect(t, router, s, "$8\r\n0.000000\r\n", "MATH.SQRT", "0")
	auditExpect(t, router, s, "$8\r\n3.000000\r\n", "MATH.SQRT", "9")
	auditExpectError(t, router, s, "MATH.SQRT", "-4")
	auditExpectError(t, router, s, "MATH.RANDOM", "10", "5")
	auditExpect(t, router, s, ":7\r\n", "MATH.RANDOM", "7", "7")
	for i := 0; i < 200; i++ {
		got := auditRun(t, router, s, "MATH.RANDOM", "-3", "3")
		if got != ":-3\r\n" && got != ":-2\r\n" && got != ":-1\r\n" && got != ":0\r\n" && got != ":1\r\n" && got != ":2\r\n" && got != ":3\r\n" {
			t.Fatalf("MATH.RANDOM -3 3 = %q, out of range", got)
		}
	}
}

func TestAuditEvalRegex(t *testing.T) {
	router, s := auditRouter()
	auditExpect(t, router, s, ":1\r\n", "EVAL.REGEX", `^h.l+o$`, "hello")
	auditExpect(t, router, s, ":0\r\n", "EVAL.REGEX", `^h.l+o$`, "help")
	auditExpectError(t, router, s, "EVAL.REGEX", "(", "x")
	auditExpect(t, router, s, "*2\r\n$2\r\n12\r\n$3\r\n345\r\n", "EVAL.REGEXMATCH", `[0-9]+`, "a12b345")
	auditExpectError(t, router, s, "EVAL.REGEXMATCH", "[", "x")
	auditExpect(t, router, s, "$5\r\naXbXc\r\n", "EVAL.REGEXREPLACE", `[0-9]+`, "X", "a12b3c")
	auditExpect(t, router, s, "$9\r\nb-a-c-c-c\r\n", "EVAL.REGEXREPLACE", `(\w)(\w)`, "$2-$1-", "abccc")
	auditExpectError(t, router, s, "EVAL.REGEXREPLACE", "(", "x", "y")
}

func TestAuditValidateJSON(t *testing.T) {
	router, s := auditRouter()
	auditExpect(t, router, s, ":1\r\n", "VALIDATE.JSON", `{"a":[1,2]}`)
	auditExpect(t, router, s, ":1\r\n", "VALIDATE.JSON", `42`)
	auditExpect(t, router, s, ":0\r\n", "VALIDATE.JSON", `{not json}`)
	auditExpect(t, router, s, ":0\r\n", "VALIDATE.JSON", `[1,]`)
}

func TestAuditTimestampParse(t *testing.T) {
	router, s := auditRouter()
	for _, c := range []struct {
		in   string
		unix string
	}{
		{"1700000000", ":1700000000"},
		{"1700000000000", ":1700000000"},
		{"12345", ":12345"},
		{"2023-11-14T22:13:20Z", ":1700000000"},
		{"2023-11-14", ":1699920000"},
		{"2023-11-14 22:13:20", ":1700000000"},
	} {
		got := auditRun(t, router, s, "TIMESTAMP.PARSE", c.in)
		if !strings.HasPrefix(got, "*4\r\n$4\r\nunix\r\n"+c.unix+"\r\n") {
			t.Fatalf("TIMESTAMP.PARSE %q = %q, want unix %s", c.in, got, c.unix)
		}
	}
	auditExpectError(t, router, s, "TIMESTAMP.PARSE", "yesterday")
}

func TestAuditStrTitle(t *testing.T) {
	router, s := auditRouter()
	auditExpect(t, router, s, "$17\r\nHello Big  World\t\r\n", "STR.TITLE", "hELLO big  wORLD\t")
	auditExpect(t, router, s, "$0\r\n\r\n", "STR.TITLE", "")
}

func TestAuditTensorShapeMustMatchData(t *testing.T) {
	router, s := auditRouter()
	auditExpectError(t, router, s, "TENSOR.CREATE", "t", "2", "3", "|", "1", "2")
	auditExpectError(t, router, s, "TENSOR.CREATE", "t", "-2", "-3", "|", "1", "2", "3", "4", "5", "6")
	auditExpect(t, router, s, "+OK\r\n", "TENSOR.CREATE", "t", "2", "3", "|", "1", "2", "3", "4", "5", "6")
	auditExpect(t, router, s, "+OK\r\n", "TENSOR.RESHAPE", "t", "3", "2")
	auditExpectError(t, router, s, "TENSOR.RESHAPE", "t", "4", "2")
	auditExpectError(t, router, s, "TENSOR.RESHAPE", "t", "0")
}

func TestAuditVectorAddChecksDimension(t *testing.T) {
	router, s := auditRouter()
	auditRun(t, router, s, "VECTOR.CREATE", "v", "3")
	auditExpectError(t, router, s, "VECTOR.ADD", "v", "a", "2", "1", "2")
	auditExpectError(t, router, s, "VECTOR.ADD", "v", "a", "3", "1", "2")
	auditExpect(t, router, s, "+OK\r\n", "VECTOR.ADD", "v", "a", "3", "1", "2", "3")
}

func TestAuditLeaderCurrentExpires(t *testing.T) {
	router, s := auditRouter()
	auditRun(t, router, s, "LEADER.ELECT", "e", "node1", "1")
	time.Sleep(10 * time.Millisecond)
	auditExpect(t, router, s, "$-1\r\n", "LEADER.CURRENT", "e")
	auditRun(t, router, s, "LEADER.ELECT", "e", "node2", "60000")
	if got := auditRun(t, router, s, "LEADER.CURRENT", "e"); !strings.Contains(got, "node2") {
		t.Fatalf("LEADER.CURRENT = %q, want node2", got)
	}
}

func TestAuditKeyRenameKeepsTTL(t *testing.T) {
	router, s := auditRouter()
	for _, c := range []struct {
		cmd, src, dst string
		args          []string
	}{
		{"KEY.RENAME", "a1", "b1", []string{"a1", "b1"}},
		{"KEY.RENAMENX", "a2", "b2", []string{"a2", "b2"}},
		{"KEY.MOVE", "a3", "ns:a3", []string{"a3", "ns"}},
	} {
		s.Set(c.src, &store.StringValue{Data: []byte("v")}, store.SetOptions{TTL: 100 * time.Second})
		if got := auditRun(t, router, s, c.cmd, c.args...); strings.HasPrefix(got, "-") {
			t.Fatalf("%s = %q", c.cmd, got)
		}
		if ttl := s.TTLMillis(c.dst); ttl <= 0 || ttl > 100000 {
			t.Fatalf("%s: TTL of %s = %d, want the source's remaining TTL", c.cmd, c.dst, ttl)
		}
		if s.Exists(c.src) {
			t.Fatalf("%s left %s behind", c.cmd, c.src)
		}
	}
	s.Set("x", &store.StringValue{Data: []byte("1")}, store.SetOptions{})
	s.Set("y", &store.StringValue{Data: []byte("2")}, store.SetOptions{})
	auditExpect(t, router, s, ":0\r\n", "KEY.RENAMENX", "x", "y")
	auditExpectError(t, router, s, "KEY.RENAME", "nope", "z")
}

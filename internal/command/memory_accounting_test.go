package command

import (
	"bytes"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func newMemAcctRouter() *Router {
	r := NewRouter()
	RegisterStringCommands(r)
	RegisterKeyCommands(r)
	RegisterServerCommands(r)
	RegisterListCommands(r)
	RegisterHashCommands(r)
	RegisterSetCommands(r)
	RegisterSortedSetCommands(r)
	RegisterStreamCommands(r)
	RegisterTransactionCommands(r)
	RegisterScriptCommands(r)
	return r
}

func memAcctRun(r *Router, s *store.Store, tx *Transaction, args ...string) string {
	raw := make([][]byte, 0, len(args)-1)
	for _, a := range args[1:] {
		raw = append(raw, []byte(a))
	}
	var buf bytes.Buffer
	w := resp.NewWriter(&buf)
	ctx := NewContext(args[0], raw, s, w)
	if tx != nil {
		ctx.Transaction = tx
	}
	if err := r.Execute(ctx); err != nil {
		_ = w.WriteError(err.Error())
	}
	_ = w.Flush()
	return buf.String()
}

// refValueSize recounts a value's size from scratch with the formulas the
// incremental counters must agree with.
func refValueSize(v store.Value) int64 {
	switch v := v.(type) {
	case *store.ListValue:
		n := int64(165)
		for _, el := range v.Elements {
			n += int64(len(el)) + 47
		}
		return n
	case *store.HashValue:
		n := int64(450)
		for k, val := range v.Fields {
			n += int64(len(k)) + int64(len(val)) + 88
		}
		return n
	case *store.SetValue:
		n := int64(337)
		for k := range v.Members {
			n += int64(len(k)) + 45
		}
		return n
	case *store.SortedSetValue:
		n := int64(930)
		for k := range v.Members {
			n += int64(len(k)) + 122
		}
		return n
	case *store.StreamValue:
		n := int64(48)
		for _, e := range v.Entries {
			n += int64(len(e.ID)) + 32
			for _, p := range e.FieldPairs() {
				n += int64(len(p.Name)) + int64(len(p.Value)) + 80
			}
		}
		return n
	}
	return v.SizeOf()
}

// checkMemAccounting asserts that every value's incremental size equals a
// full recount and that the store's and tracker's usage equal the sum of the
// recounted footprints.
func checkMemAccounting(t *testing.T, s *store.Store, step string) {
	t.Helper()
	var total int64
	for _, key := range s.Keys() {
		entry, ok := s.GetShard(key).Get(key)
		if !ok {
			continue
		}
		ref := refValueSize(entry.Value)
		if got := entry.Value.SizeOf(); got != ref {
			t.Fatalf("after %s: %s (%T) SizeOf=%d, recount=%d", step, key, entry.Value, got, ref)
		}
		fp := int64(store.EntryOverhead) + ref + int64(len(key)) + 16
		for _, tag := range entry.TagList() {
			fp += int64(len(tag)) + 16
		}
		total += fp
	}
	if got := s.MemUsage(); got != total {
		t.Fatalf("after %s: MemUsage=%d, recount=%d", step, got, total)
	}
	if mt := s.MemoryTracker(); mt != nil && mt.Usage() != total {
		t.Fatalf("after %s: tracker usage=%d, recount=%d", step, mt.Usage(), total)
	}
}

func randVal(rng *rand.Rand) string {
	sizes := []int{0, 1, 3, 8, 40, 200}
	n := sizes[rng.Intn(len(sizes))]
	return strings.Repeat(string(rune('a'+rng.Intn(4))), n)
}

func pick(rng *rand.Rand, xs ...string) string { return xs[rng.Intn(len(xs))] }

func randCommand(rng *rand.Rand, streamSeq *int) []string {
	lk := func() string { return pick(rng, "l0", "l1", "l2") }
	hk := func() string { return pick(rng, "h0", "h1") }
	sk := func() string { return pick(rng, "s0", "s1", "s2") }
	zk := func() string { return pick(rng, "z0", "z1", "z2") }
	mem := func() string { return pick(rng, "a", "bb", "ccc", "dddddddd", strings.Repeat("e", 50)) }
	num := func(lo, hi int) string { return strconv.Itoa(lo + rng.Intn(hi-lo+1)) }
	score := func() string { return pick(rng, "1", "2.5", "-3", "10", "100") }
	switch rng.Intn(48) {
	case 0:
		return []string{"LPUSH", lk(), randVal(rng), randVal(rng)}
	case 1:
		return []string{"RPUSH", lk(), randVal(rng), randVal(rng), randVal(rng)}
	case 2:
		return []string{"LPUSHX", lk(), randVal(rng)}
	case 3:
		return []string{"RPUSHX", lk(), randVal(rng)}
	case 4:
		return []string{"LPOP", lk(), num(1, 3)}
	case 5:
		return []string{"RPOP", lk()}
	case 6:
		return []string{"LSET", lk(), num(-3, 3), randVal(rng)}
	case 7:
		return []string{"LREM", lk(), num(-2, 2), randVal(rng)}
	case 8:
		return []string{"LINSERT", lk(), pick(rng, "BEFORE", "AFTER"), randVal(rng), randVal(rng)}
	case 9:
		return []string{"LTRIM", lk(), num(-4, 3), num(-4, 6)}
	case 10:
		return []string{"LMOVE", lk(), lk(), pick(rng, "LEFT", "RIGHT"), pick(rng, "LEFT", "RIGHT")}
	case 11:
		return []string{"RPOPLPUSH", lk(), lk()}
	case 12:
		return []string{"LMPOP", "2", lk(), lk(), pick(rng, "LEFT", "RIGHT"), "COUNT", num(1, 4)}
	case 13:
		return []string{"HSET", hk(), mem(), randVal(rng), mem(), randVal(rng)}
	case 14:
		return []string{"HSETNX", hk(), mem(), randVal(rng)}
	case 15:
		return []string{"HDEL", hk(), mem(), mem()}
	case 16:
		return []string{"HINCRBY", hk(), pick(rng, "n1", "n2"), num(-1000, 100000)}
	case 17:
		return []string{"HINCRBYFLOAT", hk(), pick(rng, "f1", "f2"), pick(rng, "0.5", "1e3", "-7.25")}
	case 18:
		return []string{"HMSET", hk(), mem(), randVal(rng)}
	case 19:
		return []string{"HGETDEL", hk(), "FIELDS", "1", mem()}
	case 20:
		return []string{"SADD", sk(), mem(), mem(), mem()}
	case 21:
		return []string{"SREM", sk(), mem()}
	case 22:
		return []string{"SPOP", sk(), num(1, 3)}
	case 23:
		return []string{"SMOVE", sk(), sk(), mem()}
	case 24:
		return []string{pick(rng, "SINTERSTORE", "SUNIONSTORE", "SDIFFSTORE"), sk(), sk(), sk()}
	case 25:
		return []string{"ZADD", zk(), score(), mem(), score(), mem()}
	case 26:
		return []string{"ZADD", zk(), pick(rng, "NX", "XX", "GT", "LT", "CH"), score(), mem()}
	case 27:
		return []string{"ZINCRBY", zk(), score(), mem()}
	case 28:
		return []string{"ZREM", zk(), mem(), mem()}
	case 29:
		return []string{pick(rng, "ZPOPMIN", "ZPOPMAX"), zk(), num(1, 2)}
	case 30:
		return []string{"ZREMRANGEBYRANK", zk(), num(-2, 1), num(-1, 2)}
	case 31:
		return []string{"ZREMRANGEBYSCORE", zk(), pick(rng, "-inf", "0", "2"), pick(rng, "+inf", "5", "10")}
	case 32:
		return []string{"ZREMRANGEBYLEX", zk(), pick(rng, "-", "[b"), pick(rng, "+", "(d")}
	case 33:
		return []string{"ZMPOP", "1", zk(), pick(rng, "MIN", "MAX"), "COUNT", num(1, 3)}
	case 34:
		return []string{pick(rng, "ZUNIONSTORE", "ZINTERSTORE", "ZDIFFSTORE"), zk(), "2", zk(), zk()}
	case 35:
		return []string{"ZRANGESTORE", zk(), zk(), "0", num(-1, 2)}
	case 36:
		*streamSeq++
		return []string{"XADD", pick(rng, "x0", "x1"), fmt.Sprintf("%d-1", *streamSeq), mem(), randVal(rng)}
	case 37:
		*streamSeq++
		return []string{"XADD", pick(rng, "x0", "x1"), "MAXLEN", num(0, 5), fmt.Sprintf("%d-1", *streamSeq), mem(), randVal(rng), mem(), randVal(rng)}
	case 38:
		return []string{"XDEL", pick(rng, "x0", "x1"), fmt.Sprintf("%d-1", rng.Intn(*streamSeq+1))}
	case 39:
		return []string{"XTRIM", pick(rng, "x0", "x1"), "MAXLEN", num(0, 6)}
	case 40:
		return []string{"APPEND", pick(rng, "str0", "str1"), randVal(rng)}
	case 41:
		return []string{"SETRANGE", pick(rng, "str0", "str1"), num(0, 30), randVal(rng)}
	case 42:
		return []string{"SET", pick(rng, "str0", "str1", "l0", "h0"), randVal(rng)}
	case 43:
		return []string{"DEL", pick(rng, "l0", "h1", "s0", "z1", "x0", "str1")}
	case 44:
		group := pick(rng, "l", "h", "s", "z")
		return []string{"RENAME", group + num(0, 1), group + num(0, 2)}
	case 45:
		group := pick(rng, "l", "s", "z")
		return []string{"COPY", group + num(0, 1), group + num(1, 2), "REPLACE"}
	case 46:
		return []string{"SORT", lk(), "ALPHA", "STORE", lk()}
	default:
		return []string{"EVAL", "redis.call('RPUSH', KEYS[1], ARGV[1]); return redis.call('HSET', KEYS[2], ARGV[1], ARGV[2])", "2", lk(), hk(), mem(), randVal(rng)}
	}
}

// Property: after any sequence of writes, each collection's incrementally
// maintained size equals a full recount, and the store's and the memory
// tracker's usage equal the sum of the recounted footprints.
func TestMemoryAccountingMatchesRecount(t *testing.T) {
	ran := make(map[string]int)
	t.Cleanup(func() {
		// Guard the property's power: every generated command must have
		// succeeded at least once, or a typo would silently test nothing.
		for name, ok := range ran {
			if ok == 0 {
				t.Errorf("%s never succeeded", name)
			}
		}
	})
	for seed := int64(1); seed <= 6; seed++ {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			r := newMemAcctRouter()
			s := store.NewStore()
			s.ConfigureMemory(1<<40, store.EvictionNoEviction, 70, 85, 5)
			rng := rand.New(rand.NewSource(seed))
			seq := 0
			tx := NewTransaction()
			for i := 0; i < 2500; i++ {
				if rng.Intn(25) == 0 {
					memAcctRun(r, s, tx, "MULTI")
					for j := 0; j < 1+rng.Intn(4); j++ {
						memAcctRun(r, s, tx, randCommand(rng, &seq)...)
					}
					memAcctRun(r, s, tx, "EXEC")
					checkMemAccounting(t, s, fmt.Sprintf("step %d EXEC", i))
					continue
				}
				cmd := randCommand(rng, &seq)
				out := memAcctRun(r, s, nil, cmd...)
				ran[cmd[0]] += 0
				if !strings.HasPrefix(out, "-") {
					ran[cmd[0]]++
				}
				checkMemAccounting(t, s, fmt.Sprintf("step %d %q", i, cmd))
			}
		})
	}
}

func infoUsedMemory(t *testing.T, r *Router, s *store.Store) int64 {
	t.Helper()
	out := memAcctRun(r, s, nil, "INFO", "memory")
	for _, line := range strings.Split(out, "\r\n") {
		if v, ok := strings.CutPrefix(line, "used_memory:"); ok {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				t.Fatalf("used_memory %q: %v", v, err)
			}
			return n
		}
	}
	t.Fatalf("INFO has no used_memory: %q", out)
	return 0
}

// Regression: in-place growth was never charged, so 10 MB pushed into one
// list against a 1 MB limit was tracked as ~1 KB and never refused.
func TestMaxmemoryBoundsInPlaceGrowth(t *testing.T) {
	const limit = 1 << 20
	r := newMemAcctRouter()
	s := store.NewStore()
	s.ConfigureMemory(limit, store.EvictionNoEviction, 70, 85, 5)
	val := strings.Repeat("x", 1024)

	refused := 0
	for i := 0; i < 10000; i++ {
		out := memAcctRun(r, s, nil, "RPUSH", "big", val)
		if strings.HasPrefix(out, "-") {
			if !strings.HasPrefix(out, "-OOM command not allowed when used memory > 'maxmemory'.") {
				t.Fatalf("refusal reply %q", out)
			}
			refused++
		}
	}
	if refused == 0 {
		t.Fatal("pushing 10 MB into one list never hit the 1 MB limit")
	}
	used := s.MemoryTracker().Usage()
	if used < limit || used > limit+2048 {
		t.Fatalf("used memory %d, want just over the %d limit", used, limit)
	}
	if got := infoUsedMemory(t, r, s); got != used {
		t.Fatalf("INFO used_memory=%d, tracker=%d", got, used)
	}
	checkMemAccounting(t, s, "the fill")

	// Every growth path is refused, while reads and shrinking writes run.
	for _, cmd := range [][]string{
		{"LPUSH", "big", "v"}, {"HSET", "h", "f", "v"}, {"SADD", "s", "m"},
		{"ZADD", "z", "1", "m"}, {"XADD", "x", "*", "f", "v"}, {"SET", "k", "v"},
		{"APPEND", "k", "v"},
	} {
		if out := memAcctRun(r, s, nil, cmd...); !strings.HasPrefix(out, "-OOM") {
			t.Fatalf("%v over the limit: %q", cmd, out)
		}
	}
	if out := memAcctRun(r, s, nil, "LLEN", "big"); !strings.HasPrefix(out, ":") {
		t.Fatalf("LLEN over the limit: %q", out)
	}

	// Popping frees memory, so writes are admitted again.
	memAcctRun(r, s, nil, "LPOP", "big", "100")
	if s.MemoryTracker().Usage() >= used-100*1024 {
		t.Fatalf("LPOP of 100 KB left usage at %d (was %d)", s.MemoryTracker().Usage(), used)
	}
	if out := memAcctRun(r, s, nil, "RPUSH", "big", val); !strings.HasPrefix(out, ":") {
		t.Fatalf("RPUSH after LPOP freed memory: %q", out)
	}
	checkMemAccounting(t, s, "LPOP")

	// DEL returns the whole list.
	memAcctRun(r, s, nil, "DEL", "big")
	if u := s.MemoryTracker().Usage(); u != 0 {
		t.Fatalf("usage after DEL = %d, want 0", u)
	}
	if out := memAcctRun(r, s, nil, "RPUSH", "big", val); out != ":1\r\n" {
		t.Fatalf("RPUSH after DEL: %q", out)
	}
	if got := infoUsedMemory(t, r, s); got != s.MemoryTracker().Usage() {
		t.Fatalf("INFO used_memory=%d, tracker=%d", got, s.MemoryTracker().Usage())
	}
}

// Each collection type is bounded the same way.
func TestMaxmemoryBoundsEveryCollectionType(t *testing.T) {
	const limit = 256 << 10
	val := strings.Repeat("v", 512)
	writes := map[string]func(i int) []string{
		"hash":   func(i int) []string { return []string{"HSET", "k", "f" + strconv.Itoa(i), val} },
		"set":    func(i int) []string { return []string{"SADD", "k", val + strconv.Itoa(i)} },
		"zset":   func(i int) []string { return []string{"ZADD", "k", "1", val + strconv.Itoa(i)} },
		"stream": func(i int) []string { return []string{"XADD", "k", "*", "f", val} },
		"lpush":  func(i int) []string { return []string{"LPUSH", "k", val} },
	}
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			r := newMemAcctRouter()
			s := store.NewStore()
			s.ConfigureMemory(limit, store.EvictionNoEviction, 70, 85, 5)
			refused := false
			for i := 0; i < 4000 && !refused; i++ {
				refused = strings.HasPrefix(memAcctRun(r, s, nil, write(i)...), "-OOM")
			}
			if !refused {
				t.Fatalf("2 MB of %s writes never refused against a %d byte limit (usage %d)", name, limit, s.MemoryTracker().Usage())
			}
			if u := s.MemoryTracker().Usage(); u > limit+2048 {
				t.Fatalf("usage %d far over the limit %d", u, limit)
			}
			checkMemAccounting(t, s, name)
		})
	}
}

// With an eviction policy a write over the limit evicts other keys instead
// of being refused.
func TestMaxmemoryEvictsForInPlaceGrowth(t *testing.T) {
	const limit = 256 << 10
	r := newMemAcctRouter()
	s := store.NewStore()
	s.ConfigureMemory(limit, store.EvictionAllKeysLRU, 70, 85, 5)
	val := strings.Repeat("x", 512)
	for i := 0; i < 4000; i++ {
		key := "list" + strconv.Itoa(i%50)
		if out := memAcctRun(r, s, nil, "RPUSH", key, val); strings.HasPrefix(out, "-") {
			t.Fatalf("RPUSH %d refused under allkeys-lru: %q", i, out)
		}
	}
	if u := s.MemoryTracker().Usage(); u > limit+4096 {
		t.Fatalf("usage %d over the limit %d despite eviction", u, limit)
	}
	checkMemAccounting(t, s, "eviction")
}

// Redis refuses a denyoom command queued in MULTI, which discards the
// transaction, and refuses a script's first write.
func TestMaxmemoryRefusalInTransactionsAndScripts(t *testing.T) {
	r := newMemAcctRouter()
	s := store.NewStore()
	s.ConfigureMemory(64<<10, store.EvictionNoEviction, 70, 85, 5)
	val := strings.Repeat("x", 1024)
	for i := 0; i < 200; i++ {
		memAcctRun(r, s, nil, "RPUSH", "big", val)
	}

	tx := NewTransaction()
	memAcctRun(r, s, tx, "MULTI")
	if out := memAcctRun(r, s, tx, "RPUSH", "big", "v"); !strings.HasPrefix(out, "-OOM") {
		t.Fatalf("queueing RPUSH over the limit: %q", out)
	}
	if out := memAcctRun(r, s, tx, "EXEC"); !strings.HasPrefix(out, "-EXECABORT") {
		t.Fatalf("EXEC after a refused command: %q", out)
	}

	out := memAcctRun(r, s, nil, "EVAL", "return redis.pcall('RPUSH', KEYS[1], 'v')", "1", "big")
	if !strings.Contains(out, "OOM command not allowed") {
		t.Fatalf("script write over the limit: %q", out)
	}
	if out := memAcctRun(r, s, nil, "EVAL", "return redis.call('LLEN', KEYS[1])", "1", "big"); !strings.HasPrefix(out, ":") {
		t.Fatalf("script read over the limit: %q", out)
	}

	// A transaction queued before the limit was crossed is refused at EXEC.
	memAcctRun(r, s, nil, "DEL", "big")
	tx = NewTransaction()
	memAcctRun(r, s, tx, "MULTI")
	memAcctRun(r, s, tx, "RPUSH", "big", "v")
	for i := 0; i < 200; i++ {
		memAcctRun(r, s, nil, "RPUSH", "other", val)
	}
	if out := memAcctRun(r, s, tx, "EXEC"); !strings.HasPrefix(out, "-EXECABORT Transaction discarded because of: OOM") {
		t.Fatalf("EXEC of a denyoom transaction over the limit: %q", out)
	}
}

// RPUSH onto a 200k-element list must cost the same as onto a short one:
// accounting a write may not walk the collection.
func BenchmarkRPUSHOnLargeList(b *testing.B) {
	for _, size := range []int{1000, 200_000} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			r := newMemAcctRouter()
			s := store.NewStore()
			s.ConfigureMemory(1<<40, store.EvictionNoEviction, 70, 85, 5)
			for i := 0; i < size; i += 1000 {
				args := []string{"RPUSH", "list"}
				for j := 0; j < 1000; j++ {
					args = append(args, "element-value")
				}
				memAcctRun(r, s, nil, args...)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				memAcctRun(r, s, nil, "RPUSH", "list", "element-value")
			}
		})
	}
}

// Extension registries live outside the keyspace, so their growth is refused
// over maxmemory like a denyoom command, and also when the live heap is past
// the limit; commands that read or remove state still run.
func TestMaxmemoryRefusesGrowingExtensionCommands(t *testing.T) {
	r := newMemAcctRouter()
	RegisterEventCommands(r)
	s := store.NewStore()
	s.ConfigureMemory(64<<10, store.EvictionNoEviction, 70, 85, 5)
	heapSample.live.Store(0)
	heapSample.at.Store(time.Now().UnixNano() + int64(time.Hour))
	t.Cleanup(func() { heapSample.at.Store(0) })
	memAcctRun(r, s, nil, "QUEUE.CLEAR", "memq")
	memAcctRun(r, s, nil, "QUEUE.PUSH", "memq", "a")

	val := strings.Repeat("x", 1024)
	for i := 0; i < 200; i++ {
		memAcctRun(r, s, nil, "RPUSH", "big", val)
	}
	if out := memAcctRun(r, s, nil, "QUEUE.PUSH", "memq", "b"); !strings.HasPrefix(out, "-OOM") {
		t.Fatalf("QUEUE.PUSH over maxmemory: %q", out)
	}
	if out := memAcctRun(r, s, nil, "QUEUE.LEN", "memq"); out != ":1\r\n" {
		t.Fatalf("QUEUE.LEN over maxmemory: %q", out)
	}
	if out := memAcctRun(r, s, nil, "QUEUE.POP", "memq"); !strings.HasPrefix(out, "$1") {
		t.Fatalf("QUEUE.POP over maxmemory: %q", out)
	}

	memAcctRun(r, s, nil, "DEL", "big")
	if out := memAcctRun(r, s, nil, "QUEUE.PUSH", "memq", "c"); out != ":1\r\n" {
		t.Fatalf("QUEUE.PUSH under maxmemory: %q", out)
	}
	heapSample.live.Store(1 << 40)
	if out := memAcctRun(r, s, nil, "QUEUE.PUSH", "memq", "d"); !strings.HasPrefix(out, "-OOM") {
		t.Fatalf("QUEUE.PUSH with the heap past the limit: %q", out)
	}
	if out := memAcctRun(r, s, nil, "SET", "k", "v"); out != "+OK\r\n" {
		t.Fatalf("SET is bounded by used_memory only: %q", out)
	}
	memAcctRun(r, s, nil, "QUEUE.CLEAR", "memq")
}

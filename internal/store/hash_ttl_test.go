package store

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"
)

func hashWithFields(fields ...string) *HashValue {
	h := &HashValue{Fields: make(map[string][]byte)}
	for _, f := range fields {
		h.Fields[f] = []byte("v-" + f)
	}
	return h
}

// fieldExpiryRecorder captures OnHashFieldsExpiredLocked calls.
type fieldExpiryRecorder struct {
	mu     sync.Mutex
	fields map[string][]string
}

func recordFieldExpiry(s *Store) *fieldExpiryRecorder {
	r := &fieldExpiryRecorder{fields: make(map[string][]string)}
	s.SetHooks(StoreHooks{OnHashFieldsExpiredLocked: func(_ *Store, key string, fields []string) {
		r.mu.Lock()
		r.fields[key] = append(r.fields[key], fields...)
		r.mu.Unlock()
	}})
	return r
}

func (r *fieldExpiryRecorder) get(key string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]string(nil), r.fields[key]...)
	sort.Strings(out)
	return out
}

func setFieldTTL(t *testing.T, s *Store, key string, at int64, fields ...string) {
	t.Helper()
	entry, ok := s.shards[s.shardIndex(key)].Get(key)
	if !ok {
		t.Fatalf("%s missing", key)
	}
	h := entry.Value.(*HashValue)
	h.Lock()
	for _, f := range fields {
		h.SetFieldExpireAt(f, at)
	}
	h.Unlock()
	s.ScheduleHashFieldExpiry(key, h)
	s.Reaccount(key)
}

func TestHashFieldTTLBookkeeping(t *testing.T) {
	h := hashWithFields("a", "b", "c")
	base := h.SizeOf()
	if h.HasFieldTTLs() || h.FieldTTLsUsed() || h.FieldExpiries() != nil {
		t.Fatal("a new hash has no field expiries")
	}
	h.Lock()
	h.SetFieldExpireAt("a", 3000)
	h.SetFieldExpireAt("b", 2000)
	h.SetFieldExpireAt("missing", 1000)
	h.Unlock()
	if got := h.NextFieldExpiry(); got != 2000 {
		t.Fatalf("NextFieldExpiry = %d, want 2000", got)
	}
	if got := h.SizeOf(); got != base+2*hashFieldTTLOverhead {
		t.Fatalf("SizeOf = %d, want %d", got, base+2*hashFieldTTLOverhead)
	}
	if h.FieldExpireAt("missing") != 0 {
		t.Fatal("a missing field cannot get an expiry")
	}

	h.Lock()
	if !h.PersistField("b") || h.PersistField("b") || h.PersistField("c") {
		t.Fatal("PersistField reports whether the field had an expiry")
	}
	h.Unlock()
	if got := h.NextFieldExpiry(); got != 3000 {
		t.Fatalf("NextFieldExpiry after persist = %d, want 3000", got)
	}

	h.Lock()
	h.SetFieldKeepTTL("a", []byte("kept"))
	keep := h.FieldExpireAt("a")
	h.SetField("a", []byte("cleared"))
	cleared := h.FieldExpireAt("a")
	h.Unlock()
	if keep != 3000 || cleared != 0 {
		t.Fatalf("SetFieldKeepTTL kept %d, SetField left %d; want 3000 and 0", keep, cleared)
	}
	if h.HasFieldTTLs() || !h.FieldTTLsUsed() {
		t.Fatal("no field has an expiry any more, but the hash used them")
	}

	h.Lock()
	h.SetFieldExpireAt("c", 5000)
	h.DeleteField("c")
	h.Unlock()
	if h.HasFieldTTLs() {
		t.Fatal("deleting a field drops its expiry")
	}

	var recount HashValue
	recount.Fields = h.Fields
	if got, want := h.SizeOf(), recount.SizeOf(); got != want {
		t.Fatalf("incremental SizeOf %d != recount %d", got, want)
	}
}

func TestHashFieldTTLHeapStaysBounded(t *testing.T) {
	h := hashWithFields("a")
	h.Lock()
	for i := 1; i <= 10000; i++ {
		h.SetFieldExpireAt("a", int64(1_000_000+i))
	}
	n := len(h.ttl.heap)
	h.Unlock()
	if n > 64 {
		t.Fatalf("heap holds %d entries for one field", n)
	}
	if got := h.NextFieldExpiry(); got != 1_010_000 {
		t.Fatalf("NextFieldExpiry = %d", got)
	}
}

func TestHashFieldExpireDueFieldsOrderAndLimit(t *testing.T) {
	h := hashWithFields("a", "b", "c", "d")
	h.Lock()
	h.SetFieldExpireAt("a", 10)
	h.SetFieldExpireAt("b", 20)
	h.SetFieldExpireAt("c", 30)
	h.SetFieldExpireAt("d", 100)
	got := h.ExpireDueFields(25, 1)
	got2 := h.ExpireDueFields(50, 0)
	h.Unlock()
	if fmt.Sprint(got) != "[a]" || fmt.Sprint(got2) != "[b c]" {
		t.Fatalf("expired %v then %v", got, got2)
	}
	if len(h.Fields) != 1 || h.NextFieldExpiry() != 100 {
		t.Fatalf("left %v, next %d", h.Fields, h.NextFieldExpiry())
	}
}

func TestHashFieldCloneCopiesTTLs(t *testing.T) {
	h := hashWithFields("a", "b")
	h.Lock()
	h.SetFieldExpireAt("a", 4102444800000)
	h.Unlock()
	c := h.Clone().(*HashValue)
	if c.FieldExpireAt("a") != 4102444800000 || c.NextFieldExpiry() != 4102444800000 {
		t.Fatal("Clone must copy field expiries")
	}
	c.Lock()
	c.PersistField("a")
	c.Unlock()
	if h.FieldExpireAt("a") == 0 {
		t.Fatal("the clone's expiries must be its own")
	}
}

func TestHashFieldLazyExpiryOnRead(t *testing.T) {
	s := NewStore()
	rec := recordFieldExpiry(s)
	if err := s.Set("h", hashWithFields("a", "b", "c"), SetOptions{}); err != nil {
		t.Fatal(err)
	}
	past := time.Now().UnixMilli() - 1
	setFieldTTL(t, s, "h", time.Now().Add(time.Hour).UnixMilli(), "c")
	setFieldTTL(t, s, "h", past, "a", "b")

	before := GlobalMetrics.ExpiredSubkeys.Load()
	entry, ok := s.Get("h")
	if !ok {
		t.Fatal("h must still exist")
	}
	h := entry.Value.(*HashValue)
	h.RLock()
	n := len(h.Fields)
	h.RUnlock()
	if n != 1 {
		t.Fatalf("read saw %d fields, want 1", n)
	}
	if got := rec.get("h"); fmt.Sprint(got) != "[a b]" {
		t.Fatalf("expired fields announced: %v", got)
	}
	if GlobalMetrics.ExpiredSubkeys.Load()-before != 2 {
		t.Fatal("expired_subkeys must count the two fields")
	}
	checkMemUsage(t, s)

	setFieldTTL(t, s, "h", past, "c")
	if s.Exists("h") {
		t.Fatal("the key goes away with its last field")
	}
	if s.KeyCount() != 0 || s.MemUsage() != 0 {
		t.Fatalf("keys %d, memory %d after the last field expired", s.KeyCount(), s.MemUsage())
	}
	if !h.IsDead() {
		t.Fatal("the emptied hash must be marked dead")
	}
}

func checkMemUsage(t *testing.T, s *Store) {
	t.Helper()
	var want int64
	for _, sh := range &s.shards {
		sh.mu.RLock()
		for k, e := range sh.data {
			want += footprint(k, e)
		}
		sh.mu.RUnlock()
	}
	if got := s.MemUsage(); got != want {
		t.Fatalf("MemUsage %d, footprint of the entries %d", got, want)
	}
}

func TestHashFieldWriteTreatsExpiredFieldAsAbsent(t *testing.T) {
	s := NewStore()
	if err := s.Set("h", hashWithFields("a", "b"), SetOptions{}); err != nil {
		t.Fatal(err)
	}
	setFieldTTL(t, s, "h", time.Now().UnixMilli()-1, "a")
	added, err := s.ApplyHashPairs("h", true, [][]byte{[]byte("a"), []byte("new")})
	if err != nil || added != 1 {
		t.Fatalf("HSETNX on an expired field: added %d, err %v", added, err)
	}
	entry, _ := s.Get("h")
	h := entry.Value.(*HashValue)
	h.RLock()
	defer h.RUnlock()
	if string(h.Fields["a"]) != "new" || h.FieldExpireAt("a") != 0 {
		t.Fatalf("a = %q with expiry %d", h.Fields["a"], h.FieldExpireAt("a"))
	}
}

func TestHashFieldWriteToFullyExpiredHashRecreatesIt(t *testing.T) {
	s := NewStore()
	if err := s.Set("h", hashWithFields("a"), SetOptions{}); err != nil {
		t.Fatal(err)
	}
	entry, _ := s.Get("h")
	old := entry.Value.(*HashValue)
	old.Lock()
	old.SetFieldExpireAt("a", time.Now().UnixMilli()-1)
	old.Unlock()
	if _, err := s.ApplyHashPairs("h", false, [][]byte{[]byte("b"), []byte("1")}); err != nil {
		t.Fatal(err)
	}
	entry, _ = s.Get("h")
	h := entry.Value.(*HashValue)
	if h == old || len(h.Fields) != 1 {
		t.Fatalf("the write must create a fresh hash, got %v", h.Fields)
	}
	checkMemUsage(t, s)
}

func TestHashFieldPassiveStoreHidesWithoutDeleting(t *testing.T) {
	s := NewStore()
	rec := recordFieldExpiry(s)
	if err := s.Set("h", hashWithFields("a", "b"), SetOptions{}); err != nil {
		t.Fatal(err)
	}
	setFieldTTL(t, s, "h", time.Now().UnixMilli()-1, "a")
	s.passiveExpiry.Store(true)
	entry, ok := s.Get("h")
	if !ok {
		t.Fatal("h must exist")
	}
	view := entry.Value.(*HashValue)
	if _, ok := view.Fields["a"]; ok || len(view.Fields) != 1 {
		t.Fatalf("a replica must hide the expired field: %v", view.Fields)
	}
	if len(rec.get("h")) != 0 {
		t.Fatal("a replica must not expire fields itself")
	}
	s.loading.Store(true)
	entry, _ = s.Get("h")
	if len(entry.Value.(*HashValue).Fields) != 2 {
		t.Fatal("the master's stream sees the field until its HDEL arrives")
	}
	s.loading.Store(false)

	setFieldTTL(t, s, "h", time.Now().UnixMilli()-1, "b")
	if _, ok := s.Get("h"); ok {
		t.Fatal("a hash whose every field expired reads as missing on a replica")
	}
	if s.KeyCount() != 1 {
		t.Fatal("the replica keeps the key until the master deletes it")
	}
}

func TestHashFieldActiveExpiryWithoutReads(t *testing.T) {
	s := NewStore()
	rec := recordFieldExpiry(s)
	s.StartExpiry()
	defer s.StopExpiry()
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("h%d", i)
		if err := s.Set(key, hashWithFields("gone", "kept"), SetOptions{}); err != nil {
			t.Fatal(err)
		}
		setFieldTTL(t, s, key, time.Now().Add(50*time.Millisecond).UnixMilli(), "gone")
	}
	if err := s.Set("all", hashWithFields("x", "y"), SetOptions{}); err != nil {
		t.Fatal(err)
	}
	setFieldTTL(t, s, "all", time.Now().Add(50*time.Millisecond).UnixMilli(), "x", "y")
	if err := s.Set("later", hashWithFields("x"), SetOptions{}); err != nil {
		t.Fatal(err)
	}
	setFieldTTL(t, s, "later", time.Now().Add(time.Hour).UnixMilli(), "x")

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && (len(rec.get("h49")) == 0 || s.KeyCount() != 51) {
		time.Sleep(20 * time.Millisecond)
	}
	for i := 0; i < 50; i++ {
		if got := rec.get(fmt.Sprintf("h%d", i)); fmt.Sprint(got) != "[gone]" {
			t.Fatalf("h%d: expired %v", i, got)
		}
	}
	if s.KeyCount() != 51 {
		t.Fatalf("keys = %d, want 51: the fully expired hash is removed", s.KeyCount())
	}
	if s.HashFieldExpiryScheduled() != 1 {
		t.Fatalf("index lists %d keys, want only \"later\"", s.HashFieldExpiryScheduled())
	}
	checkMemUsage(t, s)
}

func TestHashFieldActiveExpiryIsBounded(t *testing.T) {
	s := NewStore()
	if err := s.Set("big", hashWithFields(), SetOptions{}); err != nil {
		t.Fatal(err)
	}
	entry, _ := s.Get("big")
	h := entry.Value.(*HashValue)
	past := time.Now().UnixMilli() - 1
	h.Lock()
	for i := 0; i < 3*hashFieldActiveBudget; i++ {
		f := fmt.Sprintf("f%d", i)
		h.Fields[f] = []byte("v")
		h.SetFieldExpireAt(f, past)
	}
	h.Unlock()
	s.ScheduleHashFieldExpiry("big", h)
	s.activeExpireHashFields(time.Now().UnixMilli())
	h.RLock()
	left := len(h.Fields)
	h.RUnlock()
	if left != 2*hashFieldActiveBudget {
		t.Fatalf("one cycle left %d fields, want %d", left, 2*hashFieldActiveBudget)
	}
	s.activeExpireHashFields(time.Now().UnixMilli())
	s.activeExpireHashFields(time.Now().UnixMilli())
	if s.Exists("big") {
		t.Fatal("three cycles expire every field")
	}
}

func TestHashFieldIndexFollowsRenameAndFlush(t *testing.T) {
	s := NewStore()
	if err := s.Set("h", hashWithFields("a"), SetOptions{}); err != nil {
		t.Fatal(err)
	}
	setFieldTTL(t, s, "h", time.Now().Add(time.Hour).UnixMilli(), "a")
	if _, err := s.Transfer("h", s, "h2", TransferOptions{}); err != nil {
		t.Fatal(err)
	}
	s.hfe.mu.Lock()
	_, listed := s.hfe.sched["h2"]
	s.hfe.mu.Unlock()
	if !listed {
		t.Fatal("a renamed hash must be listed under its new name")
	}
	s.Flush()
	if s.HashFieldExpiryScheduled() != 0 {
		t.Fatal("Flush empties the index")
	}
}

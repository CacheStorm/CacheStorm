package server

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func tagServer(t *testing.T, keys ...string) (*Server, *command.Router) {
	t.Helper()
	s := &Server{store: store.NewStore()}
	for _, k := range keys {
		if err := s.store.Set(k, &store.StringValue{Data: []byte("v")}, store.SetOptions{Tags: []string{"t"}}); err != nil {
			t.Fatal(err)
		}
	}
	r := command.NewRouter()
	command.RegisterTagCommands(r)
	return s, r
}

// keyOffStripes returns a key whose stripe is none of those of avoid.
func keyOffStripes(prefix string, avoid ...string) string {
	used := map[int]bool{}
	for _, k := range avoid {
		used[stripeOf(k)] = true
	}
	for i := 0; ; i++ {
		k := fmt.Sprintf("%s%d", prefix, i)
		if !used[stripeOf(k)] {
			return k
		}
	}
}

func TestInvalidateTakesTagKeyStripesX(t *testing.T) {
	s, r := tagServer(t, "k1", "k2")
	ctx := command.NewContext("INVALIDATE", [][]byte{[]byte("t")}, s.store, resp.NewWriter(&bytes.Buffer{}))
	unlock := s.holdGate(ctx)
	other := keyOffStripes("o", "k1", "k2", "t")
	if !s.gate.stripes[stripeOf(other)].TryLock() {
		t.Fatal("INVALIDATE of a two-key tag holds the stripe of an unrelated key")
	}
	s.gate.stripes[stripeOf(other)].Unlock()
	if s.gate.stripes[stripeOf("k1")].TryLock() {
		t.Fatal("INVALIDATE does not hold the stripe of a key of its tag")
	}
	if err := r.Execute(ctx); err != nil {
		t.Fatal(err)
	}
	unlock()
	if s.store.Exists("k1") || s.store.Exists("k2") {
		t.Fatal("tagged keys survived INVALIDATE")
	}
	cmds, set := ctx.Propagation()
	if !set || len(cmds) != 1 || cmds[0].Name != "DEL" {
		t.Fatalf("propagation=%+v set=%v, want one DEL", cmds, set)
	}
	var got []string
	for _, a := range cmds[0].Args {
		got = append(got, string(a))
	}
	sort.Strings(got)
	if strings.Join(got, ",") != "k1,k2" {
		t.Fatalf("DEL args=%v", got)
	}
}

func TestInvalidateEscalatesWhenTagGainsKeyX(t *testing.T) {
	s, r := tagServer(t, "k1")
	ctx := command.NewContext("INVALIDATE", [][]byte{[]byte("t")}, s.store, resp.NewWriter(&bytes.Buffer{}))
	unlock := s.holdGate(ctx)
	late := keyOffStripes("late", "k1", "t")
	// The late key joins the tag after the hold was planned, on a stripe the
	// hold does not cover.
	if err := s.store.Set(late, &store.StringValue{Data: []byte("v")}, store.SetOptions{Tags: []string{"t"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Execute(ctx); err != nil {
		t.Fatal(err)
	}
	if s.gate.global.TryRLock() {
		s.gate.global.RUnlock()
		t.Fatal("INVALIDATE did not take the whole gate for a key it had not locked")
	}
	unlock()
	if s.store.Exists("k1") || s.store.Exists(late) {
		t.Fatal("tagged keys survived INVALIDATE")
	}
	if got := delArgs(t, ctx); got != "k1,"+late {
		t.Fatalf("DEL args=%s", got)
	}
}

func TestInvalidateReplicaFollowsConcurrentTaggingX(t *testing.T) {
	master := startReplServer(t, nil)
	replica := startReplServer(t, nil)
	rc := replicate(t, replica, master)
	dialRepl(t, master).do("TAGLINK", "u0", "u1")
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			c := dialRepl(t, master)
			for i := 0; i < 200; i++ {
				c.do("SETTAG", fmt.Sprintf("w%d:%d", w, i%20), "v", "t", fmt.Sprintf("u%d", i%3))
				if i%5 == 0 {
					c.do("INVALIDATE", "t")
				}
				if i%7 == 0 {
					c.do("INVALIDATE", fmt.Sprintf("u%d", i%3))
				}
				if i%11 == 0 {
					c.do("INVALIDATE", "u0", "CASCADE")
				}
			}
		}(w)
	}
	wg.Wait()
	waitSynced(t, replica, master)
	mc := dialRepl(t, master)
	want, got := dumpAll(t, mc), dumpAll(t, rc)
	if fmt.Sprint(want) != fmt.Sprint(got) {
		t.Fatalf("replica diverged\nmaster:  %v\nreplica: %v", want, got)
	}
}

func TestInvalidateAbortsWatchersX(t *testing.T) {
	s := startReplServer(t, nil)
	a, b := dialRepl(t, s), dialRepl(t, s)
	a.do("SETTAG", "k", "v", "t")
	a.do("SETTAG", "other", "v", "u")
	a.do("WATCH", "k", "other")
	if got := b.do("INVALIDATE", "t"); got.Type != resp.TypeInteger || got.Int != 1 {
		t.Fatalf("INVALIDATE=%+v", got)
	}
	a.do("MULTI")
	a.do("SET", "x", "1")
	if got := a.do("EXEC"); !got.IsNull {
		t.Fatalf("EXEC after INVALIDATE of a watched key=%+v, want nil", got)
	}
	a.do("WATCH", "other")
	b.do("INVALIDATE", "t")
	a.do("MULTI")
	a.do("SET", "x", "1")
	if got := a.do("EXEC"); got.IsNull {
		t.Fatal("INVALIDATE of a tag without the watched key aborted the transaction")
	}
}

func delArgs(t *testing.T, ctx *command.Context) string {
	t.Helper()
	cmds, set := ctx.Propagation()
	if !set || len(cmds) != 1 || cmds[0].Name != "DEL" {
		t.Fatalf("propagation=%+v set=%v, want one DEL", cmds, set)
	}
	var got []string
	for _, a := range cmds[0].Args {
		got = append(got, string(a))
	}
	sort.Strings(got)
	return strings.Join(got, ",")
}

func TestInvalidateCascadeTakesStripesX(t *testing.T) {
	s, r := tagServer(t, "a")
	if err := s.store.Set("b", &store.StringValue{Data: []byte("v")}, store.SetOptions{Tags: []string{"c"}}); err != nil {
		t.Fatal(err)
	}
	s.store.GetTagIndex().Link("t", "c")
	ctx := command.NewContext("INVALIDATE", [][]byte{[]byte("t"), []byte("CASCADE")}, s.store, resp.NewWriter(&bytes.Buffer{}))
	unlock := s.holdGate(ctx)
	other := keyOffStripes("o", "a", "b", "t", "c")
	if !s.gate.stripes[stripeOf(other)].TryLock() {
		t.Fatal("INVALIDATE CASCADE of small tags holds the stripe of an unrelated key")
	}
	s.gate.stripes[stripeOf(other)].Unlock()
	if s.gate.stripes[stripeOf("b")].TryLock() {
		t.Fatal("INVALIDATE CASCADE does not hold the stripe of a child tag's key")
	}
	if err := r.Execute(ctx); err != nil {
		t.Fatal(err)
	}
	unlock()
	if s.store.Exists("a") || s.store.Exists("b") {
		t.Fatal("keys survived INVALIDATE CASCADE")
	}
	if got := delArgs(t, ctx); got != "a,b" {
		t.Fatalf("DEL args=%s", got)
	}
}

func TestInvalidateCascadeEscalatesMidwayX(t *testing.T) {
	s, r := tagServer(t, "a")
	s.store.GetTagIndex().Link("t", "c")
	ctx := command.NewContext("INVALIDATE", [][]byte{[]byte("t"), []byte("CASCADE")}, s.store, resp.NewWriter(&bytes.Buffer{}))
	unlock := s.holdGate(ctx)
	late := keyOffStripes("late", "a", "t", "c")
	if err := s.store.Set(late, &store.StringValue{Data: []byte("v")}, store.SetOptions{Tags: []string{"c"}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Execute(ctx); err != nil {
		t.Fatal(err)
	}
	if s.gate.global.TryRLock() {
		s.gate.global.RUnlock()
		t.Fatal("INVALIDATE CASCADE did not take the whole gate for a key it had not locked")
	}
	unlock()
	if s.store.Exists("a") || s.store.Exists(late) {
		t.Fatal("keys survived INVALIDATE CASCADE")
	}
	// Part of it ran beside other writes, so it replicates as the deletes.
	if got := delArgs(t, ctx); got != "a,"+late {
		t.Fatalf("DEL args=%s", got)
	}
}

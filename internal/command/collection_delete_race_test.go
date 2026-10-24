package command

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func collectionRaceRouter() *Router {
	r := NewRouter()
	RegisterListCommands(r)
	RegisterHashCommands(r)
	RegisterSetCommands(r)
	RegisterSortedSetCommands(r)
	RegisterStreamCommands(r)
	return r
}

func collectionRaceRun(r *Router, s *store.Store, args ...string) string {
	var out bytes.Buffer
	argv := make([][]byte, len(args)-1)
	for i, a := range args[1:] {
		argv[i] = []byte(a)
	}
	ctx := NewContext(args[0], argv, s, resp.NewWriter(&out))
	if err := r.Execute(ctx); err != nil {
		ctx.Writer.WriteError(err.Error())
	}
	return out.String()
}

func collectionRaceInt(t *testing.T, reply string) int {
	t.Helper()
	if !strings.HasPrefix(reply, ":") {
		t.Fatalf("expected an integer reply, got %q", reply)
	}
	n, err := strconv.Atoi(strings.TrimSuffix(reply[1:], "\r\n"))
	if err != nil {
		t.Fatalf("bad integer reply %q", reply)
	}
	return n
}

// A pop that emptied a list deleted the key after releasing the list's lock.
// A push that had already looked the list up appended in between and its
// elements were deleted with the key, or appended after the delete to a list
// no longer stored. Either way a pushed element was neither popped nor left.
func TestConcurrentPushPopLosesNothing(t *testing.T) {
	r := collectionRaceRouter()
	s := store.NewStore()
	const pushers, perPusher = 4, 5000
	var popped atomic.Int64
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if out := collectionRaceRun(r, s, "LPOP", "q"); out[0] == '$' && out != "$-1\r\n" {
					popped.Add(1)
				}
			}
		}()
	}
	var pw sync.WaitGroup
	for p := 0; p < pushers; p++ {
		pw.Add(1)
		go func() {
			defer pw.Done()
			for i := 0; i < perPusher; i++ {
				collectionRaceRun(r, s, "RPUSH", "q", strconv.Itoa(i))
			}
		}()
	}
	pw.Wait()
	close(stop)
	wg.Wait()
	left := collectionRaceInt(t, collectionRaceRun(r, s, "LLEN", "q"))
	if got := int(popped.Load()) + left; got != pushers*perPusher {
		t.Fatalf("pushed %d, popped %d, left %d: lost %d elements",
			pushers*perPusher, popped.Load(), left, pushers*perPusher-got)
	}
}

// addRemoveRace runs writers adding distinct members to one key while
// removers delete members already added, so the key empties and is deleted
// over and over. Every member added is either removed (reply 1) or still
// present at the end.
func addRemoveRace(t *testing.T, add func(member string) []string, remove func(member string) []string, card []string) {
	t.Helper()
	r := collectionRaceRouter()
	s := store.NewStore()
	const writers, perWriter = 4, 3000
	added := make(chan string, writers*perWriter)
	var removed atomic.Int64
	var rw sync.WaitGroup
	for w := 0; w < 4; w++ {
		rw.Add(1)
		go func() {
			defer rw.Done()
			for m := range added {
				if out := collectionRaceRun(r, s, remove(m)...); out == ":1\r\n" {
					removed.Add(1)
				}
			}
		}()
	}
	var ww sync.WaitGroup
	for w := 0; w < writers; w++ {
		ww.Add(1)
		go func(w int) {
			defer ww.Done()
			for i := 0; i < perWriter; i++ {
				m := fmt.Sprintf("m%d-%d", w, i)
				if out := collectionRaceRun(r, s, add(m)...); out != ":1\r\n" {
					t.Errorf("%v replied %q", add(m), out)
					return
				}
				if i%2 == 0 {
					added <- m
				}
			}
		}(w)
	}
	ww.Wait()
	close(added)
	rw.Wait()
	total := writers * perWriter
	wantRemoved := total / 2
	left := collectionRaceInt(t, collectionRaceRun(r, s, card...))
	if int(removed.Load()) != wantRemoved || left != total-wantRemoved {
		t.Fatalf("added %d, removed %d of %d, left %d of %d", total, removed.Load(), wantRemoved, left, total-wantRemoved)
	}
}

func TestConcurrentHSetHDelLosesNothing(t *testing.T) {
	addRemoveRace(t,
		func(m string) []string { return []string{"HSET", "h", m, "v"} },
		func(m string) []string { return []string{"HDEL", "h", m} },
		[]string{"HLEN", "h"})
}

func TestConcurrentSAddSRemLosesNothing(t *testing.T) {
	addRemoveRace(t,
		func(m string) []string { return []string{"SADD", "s", m} },
		func(m string) []string { return []string{"SREM", "s", m} },
		[]string{"SCARD", "s"})
}

func TestConcurrentZAddZRemLosesNothing(t *testing.T) {
	addRemoveRace(t,
		func(m string) []string { return []string{"ZADD", "z", "1", m} },
		func(m string) []string { return []string{"ZREM", "z", m} },
		[]string{"ZCARD", "z"})
}

// Creating a collection looked the key up and then stored a new value: two
// writers that both found the key missing both stored one, and the second
// replaced the first along with what the first had written.
func TestConcurrentFirstWritesToAFreshKeyLoseNothing(t *testing.T) {
	cases := []struct {
		name string
		add  func(key string, w int) []string
		card string
	}{
		{"RPUSH", func(k string, w int) []string { return []string{"RPUSH", k, strconv.Itoa(w)} }, "LLEN"},
		{"LPUSH", func(k string, w int) []string { return []string{"LPUSH", k, strconv.Itoa(w)} }, "LLEN"},
		{"LMPUSH", func(k string, w int) []string { return []string{"LMPUSH", k, strconv.Itoa(w)} }, "LLEN"},
		{"HSET", func(k string, w int) []string { return []string{"HSET", k, strconv.Itoa(w), "v"} }, "HLEN"},
		{"HINCRBY", func(k string, w int) []string { return []string{"HINCRBY", k, strconv.Itoa(w), "1"} }, "HLEN"},
		{"SADD", func(k string, w int) []string { return []string{"SADD", k, strconv.Itoa(w)} }, "SCARD"},
		{"ZADD", func(k string, w int) []string { return []string{"ZADD", k, "1", strconv.Itoa(w)} }, "ZCARD"},
		{"ZINCRBY", func(k string, w int) []string { return []string{"ZINCRBY", k, "1", strconv.Itoa(w)} }, "ZCARD"},
		{"XADD", func(k string, w int) []string { return []string{"XADD", k, "*", "w", strconv.Itoa(w)} }, "XLEN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := collectionRaceRouter()
			s := store.NewStore()
			const writers = 8
			for round := 0; round < 200; round++ {
				key := fmt.Sprintf("fresh%d", round)
				start := make(chan struct{})
				var wg sync.WaitGroup
				for w := 0; w < writers; w++ {
					wg.Add(1)
					go func(w int) {
						defer wg.Done()
						<-start
						collectionRaceRun(r, s, tc.add(key, w)...)
					}(w)
				}
				close(start)
				wg.Wait()
				if n := collectionRaceInt(t, collectionRaceRun(r, s, tc.card, key)); n != writers {
					t.Fatalf("round %d: %d concurrent writers created %s, %s = %d", round, writers, key, tc.card, n)
				}
			}
		})
	}
}

// SMOVE into a missing destination while the source is drained by SREM and
// refilled by SADD: every member ends in exactly one of the two sets.
func TestConcurrentSMoveLosesNothing(t *testing.T) {
	r := collectionRaceRouter()
	s := store.NewStore()
	const members = 3000
	var moved atomic.Int64
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := w; i < members; i += 4 {
				m := strconv.Itoa(i)
				collectionRaceRun(r, s, "SADD", "src", m)
				if collectionRaceRun(r, s, "SMOVE", "src", "dst", m) == ":1\r\n" {
					moved.Add(1)
				}
				if collectionRaceRun(r, s, "SMOVE", "dst", "src", m) == ":1\r\n" {
					moved.Add(-1)
				}
			}
		}(w)
	}
	wg.Wait()
	src := collectionRaceInt(t, collectionRaceRun(r, s, "SCARD", "src"))
	dst := collectionRaceInt(t, collectionRaceRun(r, s, "SCARD", "dst"))
	if src+dst != members || dst != int(moved.Load()) {
		t.Fatalf("src %d + dst %d != %d members (net moved %d)", src, dst, members, moved.Load())
	}
}

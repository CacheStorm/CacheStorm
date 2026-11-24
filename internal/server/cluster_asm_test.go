package server

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/hashslot"
)

// migrationStatus reads CLUSTER MIGRATION STATUS ID id on n.
func migrationStatus(n *busNode, id string) map[string]string {
	v := n.c.do("CLUSTER", "MIGRATION", "STATUS", "ID", id)
	if len(v.Array) != 1 {
		return nil
	}
	out := map[string]string{}
	pairs := v.Array[0].Array
	for i := 0; i+1 < len(pairs); i += 2 {
		out[str(pairs[i])] = str(pairs[i+1])
	}
	return out
}

func waitMigration(t *testing.T, n *busNode, id, state string) map[string]string {
	t.Helper()
	var st map[string]string
	eventually(t, 20*time.Second, "the slot migration task to be "+state, func() bool {
		st = migrationStatus(n, id)
		return st != nil && st["state"] == state
	})
	return st
}

// TestClusterAtomicSlotMigration moves slots 100-199 with Redis 8.4's
// CLUSTER MIGRATION IMPORT while a client keeps writing to them, and checks
// that no acknowledged write is lost, values and TTLs arrive, the source
// and its replica drop the keys, the destination's replica gets them, and
// the slots can move back.
func TestClusterAtomicSlotMigration(t *testing.T) {
	sc := startShardedCluster(t)
	src, dst := sc.masters[0], sc.masters[1]
	srcReplica, dstReplica := sc.replicas[0], sc.replicas[1]

	expect := func(n *busNode, want string, args ...string) {
		t.Helper()
		if got := n.c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	expect(dst, "ERR:ERR wrong number of arguments for 'cluster|migration' command", "CLUSTER", "MIGRATION", "IMPORT")
	expect(dst, "ERR:ERR wrong number of arguments for 'cluster|migration' command", "CLUSTER", "MIGRATION", "IMPORT", "100")
	expect(dst, "ERR:ERR start slot number 200 is greater than end slot number 100", "CLUSTER", "MIGRATION", "IMPORT", "200", "100")
	expect(dst, "ERR:ERR Invalid or out of range slot", "CLUSTER", "MIGRATION", "IMPORT", "0", "16384")
	expect(dst, "ERR:ERR Slot 150 specified multiple times", "CLUSTER", "MIGRATION", "IMPORT", "100", "199", "150", "160")
	expect(dst, "ERR:ERR this node is already the owner of the slot range", "CLUSTER", "MIGRATION", "IMPORT", "6000", "6001")
	expect(dst, "ERR:ERR slots belong to different source nodes", "CLUSTER", "MIGRATION", "IMPORT", "5000", "6000")
	expect(dstReplica, "ERR:ERR slot migration not allowed on replica.", "CLUSTER", "MIGRATION", "IMPORT", "100", "199")
	expect(dst, "ERR:ERR unknown argument", "CLUSTER", "MIGRATION", "LIST", "x")
	expect(dst, "0", "CLUSTER", "MIGRATION", "CANCEL", "ALL")
	expect(dst, "[]", "CLUSTER", "MIGRATION", "STATUS", "ALL")
	expect(dst, "ERR:WRONGPASS invalid internal password", "AUTH", "internal connection", "nope")
	outsider := dialRepl(t, src.s)
	if got := outsider.str("CLUSTER", "SYNCSLOTS", "SYNC", strings.Repeat("a", 40), "100", "199"); got != "ERR:ERR CLUSTER SYNCSLOTS subcommands are only allowed for internal clients" {
		t.Fatalf("SYNCSLOTS from a client: %s", got)
	}
	if info := dst.c.str("CLUSTER", "INFO"); !strings.Contains(info, "cluster_slot_migration_active_tasks:0") {
		t.Fatalf("CLUSTER INFO: %s", info)
	}

	// The data: every type, TTLs, and a key outside the range that stays.
	keys := map[string]int{}
	for slot := 100; slot < 200; slot += 7 {
		keys[keyInSlot(t, "asm", slot)] = slot
	}
	i := 0
	ttlKeys := map[string]bool{}
	for k := range keys {
		var cmd []string
		switch i % 5 {
		case 0:
			cmd = []string{"SET", k, "v" + k, "PX", "600000"}
			ttlKeys[k] = true
		case 1:
			cmd = []string{"HSET", k, "a", "1", "b", "2"}
		case 2:
			cmd = []string{"RPUSH", k, "x", "y", "z"}
		case 3:
			cmd = []string{"ZADD", k, "1", "a", "2", "b"}
		default:
			cmd = []string{"XADD", k, "1-1", "f", "v"}
		}
		if got := src.c.str(cmd...); strings.HasPrefix(got, "ERR:") {
			t.Fatalf("%v: %s", cmd, got)
		}
		i++
	}
	dumps := map[string]string{}
	for k := range keys {
		dumps[k] = src.c.str("DUMP", k)
	}
	stay := keyInSlot(t, "stay", 300)
	src.c.str("SET", stay, "here")
	// A tagged key: the snapshot's RESTORE carries its tags.
	tagged := keyInSlot(t, "tagged", 133)
	if got := src.c.str("SETTAG", tagged, "v", "asm-a", "asm-b"); got != "OK" {
		t.Fatalf("SETTAG: %s", got)
	}
	if got := src.c.str("FUNCTION", "LOAD", "#!lua name=asmlib\nredis.register_function('asmf', function() return 1 end)"); got != "asmlib" {
		t.Fatalf("FUNCTION LOAD: %s", got)
	}

	// A client keeps incrementing counters in the slots; every acknowledged
	// increment must be on the destination afterwards.
	counters := []string{keyInSlot(t, "ctr", 120), keyInSlot(t, "ctr", 180)}
	acked := make([]int, len(counters))
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w := dialRepl(t, src.s)
		for n := 0; ; n++ {
			select {
			case <-stop:
				return
			default:
			}
			j := n % len(counters)
			got := w.str("INCR", counters[j])
			if strings.HasPrefix(got, "ERR:MOVED") {
				return
			}
			if strings.HasPrefix(got, "ERR:") {
				t.Errorf("INCR: %s", got)
				return
			}
			acked[j]++
		}
	}()
	time.Sleep(50 * time.Millisecond)

	id := dst.c.str("CLUSTER", "MIGRATION", "IMPORT", "100", "149", "150", "199")
	if len(id) != 40 {
		t.Fatalf("IMPORT: %s", id)
	}
	expect(dst, "ERR:ERR another ASM task is already in progress", "CLUSTER", "MIGRATION", "IMPORT", "300", "301")
	st := waitMigration(t, dst, id, "completed")
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		close(stop)
		<-done
		t.Fatal("the writer never saw MOVED")
	}
	if st["operation"] != "import" || st["slots"] != "100-199" || st["source"] != src.id || st["dest"] != dst.id ||
		st["last_error"] != "" || st["write_pause_ms"] != "0" || st["start_time"] == "-1" || st["end_time"] == "-1" {
		t.Fatalf("import status: %v", st)
	}
	ms := waitMigration(t, src, id, "completed")
	if ms["operation"] != "migrate" || ms["dest"] != dst.id || ms["slots"] != "100-199" {
		t.Fatalf("migrate status: %v", ms)
	}
	if p, err := strconv.Atoi(ms["write_pause_ms"]); err != nil || p < 0 || p > 10000 {
		t.Fatalf("write_pause_ms: %v", ms)
	}

	// Every node sends the slots to the destination.
	for _, n := range append(sc.masters, sc.replicas...) {
		eventually(t, 10*time.Second, "the slots to move everywhere", func() bool {
			return n.c.str("GET", counters[0]) == moved(hashSlotOf(counters[0]), dst) || n == dst
		})
	}
	for j, k := range counters {
		if got := dst.c.str("GET", k); got != strconv.Itoa(acked[j]) {
			t.Fatalf("counter %s = %s on the destination, %d increments were acknowledged", k, got, acked[j])
		}
	}
	if acked[0]+acked[1] == 0 {
		t.Fatal("the writer acknowledged no increment")
	}
	for k := range keys {
		if got := dst.c.str("DUMP", k); got != dumps[k] {
			t.Fatalf("DUMP %s differs on the destination", k)
		}
	}
	for k := range ttlKeys {
		{
			if ttl := dst.c.str("PTTL", k); ttl == "-1" || ttl == "-2" {
				t.Fatalf("PTTL %s = %s on the destination", k, ttl)
			}
		}
	}
	expect(dst, "[[library_name asmlib engine LUA functions [[name asmf description <null> flags []]]]]", "FUNCTION", "LIST")
	expect(src, "here", "GET", stay)
	expect(dst, "[asm-a asm-b]", "TAGS", tagged)
	expect(dst, "1", "INVALIDATE", "asm-b")

	// The source and its replica dropped the slots' keys; the destination's
	// replica has them.
	countIn := func(n *busNode) int {
		total := 0
		for slot := 100; slot < 200; slot++ {
			c, _ := strconv.Atoi(n.c.str("CLUSTER", "COUNTKEYSINSLOT", strconv.Itoa(slot)))
			total += c
		}
		return total
	}
	want := len(keys) + len(counters)
	for _, n := range []*busNode{src, srcReplica} {
		eventually(t, 10*time.Second, "the source's slots to be trimmed", func() bool { return countIn(n) == 0 })
	}
	eventually(t, 10*time.Second, "the destination's replica to get the slots", func() bool { return countIn(dstReplica) == want })
	if got := countIn(dst); got != want {
		t.Fatalf("destination holds %d keys of the slots, want %d", got, want)
	}

	// The slots move back.
	back := src.c.str("CLUSTER", "MIGRATION", "IMPORT", "100", "199")
	if len(back) != 40 {
		t.Fatalf("IMPORT back: %s", back)
	}
	waitMigration(t, src, back, "completed")
	waitMigration(t, dst, back, "completed")
	eventually(t, 10*time.Second, "the slots to come back", func() bool {
		return dst.c.str("GET", counters[0]) == moved(hashSlotOf(counters[0]), src)
	})
	for j, k := range counters {
		if got := src.c.str("GET", k); got != strconv.Itoa(acked[j]) {
			t.Fatalf("counter %s = %s back on the source, want %d", k, got, acked[j])
		}
	}
	eventually(t, 10*time.Second, "the destination's slots to be trimmed", func() bool { return countIn(dst) == 0 })

	all := dst.c.do("CLUSTER", "MIGRATION", "STATUS", "ALL")
	if len(all.Array) != 2 || str(all.Array[0].Array[1]) != back || str(all.Array[1].Array[1]) != id {
		t.Fatalf("STATUS ALL: %s", str(all))
	}
	expect(dst, "0", "CLUSTER", "MIGRATION", "CANCEL", "ID", id)
	expect(dst, "[]", "CLUSTER", "MIGRATION", "STATUS", "ID", "nope")
	if info := src.c.str("CLUSTER", "INFO"); !strings.Contains(info, "cluster_slot_migration_active_tasks:0") {
		t.Fatalf("CLUSTER INFO: %s", info)
	}
}

func hashSlotOf(key string) int { return hashslot.Key(key) }

// TestTrimSlots covers TRIMSLOTS: it refuses the slots this node serves and
// deletes the keys of others, and the deletion reaches the replica.
func TestTrimSlots(t *testing.T) {
	sc := startShardedCluster(t)
	src, other := sc.masters[0], sc.masters[1]
	expect := func(n *busNode, want string, args ...string) {
		t.Helper()
		if got := n.c.str(args...); got != want {
			t.Fatalf("%v = %q, want %q", args, got, want)
		}
	}
	expect(src, "ERR:ERR wrong number of arguments for 'trimslots' command", "TRIMSLOTS", "RANGES", "1", "5")
	expect(src, "ERR:ERR missing ranges argument", "TRIMSLOTS", "RANGE", "1", "5", "6")
	expect(src, "ERR:ERR invalid number of ranges", "TRIMSLOTS", "RANGES", "2", "5", "6")
	expect(src, "ERR:ERR invalid number of ranges", "TRIMSLOTS", "RANGES", "0", "5", "6")
	expect(src, "ERR:ERR value is not an integer or out of range", "TRIMSLOTS", "RANGES", "x", "5", "6")
	expect(src, "ERR:ERR the slot 5 is served by this node", "TRIMSLOTS", "RANGES", "1", "5", "6")
	// Keys in a slot of another node, put there the way a broken migration
	// would leave them.
	k := keyInSlot(t, "orphan", 6000)
	askingOrphan(t, src, other, k)
	waitShardSynced(t, src, sc.replicas[0])
	expect(src, "1", "CLUSTER", "COUNTKEYSINSLOT", "6000")
	expect(src, "OK", "TRIMSLOTS", "RANGES", "1", "6000", "6000")
	expect(src, "0", "CLUSTER", "COUNTKEYSINSLOT", "6000")
	eventually(t, 10*time.Second, "the replica to trim", func() bool {
		return sc.replicas[0].c.str("CLUSTER", "COUNTKEYSINSLOT", "6000") == "0"
	})
	expect(other, "OK", "TRIMSLOTS", "RANGES", "1", "100", "200")
}

// askingOrphan writes key on n although another node serves its slot.
func askingOrphan(t *testing.T, n, owner *busNode, key string) {
	t.Helper()
	slot := strconv.Itoa(hashSlotOf(key))
	if got := n.c.str("CLUSTER", "SETSLOT", slot, "IMPORTING", owner.id); got != "OK" {
		t.Fatalf("SETSLOT IMPORTING: %s", got)
	}
	if got := askingDo(n.c, "SET", key, "v"); got != "OK" {
		t.Fatalf("ASKING SET: %s", got)
	}
	if got := n.c.str("CLUSTER", "SETSLOT", slot, "STABLE"); got != "OK" {
		t.Fatalf("SETSLOT STABLE: %s", got)
	}
}

// TestClusterMigrationCancel cancels an import the paused source holds up,
// and a later import of the same slots still completes.
func TestClusterMigrationCancel(t *testing.T) {
	sc := startShardedCluster(t)
	src, dst := sc.masters[0], sc.masters[1]
	k := keyInSlot(t, "cancel", 42)
	src.c.str("SET", k, "v")
	if got := src.c.str("CLIENT", "PAUSE", "60000", "ALL"); got != "OK" {
		t.Fatalf("CLIENT PAUSE: %s", got)
	}
	id := dst.c.str("CLUSTER", "MIGRATION", "IMPORT", "40", "45")
	eventually(t, 10*time.Second, "the import to wait on the source", func() bool {
		st := migrationStatus(dst, id)
		return st != nil && st["state"] == "handshake-reply"
	})
	if got := dst.c.str("CLUSTER", "MIGRATION", "CANCEL", "ALL"); got != "1" {
		t.Fatalf("CANCEL ALL: %s", got)
	}
	st := migrationStatus(dst, id)
	if st["state"] != "canceled" || st["last_error"] != "Cancelled due to user request (state: handshake-reply, rdb_channel_state: none)" || st["end_time"] == "-1" {
		t.Fatalf("cancelled task: %v", st)
	}
	if got := src.c.str("CLIENT", "UNPAUSE"); got != "OK" {
		t.Fatalf("CLIENT UNPAUSE: %s", got)
	}
	if got := dst.c.str("GET", k); got != moved(42, src) {
		t.Fatalf("GET on the destination after the cancel: %s", got)
	}
	again := dst.c.str("CLUSTER", "MIGRATION", "IMPORT", "40", "45")
	waitMigration(t, dst, again, "completed")
	if got := dst.c.str("GET", k); got != "v" {
		t.Fatalf("GET after the second import: %s", got)
	}
	all := dst.c.do("CLUSTER", "MIGRATION", "STATUS", "ALL")
	if len(all.Array) != 2 {
		t.Fatalf("STATUS ALL: %s", str(all))
	}
}

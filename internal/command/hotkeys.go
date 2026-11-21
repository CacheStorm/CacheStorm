package command

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// Redis 8.6's HOTKEYS: a sampling session that charges each command's CPU
// time and network bytes to the keys it names, and reports the top keys by
// each. Redis keeps the top keys in a Cuckoo Heavy Keeper sketch; this keeps
// count*10 candidates with the Space-Saving algorithm, which likewise keeps
// the heavy keys exactly and bounds the error of the rest.

const (
	hotkeysCPU = 1 << iota
	hotkeysNet
)

type hotkeyTopK struct {
	capacity int
	counts   map[string]uint64
	total    uint64
}

func newHotkeyTopK(capacity int) *hotkeyTopK {
	return &hotkeyTopK{capacity: capacity, counts: make(map[string]uint64, capacity)}
}

// add charges n to key; when the table is full the key takes the place, and
// inherits the count, of the lightest one, as Space-Saving does.
func (t *hotkeyTopK) add(key string, n uint64) {
	t.total += n
	if c, ok := t.counts[key]; ok {
		t.counts[key] = c + n
		return
	}
	if len(t.counts) < t.capacity {
		t.counts[key] = n
		return
	}
	var minKey string
	minCount := ^uint64(0)
	for k, c := range t.counts {
		if c < minCount {
			minKey, minCount = k, c
		}
	}
	delete(t.counts, minKey)
	t.counts[key] = minCount + n
}

type hotkeyCount struct {
	key   string
	count uint64
}

func (t *hotkeyTopK) top(k int) []hotkeyCount {
	out := make([]hotkeyCount, 0, len(t.counts))
	for key, c := range t.counts {
		if c > 0 {
			out = append(out, hotkeyCount{key, c})
		}
	}
	slices.SortFunc(out, func(a, b hotkeyCount) int {
		if a.count != b.count {
			if a.count > b.count {
				return -1
			}
			return 1
		}
		return strings.Compare(a.key, b.key)
	})
	return out[:min(k, len(out))]
}

type hotkeySession struct {
	active      bool
	metrics     int
	count       int
	durationMs  int64
	sampleRatio int
	slots       []int // sorted; nil is every slot
	start       int64 // unix ms
	cpuSys      float64
	cpuUser     float64
	cpu, net    *hotkeyTopK

	timeAll, timeSelected, timeSampled int64
	netAll, netSelected, netSampled    int64
	// cpuTime is the milliseconds spent updating the top keys, INFO's
	// hotkeys-cmd-cpu-time.
	cpuTime int64
}

var hotkeys struct {
	mu      sync.Mutex
	session *hotkeySession
	// active lets the command path skip the lock when no session runs.
	active atomic.Bool
}

// expireLocked stops a session whose DURATION has passed.
func (h *hotkeySession) expireLocked(now int64) {
	if h.active && h.durationMs > 0 {
		if elapsed := now - h.start; elapsed >= h.durationMs {
			h.active = false
			h.durationMs = elapsed
			hotkeys.active.Store(false)
		}
	}
}

// RecordHotkeys charges the command that just ran to its keys when a HOTKEYS
// session is tracking: its CPU time and the bytes of its request and reply
// (and of what it fed replicas).
func (ctx *Context) RecordHotkeys(nameLen, replyBytes int) {
	if !hotkeys.active.Load() || ctx.FromMaster || ctx.Replaying || ctx.EndTime.IsZero() {
		return
	}
	keys := hotkeyKeysOf(ctx.Command, ctx.Args)
	if len(keys) == 0 {
		return
	}
	var cpu uint64
	if d := ctx.EndTime.Sub(ctx.StartTime) - ctx.waited; d > 0 {
		cpu = uint64(d.Microseconds())
	}
	netBytes := requestBytes(nameLen, ctx.Args) + uint64(max(replyBytes, 0))
	if ctx.ClusterConn != nil {
		netBytes += ctx.ClusterConn.extraOut
	}
	slot := ctx.commandSlot() - 1
	recordHotkeyCall(keys, slot, cpu, netBytes)
}

// hotkeyKeysOf is the keys Redis' key specs find in a command, or for a
// command without specs (CacheStorm's own) the keys it is taken to name.
func hotkeyKeysOf(cmd string, args [][]byte) []string {
	if keys, ok := keysBySpecs(cmd, args); ok {
		return keys
	}
	if redisCommandMeta(cmd) != nil {
		return nil
	}
	return CommandKeys(cmd, args)
}

func recordHotkeyCall(keys []string, slot int, cpu, netBytes uint64) {
	hotkeys.mu.Lock()
	defer hotkeys.mu.Unlock()
	h := hotkeys.session
	if h == nil || !h.active {
		return
	}
	now := time.Now()
	h.expireLocked(now.UnixMilli())
	if !h.active {
		return
	}
	sampled := h.sampleRatio <= 1 || rand.Float64() < 1/float64(h.sampleRatio)
	selected := h.slots == nil
	if !selected && slot >= 0 {
		_, selected = slices.BinarySearch(h.slots, slot)
	}
	h.timeAll += int64(cpu)
	h.netAll += int64(netBytes)
	if selected {
		h.timeSelected += int64(cpu)
		h.netSelected += int64(netBytes)
		if sampled && h.sampleRatio > 1 {
			h.timeSampled += int64(cpu)
			h.netSampled += int64(netBytes)
		}
	}
	if !sampled || !selected {
		return
	}
	n := uint64(len(keys))
	for _, k := range keys {
		if h.cpu != nil {
			h.cpu.add(k, cpu/n)
		}
		if h.net != nil {
			h.net.add(k, netBytes/n)
		}
	}
	h.cpuTime += time.Since(now).Milliseconds()
}

const hotkeysHelp = `HOTKEYS <subcommand> [<arg> [value] [opt] ...]. Subcommands are:
START <METRICS count [CPU] [NET]> [COUNT k] [DURATION duration] [SAMPLE ratio] [SLOTS count slot...]
    Starts hotkeys tracking with specified metrics.
    * METRICS count [CPU] [NET]
        Specify count of metrics and choose amongst:
        - CPU: Track hotkeys by CPU time percentage
        - NET: Track hotkeys by network bytes percentage
    * COUNT k
        Specifies the value of K for the top-K hotkeys tracking. Default: 10
    * DURATION duration
        Specifies tracking duration in seconds. 0 means tracking will continue until manually stopped. Default: 0
    * SAMPLE ratio
        Keys are tracked with probability 1/ratio. Default: 1 (tracks every key)
    * SLOTS count slot...
        Specify which slots to track keys from. Only available in cluster mode. Default: empty (track all slots)
STOP
    Stop hotkeys tracking. Results are still available via GET
GET
    Get results from hotkeys tracking.
RESET
    Reset memory used for hotkeys tracking. Tracking must have been stopped.
    Results will no longer be available after this command.
HELP
    Print this help.`

// legacyHotkeysCall reports a call of CacheStorm's HOTKEYS [count]: no
// argument, or a number where Redis 8.6 expects a subcommand. Redis answers
// both with an error, so they keep their older meaning.
func legacyHotkeysCall(args [][]byte) bool {
	if len(args) == 0 {
		return true
	}
	if len(args) != 1 {
		return false
	}
	_, err := redisAtoi(string(args[0]))
	return err == nil
}

func hotkeysErr(msg string) error { return errors.New("ERR " + msg) }

// cmdHOTKEYS is HOTKEYS START|STOP|GET|RESET|HELP. With no argument or a
// number it is CacheStorm's older HOTKEYS [count], which Redis answers
// only with an error.
func cmdHOTKEYS(ctx *Context) error {
	if legacyHotkeysCall(ctx.Args) {
		return hotkeysByAccess(ctx)
	}
	sub := strings.ToUpper(ctx.ArgString(0))
	switch sub {
	case "STOP", "GET", "RESET":
		if ctx.ArgCount() != 1 {
			return ctx.WriteError(hotkeysErr("wrong number of arguments for 'hotkeys|" + strings.ToLower(sub) + "' command"))
		}
	}
	switch sub {
	case "HELP":
		return ctx.WriteArray(helpReply(hotkeysHelp))
	case "START":
		return hotkeysStart(ctx)
	case "STOP":
		hotkeys.mu.Lock()
		defer hotkeys.mu.Unlock()
		h := hotkeys.session
		if h == nil || !h.active {
			return ctx.WriteNull()
		}
		h.active = false
		h.durationMs = time.Now().UnixMilli() - h.start
		hotkeys.active.Store(false)
		return ctx.WriteOK()
	case "GET":
		return hotkeysGet(ctx)
	case "RESET":
		hotkeys.mu.Lock()
		defer hotkeys.mu.Unlock()
		if h := hotkeys.session; h != nil {
			h.expireLocked(time.Now().UnixMilli())
			if h.active {
				return ctx.WriteError(hotkeysErr("hotkey tracking session in progress, stop tracking first"))
			}
		}
		hotkeys.session = nil
		return ctx.WriteOK()
	}
	return ctx.WriteError(hotkeysErr("unknown subcommand or wrong number of arguments"))
}

func hotkeysStart(ctx *Context) error {
	hotkeys.mu.Lock()
	defer hotkeys.mu.Unlock()
	if h := hotkeys.session; h != nil {
		h.expireLocked(time.Now().UnixMilli())
		if h.active {
			return ctx.WriteError(hotkeysErr("hotkey tracking session already in progress"))
		}
	}
	argc := ctx.ArgCount() + 1
	if argc < 4 || !strings.EqualFold(ctx.ArgString(1), "METRICS") {
		return ctx.WriteError(hotkeysErr("METRICS parameter is required"))
	}
	metricsCount, err := redisAtoi64(ctx.ArgString(2))
	if err != nil || metricsCount < 1 || metricsCount > 2 {
		return ctx.WriteError(hotkeysErr("METRICS count must be > 0 and <= 2"))
	}
	metrics, parsed, valid := 0, int64(0), 0
	j := 3
	for j < ctx.ArgCount() && parsed < metricsCount {
		switch strings.ToUpper(ctx.ArgString(j)) {
		case "CPU":
			if metrics&hotkeysCPU != 0 {
				return ctx.WriteError(hotkeysErr("METRICS CPU defined more than once!"))
			}
			metrics |= hotkeysCPU
			valid++
		case "NET":
			if metrics&hotkeysNet != 0 {
				return ctx.WriteError(hotkeysErr("METRICS NET defined more than once!"))
			}
			metrics |= hotkeysNet
			valid++
		}
		parsed++
		j++
	}
	if parsed != metricsCount {
		return ctx.WriteError(hotkeysErr("METRICS count does not match number of metric types provided"))
	}
	if valid == 0 {
		return ctx.WriteError(hotkeysErr("METRICS no valid metrics passed. Supported: CPU|NET"))
	}
	count, durationMs, ratio := 10, int64(0), 1
	var slots []int
	for j < ctx.ArgCount() {
		more := j+1 < ctx.ArgCount()
		opt := strings.ToUpper(ctx.ArgString(j))
		switch {
		case more && opt == "COUNT":
			v, err := redisAtoi64(ctx.ArgString(j + 1))
			if err != nil || v < 1 || v > 64 {
				return ctx.WriteError(hotkeysErr("COUNT must be between 1 and 64"))
			}
			count = int(v)
			j += 2
		case more && opt == "DURATION":
			v, err := redisAtoi64(ctx.ArgString(j + 1))
			if err != nil || v < 1 || v > 1000000 {
				return ctx.WriteError(hotkeysErr("DURATION must be between 1 and 1000000"))
			}
			durationMs = v * 1000
			j += 2
		case more && opt == "SAMPLE":
			v, err := redisAtoi64(ctx.ArgString(j + 1))
			if err != nil || v < 1 || v > 1<<31-1 {
				return ctx.WriteError(hotkeysErr("SAMPLE ratio must be positive"))
			}
			ratio = int(v)
			j += 2
		case more && opt == "SLOTS":
			c := ctx.clusterState()
			if c == nil {
				return ctx.WriteError(hotkeysErr("SLOTS parameter cannot be used in non-cluster mode"))
			}
			if slots != nil {
				return ctx.WriteError(hotkeysErr("SLOTS parameter already specified"))
			}
			n, err := redisAtoi64(ctx.ArgString(j + 1))
			if err != nil || n < 1 || n > 16384 {
				return ctx.WriteError(hotkeysErr("SLOTS count must be between 1 and 16384"))
			}
			if j+1+int(n) >= ctx.ArgCount() {
				return ctx.WriteError(hotkeysErr("not enough slot numbers provided"))
			}
			picked := make([]int, 0, n)
			for i := 0; i < int(n); i++ {
				v, err := redisAtoi64(ctx.ArgString(j + 2 + i))
				if err != nil || v < 0 || v >= 16384 {
					return ctx.WriteError(hotkeysErr("Invalid or out of range slot"))
				}
				if !c.Slot(int(v)).Mine {
					return ctx.WriteError(hotkeysErr(fmt.Sprintf("slot %d not handled by this node", v)))
				}
				if slices.Contains(picked, int(v)) {
					return ctx.WriteError(hotkeysErr("duplicate slot number"))
				}
				picked = append(picked, int(v))
			}
			slices.Sort(picked)
			slots = picked
			j += 2 + int(n)
		default:
			return ctx.WriteError(ErrSyntaxError)
		}
	}
	h := &hotkeySession{
		active: true, metrics: metrics, count: count, durationMs: durationMs,
		sampleRatio: ratio, slots: slots, start: time.Now().UnixMilli(),
	}
	h.cpuSys, h.cpuUser = processCPU()
	if metrics&hotkeysCPU != 0 {
		h.cpu = newHotkeyTopK(count * 10)
	}
	if metrics&hotkeysNet != 0 {
		h.net = newHotkeyTopK(count * 10)
	}
	hotkeys.session = h
	hotkeys.active.Store(true)
	return ctx.WriteOK()
}

// slotRangesValue renders sorted slots as Redis' slot range arrays: [s] for
// a lone slot, [start end] for a run.
func slotRangesValue(slots []int) *resp.Value {
	var out []*resp.Value
	for i := 0; i < len(slots); {
		j := i
		for j+1 < len(slots) && slots[j+1] == slots[j]+1 {
			j++
		}
		if i == j {
			out = append(out, resp.ArrayValue([]*resp.Value{resp.IntegerValue(int64(slots[i]))}))
		} else {
			out = append(out, resp.ArrayValue([]*resp.Value{resp.IntegerValue(int64(slots[i])), resp.IntegerValue(int64(slots[j]))}))
		}
		i = j + 1
	}
	return resp.ArrayValue(out)
}

func hotkeysGet(ctx *Context) error {
	hotkeys.mu.Lock()
	defer hotkeys.mu.Unlock()
	h := hotkeys.session
	if h == nil {
		return ctx.WriteNull()
	}
	now := time.Now().UnixMilli()
	h.expireLocked(now)
	duration := h.durationMs
	if h.active {
		duration = now - h.start
	}
	selected := h.slots
	if selected == nil {
		selected = localSlots(ctx)
	}
	b2i := func(b bool) int64 {
		if b {
			return 1
		}
		return 0
	}
	pairs := []*resp.Value{
		resp.BulkString("tracking-active"), resp.IntegerValue(b2i(h.active)),
		resp.BulkString("sample-ratio"), resp.IntegerValue(int64(h.sampleRatio)),
		resp.BulkString("selected-slots"), slotRangesValue(selected),
	}
	add := func(name string, v int64) { pairs = append(pairs, resp.BulkString(name), resp.IntegerValue(v)) }
	hasSlots, hasSampling := h.slots != nil, h.sampleRatio > 1
	if hasSampling && hasSlots {
		add("sampled-commands-selected-slots-us", h.timeSampled)
	}
	if hasSlots {
		add("all-commands-selected-slots-us", h.timeSelected)
	}
	add("all-commands-all-slots-us", h.timeAll)
	if hasSampling && hasSlots {
		add("net-bytes-sampled-commands-selected-slots", h.netSampled)
	}
	if hasSlots {
		add("net-bytes-all-commands-selected-slots", h.netSelected)
	}
	add("net-bytes-all-commands-all-slots", h.netAll)
	add("collection-start-time-unix-ms", h.start)
	add("collection-duration-ms", duration)
	if h.cpu != nil {
		sys, user := processCPU()
		add("total-cpu-time-user-ms", int64((user-h.cpuUser)*1000))
		add("total-cpu-time-sys-ms", int64((sys-h.cpuSys)*1000))
	}
	if h.net != nil {
		add("total-net-bytes", int64(h.net.total))
	}
	list := func(name string, t *hotkeyTopK) {
		var items []*resp.Value
		for _, kc := range t.top(h.count) {
			items = append(items, resp.BulkString(kc.key), resp.IntegerValue(int64(kc.count)))
		}
		pairs = append(pairs, resp.BulkString(name), resp.ArrayValue(items))
	}
	if h.cpu != nil {
		list("by-cpu-time-us", h.cpu)
	}
	if h.net != nil {
		list("by-net-bytes", h.net)
	}
	return ctx.WriteArray([]*resp.Value{resp.PairsValue(pairs)})
}

// localSlots is the slots this node serves (a replica: its master's), every
// slot when the server is not in cluster mode.
func localSlots(ctx *Context) []int {
	c := ctx.clusterState()
	slots := make([]int, 0, 16384)
	for s := 0; s < 16384; s++ {
		if c == nil {
			slots = append(slots, s)
			continue
		}
		if st := c.Slot(s); st.Mine || st.OwnerIsMyMaster {
			slots = append(slots, s)
		}
	}
	return slots
}

// infoHotkeys is INFO's Hotkeys section, empty until a session starts.
func infoHotkeys(w *infoWriter) {
	w.section("Hotkeys")
	hotkeys.mu.Lock()
	if h := hotkeys.session; h != nil {
		h.expireLocked(time.Now().UnixMilli())
		active := int64(0)
		if h.active {
			active = 1
		}
		w.int("hotkeys-tracking-active", active)
		w.int("hotkeys-cmd-cpu-time", h.cpuTime)
	}
	hotkeys.mu.Unlock()
	w.end()
}

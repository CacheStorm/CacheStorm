package command

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// latencyHistoryLen is how many samples Redis keeps per latency event.
const latencyHistoryLen = 160

type latencySample struct {
	at int64 // unix seconds
	ms int64
}

type latencyEvent struct {
	samples []latencySample
	next    int
	max     int64
}

// latencyMonitor is Redis' latency monitor: events slower than
// latency-monitor-threshold milliseconds are recorded, one sample per
// second per event, and reported by the LATENCY command.
var latencyMonitor = struct {
	threshold atomic.Int64
	mu        sync.Mutex
	events    map[string]*latencyEvent
}{events: make(map[string]*latencyEvent)}

// LatencyThreshold returns latency-monitor-threshold in milliseconds.
func LatencyThreshold() int64 { return latencyMonitor.threshold.Load() }

// SetLatencyThreshold sets latency-monitor-threshold; 0 turns monitoring off.
func SetLatencyThreshold(ms int64) { latencyMonitor.threshold.Store(ms) }

// RecordLatency records event when it took at least the threshold.
func RecordLatency(event string, d time.Duration) {
	threshold := latencyMonitor.threshold.Load()
	if threshold <= 0 {
		return
	}
	ms := int64(d / time.Millisecond)
	if ms < threshold {
		return
	}
	now := time.Now().Unix()
	latencyMonitor.mu.Lock()
	defer latencyMonitor.mu.Unlock()
	ev := latencyMonitor.events[event]
	if ev == nil {
		ev = &latencyEvent{}
		latencyMonitor.events[event] = ev
	}
	if ms > ev.max {
		ev.max = ms
	}
	// One sample per second: a second event in the same second keeps the
	// larger latency, as Redis does.
	if n := len(ev.samples); n > 0 {
		lastIdx := n - 1
		if n == latencyHistoryLen {
			lastIdx = (ev.next - 1 + n) % n
		}
		if last := &ev.samples[lastIdx]; last.at == now {
			if ms > last.ms {
				last.ms = ms
			}
			return
		}
	}
	if len(ev.samples) < latencyHistoryLen {
		ev.samples = append(ev.samples, latencySample{now, ms})
		return
	}
	ev.samples[ev.next] = latencySample{now, ms}
	ev.next = (ev.next + 1) % latencyHistoryLen
}

// history returns an event's samples, oldest first.
func (ev *latencyEvent) history() []latencySample {
	if len(ev.samples) < latencyHistoryLen {
		return append([]latencySample(nil), ev.samples...)
	}
	out := make([]latencySample, 0, latencyHistoryLen)
	out = append(out, ev.samples[ev.next:]...)
	return append(out, ev.samples[:ev.next]...)
}

func latencyEventNames() []string {
	names := make([]string, 0, len(latencyMonitor.events))
	for name := range latencyMonitor.events {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func cmdLATENCY(ctx *Context) error {
	if ctx.ArgCount() < 1 {
		return ctx.WriteError(arityError("latency"))
	}
	sub := strings.ToUpper(ctx.ArgString(0))
	latencyMonitor.mu.Lock()
	defer latencyMonitor.mu.Unlock()
	switch sub {
	case "LATEST":
		out := make([]*resp.Value, 0, len(latencyMonitor.events))
		for _, name := range latencyEventNames() {
			ev := latencyMonitor.events[name]
			h := ev.history()
			last := h[len(h)-1]
			out = append(out, resp.ArrayValue([]*resp.Value{
				resp.BulkString(name), resp.IntegerValue(last.at), resp.IntegerValue(last.ms), resp.IntegerValue(ev.max),
			}))
		}
		return ctx.WriteArray(out)
	case "HISTORY":
		if ctx.ArgCount() != 2 {
			return ctx.WriteError(arityError("latency|history"))
		}
		ev := latencyMonitor.events[ctx.ArgString(1)]
		out := []*resp.Value{}
		if ev != nil {
			for _, s := range ev.history() {
				out = append(out, resp.ArrayValue([]*resp.Value{resp.IntegerValue(s.at), resp.IntegerValue(s.ms)}))
			}
		}
		return ctx.WriteArray(out)
	case "RESET":
		reset := 0
		if ctx.ArgCount() == 1 {
			reset = len(latencyMonitor.events)
			latencyMonitor.events = make(map[string]*latencyEvent)
		} else {
			for i := 1; i < ctx.ArgCount(); i++ {
				if _, ok := latencyMonitor.events[ctx.ArgString(i)]; ok {
					delete(latencyMonitor.events, ctx.ArgString(i))
					reset++
				}
			}
		}
		return ctx.WriteInteger(int64(reset))
	case "GRAPH":
		if ctx.ArgCount() != 2 {
			return ctx.WriteError(arityError("latency|graph"))
		}
		ev := latencyMonitor.events[ctx.ArgString(1)]
		if ev == nil {
			return ctx.WriteError(errors.New("ERR No samples available for event '" + ctx.ArgString(1) + "'"))
		}
		var b strings.Builder
		b.WriteString(ctx.ArgString(1) + " - high " + strconv.FormatInt(ev.max, 10) + " ms\n")
		now := time.Now().Unix()
		for _, s := range ev.history() {
			b.WriteString(strconv.FormatInt(now-s.at, 10) + "s ago: " + strconv.FormatInt(s.ms, 10) + " ms\n")
		}
		return ctx.WriteVerbatim(b.String())
	case "DOCTOR":
		var b strings.Builder
		threshold := latencyMonitor.threshold.Load()
		switch {
		case threshold <= 0:
			b.WriteString("Latency monitoring is disabled in this CacheStorm instance. Enable it with \"CONFIG SET latency-monitor-threshold <milliseconds>\".\n")
		case len(latencyMonitor.events) == 0:
			b.WriteString("No latency spikes were observed above " + strconv.FormatInt(threshold, 10) + " ms since monitoring started or the last LATENCY RESET.\n")
		default:
			b.WriteString("Latency spikes above " + strconv.FormatInt(threshold, 10) + " ms were observed:\n\n")
			for i, name := range latencyEventNames() {
				ev := latencyMonitor.events[name]
				h := ev.history()
				var sum int64
				for _, s := range h {
					sum += s.ms
				}
				b.WriteString(strconv.Itoa(i+1) + ". " + name + ": " + strconv.Itoa(len(h)) + " latency spikes (average " +
					strconv.FormatInt(sum/int64(len(h)), 10) + "ms, worst " + strconv.FormatInt(ev.max, 10) + "ms).\n")
			}
			if latencyMonitor.events["command"] != nil {
				b.WriteString("\nSlow commands are listed by SLOWLOG GET.\n")
			}
		}
		return ctx.WriteVerbatim(b.String())
	case "HISTOGRAM":
		return ctx.Writer.WriteValue(resp.PairsValue([]*resp.Value{}))
	case "HELP":
		return ctx.WriteArray([]*resp.Value{
			resp.BulkString("LATEST"),
			resp.BulkString("HISTORY <event>"),
			resp.BulkString("RESET [<event> ...]"),
			resp.BulkString("GRAPH <event>"),
			resp.BulkString("DOCTOR"),
		})
	default:
		return ctx.WriteError(errors.New("ERR unknown subcommand '" + ctx.ArgString(0) + "'. Try LATENCY HELP."))
	}
}

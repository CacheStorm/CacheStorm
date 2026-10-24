package command

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// The expected replies in this file were captured from redis:7.4.

func streamShapeRun(t *testing.T, s *store.Store, r *Router, args ...string) string {
	t.Helper()
	raw := make([][]byte, len(args)-1)
	for i, a := range args[1:] {
		raw[i] = []byte(a)
	}
	var out bytes.Buffer
	w := resp.NewWriter(&out)
	if err := r.Execute(NewContext(args[0], raw, s, w)); err != nil {
		t.Fatalf("%v: %v", args, err)
	}
	_ = w.Flush()
	return out.String()
}

type streamShapeStep struct {
	args []string
	want string
}

func runStreamShapeSteps(t *testing.T, steps []streamShapeStep) {
	t.Helper()
	s := store.NewStore()
	r := NewRouter()
	RegisterStreamCommands(r)
	for _, st := range steps {
		got := streamShapeRun(t, s, r, st.args...)
		if st.want != "" && got != st.want {
			t.Errorf("%s\n got  %q\n want %q", strings.Join(st.args, " "), got, st.want)
		}
	}
}

func sargs(line string) []string { return strings.Fields(line) }

func TestXReadGroupReplyShape(t *testing.T) {
	runStreamShapeSteps(t, []streamShapeStep{
		{sargs("XADD s 1-0 a 1"), ""},
		{sargs("XADD s 2-0 b 2"), ""},
		{sargs("XADD s 3-0 c 3"), ""},
		{sargs("XGROUP CREATE s g 0"), "+OK\r\n"},
		{sargs("XREADGROUP GROUP g c1 COUNT 2 STREAMS s >"),
			"*1\r\n*2\r\n$1\r\ns\r\n*2\r\n*2\r\n$3\r\n1-0\r\n*2\r\n$1\r\na\r\n$1\r\n1\r\n*2\r\n$3\r\n2-0\r\n*2\r\n$1\r\nb\r\n$1\r\n2\r\n"},
		{sargs("XREADGROUP GROUP g c2 STREAMS s >"),
			"*1\r\n*2\r\n$1\r\ns\r\n*1\r\n*2\r\n$3\r\n3-0\r\n*2\r\n$1\r\nc\r\n$1\r\n3\r\n"},
		{sargs("XREADGROUP GROUP g c2 STREAMS s >"), "*-1\r\n"},
		{sargs("XPENDING s g"),
			"*4\r\n:3\r\n$3\r\n1-0\r\n$3\r\n3-0\r\n*2\r\n*2\r\n$2\r\nc1\r\n$1\r\n2\r\n*2\r\n$2\r\nc2\r\n$1\r\n1\r\n"},
		{sargs("XDEL s 1-0"), ":1\r\n"},
		{sargs("XREADGROUP GROUP g c1 STREAMS s 0"),
			"*1\r\n*2\r\n$1\r\ns\r\n*2\r\n*2\r\n$3\r\n1-0\r\n*-1\r\n*2\r\n$3\r\n2-0\r\n*2\r\n$1\r\nb\r\n$1\r\n2\r\n"},
		{sargs("XACK s g 1-0 2-0 99-0"), ":2\r\n"},
		{sargs("XPENDING s g"),
			"*4\r\n:1\r\n$3\r\n3-0\r\n$3\r\n3-0\r\n*1\r\n*2\r\n$2\r\nc2\r\n$1\r\n1\r\n"},
		{sargs("XREADGROUP GROUP g c1 STREAMS s 0"), "*1\r\n*2\r\n$1\r\ns\r\n*0\r\n"},
		{sargs("XACK s g 3-0"), ":1\r\n"},
		{sargs("XPENDING s g"), "*4\r\n:0\r\n$-1\r\n$-1\r\n*-1\r\n"},
		{sargs("XREADGROUP GROUP nog c STREAMS s >"),
			"-NOGROUP No such key 's' or consumer group 'nog' in XREADGROUP with GROUP option\r\n"},
		{sargs("XREADGROUP GROUP g c STREAMS s $"),
			"-ERR The $ ID is meaningless in the context of XREADGROUP: you want to read the history of this consumer by specifying a proper ID, or use the > ID to get new messages. The $ ID would just return an empty result set.\r\n"},
	})
}

func TestXAutoClaimReplyShape(t *testing.T) {
	runStreamShapeSteps(t, []streamShapeStep{
		{sargs("XADD s 1-0 a 1"), ""},
		{sargs("XADD s 2-0 b 2"), ""},
		{sargs("XADD s 3-0 c 3"), ""},
		{sargs("XGROUP CREATE s g 0"), ""},
		{sargs("XREADGROUP GROUP g c1 STREAMS s >"), ""},
		{sargs("XDEL s 2-0"), ":1\r\n"},
		{sargs("XAUTOCLAIM s g c2 0 0 COUNT 1"),
			"*3\r\n$3\r\n2-0\r\n*1\r\n*2\r\n$3\r\n1-0\r\n*2\r\n$1\r\na\r\n$1\r\n1\r\n*0\r\n"},
		{sargs("XAUTOCLAIM s g c2 0 2-0 COUNT 1 JUSTID"),
			"*3\r\n$3\r\n3-0\r\n*0\r\n*1\r\n$3\r\n2-0\r\n"},
		{sargs("XAUTOCLAIM s g c2 0 0 JUSTID"),
			"*3\r\n$3\r\n0-0\r\n*2\r\n$3\r\n1-0\r\n$3\r\n3-0\r\n*0\r\n"},
		{sargs("XAUTOCLAIM s g c2 0 0 COUNT 0"), "-ERR COUNT must be > 0\r\n"},
		{sargs("XAUTOCLAIM missing g c2 0 0"), "-NOGROUP No such key 'missing' or consumer group 'g'\r\n"},
		{sargs("XCLAIM s g c3 0 1-0 99-0 JUSTID"), "*1\r\n$3\r\n1-0\r\n"},
		{sargs("XCLAIM s g c3 0 1-0 BOGUS"), "-ERR Unrecognized XCLAIM option 'BOGUS'\r\n"},
		{sargs("XPENDING s g"),
			"*4\r\n:2\r\n$3\r\n1-0\r\n$3\r\n3-0\r\n*2\r\n*2\r\n$2\r\nc2\r\n$1\r\n1\r\n*2\r\n$2\r\nc3\r\n$1\r\n1\r\n"},
	})
}

func TestXAddIDForms(t *testing.T) {
	runStreamShapeSteps(t, []streamShapeStep{
		{sargs("XADD s 0-* f v"), "$3\r\n0-1\r\n"},
		{sargs("XADD s 0-* f v"), "$3\r\n0-2\r\n"},
		{sargs("XADD s 5 f v"), "$3\r\n5-0\r\n"},
		{sargs("XADD s 5-* f v"), "$3\r\n5-1\r\n"},
		{sargs("XADD s 4-* f v"), "-ERR The ID specified in XADD is equal or smaller than the target stream top item\r\n"},
		{sargs("XADD s 18446744073709551615-18446744073709551615 f v"), "$41\r\n18446744073709551615-18446744073709551615\r\n"},
		{sargs("XADD s * f v"), "-ERR The stream has exhausted the last possible ID, unable to add more items\r\n"},
		{sargs("XADD n 0-0 f v"), "-ERR The ID specified in XADD must be greater than 0-0\r\n"},
		{sargs("XADD n 1-1 f"), "-ERR wrong number of arguments for 'xadd' command\r\n"},
		{sargs("XADD n MAXLEN 1 LIMIT 1 * f v"), "-ERR syntax error, LIMIT cannot be used without the special ~ option\r\n"},
		{sargs("XLEN n"), ":0\r\n"},
		{[]string{"XADD", "o", "1-0", "z", "1", "a", "2", "z", "3"}, ""},
		{sargs("XRANGE o - +"), "*1\r\n*2\r\n$3\r\n1-0\r\n*6\r\n$1\r\nz\r\n$1\r\n1\r\n$1\r\na\r\n$1\r\n2\r\n$1\r\nz\r\n$1\r\n3\r\n"},
	})
}

func TestXInfoGroupsLag(t *testing.T) {
	runStreamShapeSteps(t, []streamShapeStep{
		{sargs("XADD s 1-0 a 1"), ""},
		{sargs("XADD s 2-0 b 2"), ""},
		{sargs("XADD s 3-0 c 3"), ""},
		{sargs("XGROUP CREATE s ge 2-0 ENTRIESREAD 1"), "+OK\r\n"},
		{sargs("XGROUP CREATE s gw 2-5"), "+OK\r\n"},
		{sargs("XINFO GROUPS s"),
			"*2\r\n*12\r\n$4\r\nname\r\n$2\r\nge\r\n$9\r\nconsumers\r\n:0\r\n$7\r\npending\r\n:0\r\n$17\r\nlast-delivered-id\r\n$3\r\n2-0\r\n$12\r\nentries-read\r\n:1\r\n$3\r\nlag\r\n:2\r\n" +
				"*12\r\n$4\r\nname\r\n$2\r\ngw\r\n$9\r\nconsumers\r\n:0\r\n$7\r\npending\r\n:0\r\n$17\r\nlast-delivered-id\r\n$3\r\n2-5\r\n$12\r\nentries-read\r\n$-1\r\n$3\r\nlag\r\n$-1\r\n"},
		{sargs("XTRIM s MAXLEN 1"), ":2\r\n"},
		{sargs("XINFO GROUPS s"),
			"*2\r\n*12\r\n$4\r\nname\r\n$2\r\nge\r\n$9\r\nconsumers\r\n:0\r\n$7\r\npending\r\n:0\r\n$17\r\nlast-delivered-id\r\n$3\r\n2-0\r\n$12\r\nentries-read\r\n:1\r\n$3\r\nlag\r\n:1\r\n" +
				"*12\r\n$4\r\nname\r\n$2\r\ngw\r\n$9\r\nconsumers\r\n:0\r\n$7\r\npending\r\n:0\r\n$17\r\nlast-delivered-id\r\n$3\r\n2-5\r\n$12\r\nentries-read\r\n$-1\r\n$3\r\nlag\r\n:1\r\n"},
		{sargs("XINFO STREAM missing"), "-ERR no such key\r\n"},
		{sargs("XINFO CONSUMERS s nog"), "-NOGROUP No such consumer group 'nog' for key name 's'\r\n"},
		{sargs("XGROUP CREATE missing g $"), "-ERR The XGROUP subcommand requires the key to exist. Note that for CREATE you may want to use the MKSTREAM option to create an empty stream automatically.\r\n"},
		{sargs("XGROUP CREATE s"), "-ERR wrong number of arguments for 'xgroup|create' command\r\n"},
	})
}

// XADD rewrites a generated ID into its arguments so the AOF and replicas
// replay the same entry instead of generating a new ID.
func TestXAddRewritesGeneratedIDForPropagation(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterStreamCommands(r)
	for _, idArg := range []string{"*", "7-*"} {
		raw := [][]byte{[]byte("s" + idArg), []byte("MAXLEN"), []byte("10"), []byte(idArg), []byte("f"), []byte("v")}
		var out bytes.Buffer
		ctx := NewContext("XADD", raw, s, resp.NewWriter(&out))
		if err := r.Execute(ctx); err != nil {
			t.Fatal(err)
		}
		v, err := resp.NewReader(&out).ReadValue()
		if err != nil || v.Type != resp.TypeBulkString {
			t.Fatalf("XADD %s replied %v %v", idArg, v, err)
		}
		if got := string(ctx.Args[3]); got != string(v.Bulk) {
			t.Errorf("XADD %s left argument %q, want the generated %q", idArg, got, v.Bulk)
		}
	}
}

func TestStreamGroupOpsRaceFree(t *testing.T) {
	s := store.NewStore()
	r := NewRouter()
	RegisterStreamCommands(r)
	streamShapeRun(t, s, r, sargs("XGROUP CREATE s g $ MKSTREAM")...)

	var wg sync.WaitGroup
	worker := func(fn func(i int)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				fn(i)
			}
		}()
	}
	worker(func(i int) { streamShapeRun(t, s, r, "XADD", "s", "MAXLEN", "~", "50", "*", "f", fmt.Sprint(i)) })
	worker(func(i int) {
		streamShapeRun(t, s, r, "XREADGROUP", "GROUP", "g", fmt.Sprintf("c%d", i%3), "COUNT", "5", "STREAMS", "s", ">")
	})
	worker(func(i int) { streamShapeRun(t, s, r, "XREADGROUP", "GROUP", "g", "c1", "STREAMS", "s", "0") })
	worker(func(i int) { streamShapeRun(t, s, r, "XAUTOCLAIM", "s", "g", "c9", "0", "0", "COUNT", "3") })
	worker(func(i int) { streamShapeRun(t, s, r, "XPENDING", "s", "g", "-", "+", "10") })
	worker(func(i int) { streamShapeRun(t, s, r, "XINFO", "STREAM", "s", "FULL") })
	worker(func(i int) { streamShapeRun(t, s, r, "XINFO", "CONSUMERS", "s", "g") })
	worker(func(i int) { streamShapeRun(t, s, r, "XTRIM", "s", "MAXLEN", "30") })
	wg.Wait()

	summary := streamShapeRun(t, s, r, "XPENDING", "s", "g")
	if !strings.HasPrefix(summary, "*4\r\n") {
		t.Fatalf("XPENDING after the race = %q", summary)
	}
}

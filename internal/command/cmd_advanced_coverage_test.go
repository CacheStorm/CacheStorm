package command

import (
	"io"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// newDiscardContext creates a test context that writes to io.Discard.
func newDiscardContext(cmd string, args [][]byte, s *store.Store) *Context {
	w := resp.NewWriter(io.Discard)
	return &Context{
		Command:     cmd,
		Args:        args,
		Store:       s,
		Writer:      w,
		Transaction: NewTransaction(),
	}
}

// ---------------------------------------------------------------------------
// Stream Commands
// ---------------------------------------------------------------------------

func TestAdvCoverage_XADD_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XADD", [][]byte{[]byte("s1")}, s)
	if err := cmdXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XADD_BasicAutoID(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XADD", [][]byte{
		[]byte("mystream"), []byte("*"), []byte("name"), []byte("Alice"),
	}, s)
	if err := cmdXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stream := getStream(ctx, "mystream")
	if stream == nil {
		t.Fatal("stream not created")
	}
	if stream.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", stream.Len())
	}
}

func TestAdvCoverage_XADD_ExplicitID(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XADD", [][]byte{
		[]byte("mystream"), []byte("100-0"), []byte("k1"), []byte("v1"),
	}, s)
	if err := cmdXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XADD_WithMAXLEN(t *testing.T) {
	s := store.NewStore()
	// Add several entries then MAXLEN should trim
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"a": []byte("b")})
	stream.Add("2-0", map[string][]byte{"a": []byte("b")})
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XADD", [][]byte{
		[]byte("s"), []byte("MAXLEN"), []byte("2"), []byte("*"), []byte("c"), []byte("d"),
	}, s)
	if err := cmdXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XADD_WithMAXLEN_Approx(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XADD", [][]byte{
		[]byte("s"), []byte("MAXLEN"), []byte("~"), []byte("5"), []byte("*"), []byte("f"), []byte("v"),
	}, s)
	if err := cmdXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XADD_WithMINID(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"a": []byte("b")})
	stream.Add("2-0", map[string][]byte{"a": []byte("b")})
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XADD", [][]byte{
		[]byte("s"), []byte("MINID"), []byte("2-0"), []byte("*"), []byte("c"), []byte("d"),
	}, s)
	if err := cmdXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XADD_WithNOMKSTREAM(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XADD", [][]byte{
		[]byte("s"), []byte("NOMKSTREAM"), []byte("*"), []byte("f"), []byte("v"),
	}, s)
	if err := cmdXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XADD_WrongType(t *testing.T) {
	s := store.NewStore()
	s.Set("strkey", &store.StringValue{Data: []byte("hello")}, store.SetOptions{})
	ctx := newDiscardContext("XADD", [][]byte{
		[]byte("strkey"), []byte("*"), []byte("f"), []byte("v"),
	}, s)
	// Should not panic; writes error
	if err := cmdXADD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XLEN_NonExistent(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XLEN", [][]byte{[]byte("nostream")}, s)
	if err := cmdXLEN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XLEN_Existing(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"k": []byte("v")})
	stream.Add("2-0", map[string][]byte{"k": []byte("v")})
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XLEN", [][]byte{[]byte("s")}, s)
	if err := cmdXLEN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XLEN_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XLEN", nil, s)
	if err := cmdXLEN(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XRANGE_Basic(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"a": []byte("1")})
	stream.Add("2-0", map[string][]byte{"b": []byte("2")})
	stream.Add("3-0", map[string][]byte{"c": []byte("3")})
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XRANGE", [][]byte{
		[]byte("s"), []byte("-"), []byte("+"),
	}, s)
	if err := cmdXRANGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XRANGE_WithCount(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"a": []byte("1")})
	stream.Add("2-0", map[string][]byte{"b": []byte("2")})
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XRANGE", [][]byte{
		[]byte("s"), []byte("-"), []byte("+"), []byte("COUNT"), []byte("1"),
	}, s)
	if err := cmdXRANGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XRANGE_NonExistent(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XRANGE", [][]byte{
		[]byte("nostream"), []byte("-"), []byte("+"),
	}, s)
	if err := cmdXRANGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XRANGE_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XRANGE", [][]byte{[]byte("s")}, s)
	if err := cmdXRANGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREVRANGE_Basic(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"a": []byte("1")})
	stream.Add("2-0", map[string][]byte{"b": []byte("2")})
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XREVRANGE", [][]byte{
		[]byte("s"), []byte("+"), []byte("-"),
	}, s)
	if err := cmdXREVRANGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREVRANGE_WithCount(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"a": []byte("1")})
	stream.Add("2-0", map[string][]byte{"b": []byte("2")})
	stream.Add("3-0", map[string][]byte{"c": []byte("3")})
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XREVRANGE", [][]byte{
		[]byte("s"), []byte("+"), []byte("-"), []byte("COUNT"), []byte("1"),
	}, s)
	if err := cmdXREVRANGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREVRANGE_NonExistent(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XREVRANGE", [][]byte{
		[]byte("nostream"), []byte("+"), []byte("-"),
	}, s)
	if err := cmdXREVRANGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREVRANGE_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XREVRANGE", [][]byte{[]byte("s"), []byte("+")}, s)
	if err := cmdXREVRANGE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREAD_Basic(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	s.Set("s1", stream, store.SetOptions{})

	ctx := newDiscardContext("XREAD", [][]byte{
		[]byte("STREAMS"), []byte("s1"), []byte("0"),
	}, s)
	if err := cmdXREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREAD_WithCount(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.Add("2-0", map[string][]byte{"f": []byte("v")})
	s.Set("s1", stream, store.SetOptions{})

	ctx := newDiscardContext("XREAD", [][]byte{
		[]byte("COUNT"), []byte("1"), []byte("STREAMS"), []byte("s1"), []byte("0"),
	}, s)
	if err := cmdXREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREAD_NonExistent(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XREAD", [][]byte{
		[]byte("STREAMS"), []byte("nostream"), []byte("0"),
	}, s)
	if err := cmdXREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREAD_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XREAD", [][]byte{[]byte("STREAMS")}, s)
	if err := cmdXREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREAD_DollarID(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	s.Set("s1", stream, store.SetOptions{})

	ctx := newDiscardContext("XREAD", [][]byte{
		[]byte("STREAMS"), []byte("s1"), []byte("$"),
	}, s)
	if err := cmdXREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREAD_MultipleStreams(t *testing.T) {
	s := store.NewStore()
	s1 := store.NewStreamValue(0)
	s1.Add("1-0", map[string][]byte{"f": []byte("v")})
	s.Set("s1", s1, store.SetOptions{})
	s2 := store.NewStreamValue(0)
	s2.Add("1-0", map[string][]byte{"f": []byte("v")})
	s.Set("s2", s2, store.SetOptions{})

	ctx := newDiscardContext("XREAD", [][]byte{
		[]byte("STREAMS"), []byte("s1"), []byte("s2"), []byte("0"), []byte("0"),
	}, s)
	if err := cmdXREAD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XDEL_Basic(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.Add("2-0", map[string][]byte{"f": []byte("v")})
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XDEL", [][]byte{
		[]byte("s"), []byte("1-0"),
	}, s)
	if err := cmdXDEL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XDEL_NonExistent(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XDEL", [][]byte{
		[]byte("nostream"), []byte("1-0"),
	}, s)
	if err := cmdXDEL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XDEL_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XDEL", [][]byte{[]byte("s")}, s)
	if err := cmdXDEL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XDEL_MultipleIDs(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.Add("2-0", map[string][]byte{"f": []byte("v")})
	stream.Add("3-0", map[string][]byte{"f": []byte("v")})
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XDEL", [][]byte{
		[]byte("s"), []byte("1-0"), []byte("3-0"),
	}, s)
	if err := cmdXDEL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XTRIM_Basic(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.Add("2-0", map[string][]byte{"f": []byte("v")})
	stream.Add("3-0", map[string][]byte{"f": []byte("v")})
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XTRIM", [][]byte{
		[]byte("s"), []byte("MAXLEN"), []byte("2"),
	}, s)
	if err := cmdXTRIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XTRIM_Approximate(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.Add("2-0", map[string][]byte{"f": []byte("v")})
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XTRIM", [][]byte{
		[]byte("s"), []byte("MAXLEN"), []byte("~"), []byte("1"),
	}, s)
	if err := cmdXTRIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XTRIM_NonExistent(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XTRIM", [][]byte{
		[]byte("nostream"), []byte("MAXLEN"), []byte("1"),
	}, s)
	if err := cmdXTRIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XTRIM_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XTRIM", [][]byte{[]byte("s"), []byte("MAXLEN")}, s)
	if err := cmdXTRIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XTRIM_InvalidStrategy(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XTRIM", [][]byte{
		[]byte("s"), []byte("INVALID"), []byte("2"),
	}, s)
	if err := cmdXTRIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XTRIM_InvalidMaxLen(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XTRIM", [][]byte{
		[]byte("s"), []byte("MAXLEN"), []byte("notanumber"),
	}, s)
	if err := cmdXTRIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XINFO_Stream(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XINFO", [][]byte{
		[]byte("STREAM"), []byte("s"),
	}, s)
	if err := cmdXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XINFO_StreamFull(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.CreateGroup("g1", "0")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XINFO", [][]byte{
		[]byte("STREAM"), []byte("s"), []byte("FULL"),
	}, s)
	if err := cmdXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XINFO_StreamNonExistent(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XINFO", [][]byte{
		[]byte("STREAM"), []byte("nostream"),
	}, s)
	if err := cmdXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XINFO_Groups(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.CreateGroup("g1", "0")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XINFO", [][]byte{
		[]byte("GROUPS"), []byte("s"),
	}, s)
	if err := cmdXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XINFO_GroupsNonExistent(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XINFO", [][]byte{
		[]byte("GROUPS"), []byte("nostream"),
	}, s)
	if err := cmdXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XINFO_Consumers(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.CreateGroup("g1", "0")
	group := stream.GetGroup("g1")
	group.GetOrCreateConsumer("c1")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XINFO", [][]byte{
		[]byte("CONSUMERS"), []byte("s"), []byte("g1"),
	}, s)
	if err := cmdXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XINFO_ConsumersNoGroup(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XINFO", [][]byte{
		[]byte("CONSUMERS"), []byte("s"), []byte("nogroup"),
	}, s)
	if err := cmdXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XINFO_Help(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XINFO", [][]byte{[]byte("HELP")}, s)
	if err := cmdXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XINFO_UnknownSubCmd(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XINFO", [][]byte{[]byte("UNKNOWN")}, s)
	if err := cmdXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XINFO_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XINFO", nil, s)
	if err := cmdXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XINFO_StreamWrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XINFO", [][]byte{[]byte("STREAM")}, s)
	if err := cmdXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XINFO_GroupsWrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XINFO", [][]byte{[]byte("GROUPS")}, s)
	if err := cmdXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XINFO_ConsumersWrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XINFO", [][]byte{[]byte("CONSUMERS"), []byte("s")}, s)
	if err := cmdXINFO(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XGROUP_CREATE(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XGROUP", [][]byte{
		[]byte("CREATE"), []byte("s"), []byte("mygroup"), []byte("$"),
	}, s)
	if err := cmdXGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XGROUP_CREATE_MKSTREAM(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XGROUP", [][]byte{
		[]byte("CREATE"), []byte("newstream"), []byte("mygroup"), []byte("$"), []byte("MKSTREAM"),
	}, s)
	if err := cmdXGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XGROUP_CREATE_NoStream(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XGROUP", [][]byte{
		[]byte("CREATE"), []byte("nostream"), []byte("mygroup"), []byte("$"),
	}, s)
	// Should write error, not return error
	if err := cmdXGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XGROUP_CREATE_Duplicate(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.CreateGroup("mygroup", "0")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XGROUP", [][]byte{
		[]byte("CREATE"), []byte("s"), []byte("mygroup"), []byte("$"),
	}, s)
	if err := cmdXGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XGROUP_DESTROY(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.CreateGroup("mygroup", "0")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XGROUP", [][]byte{
		[]byte("DESTROY"), []byte("s"), []byte("mygroup"),
	}, s)
	if err := cmdXGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XGROUP_DESTROY_NonExistentGroup(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XGROUP", [][]byte{
		[]byte("DESTROY"), []byte("s"), []byte("nogroup"),
	}, s)
	if err := cmdXGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XGROUP_DESTROY_NonExistentStream(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XGROUP", [][]byte{
		[]byte("DESTROY"), []byte("nostream"), []byte("mygroup"),
	}, s)
	if err := cmdXGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XGROUP_SETID(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.CreateGroup("mygroup", "0")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XGROUP", [][]byte{
		[]byte("SETID"), []byte("s"), []byte("mygroup"), []byte("5-0"),
	}, s)
	if err := cmdXGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XGROUP_SETID_NoGroup(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XGROUP", [][]byte{
		[]byte("SETID"), []byte("s"), []byte("nogroup"), []byte("5-0"),
	}, s)
	if err := cmdXGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XGROUP_DELCONSUMER(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.CreateGroup("g1", "0")
	group := stream.GetGroup("g1")
	group.GetOrCreateConsumer("c1")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XGROUP", [][]byte{
		[]byte("DELCONSUMER"), []byte("s"), []byte("g1"), []byte("c1"),
	}, s)
	if err := cmdXGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XGROUP_DELCONSUMER_NoGroup(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XGROUP", [][]byte{
		[]byte("DELCONSUMER"), []byte("s"), []byte("nogroup"), []byte("c1"),
	}, s)
	if err := cmdXGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XGROUP_Unknown(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XGROUP", [][]byte{
		[]byte("BADCMD"), []byte("s"),
	}, s)
	if err := cmdXGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XGROUP_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XGROUP", [][]byte{[]byte("CREATE")}, s)
	if err := cmdXGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREADGROUP_Basic(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.CreateGroup("g1", "0")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XREADGROUP", [][]byte{
		[]byte("GROUP"), []byte("g1"), []byte("c1"),
		[]byte("STREAMS"), []byte("s"), []byte(">"),
	}, s)
	if err := cmdXREADGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREADGROUP_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XREADGROUP", [][]byte{
		[]byte("GROUP"), []byte("g1"),
	}, s)
	if err := cmdXREADGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREADGROUP_NoGroup(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XREADGROUP", [][]byte{
		[]byte("GROUP"), []byte("nogroup"), []byte("c1"),
		[]byte("STREAMS"), []byte("s"), []byte(">"),
	}, s)
	if err := cmdXREADGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XREADGROUP_WithCount(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.Add("2-0", map[string][]byte{"f": []byte("v")})
	stream.CreateGroup("g1", "0")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XREADGROUP", [][]byte{
		[]byte("GROUP"), []byte("g1"), []byte("c1"),
		[]byte("COUNT"), []byte("1"),
		[]byte("STREAMS"), []byte("s"), []byte(">"),
	}, s)
	if err := cmdXREADGROUP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XACK_Basic(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.CreateGroup("g1", "0")
	group := stream.GetGroup("g1")
	group.GetOrCreateConsumer("c1")
	group.AddPending("1-0", "c1")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XACK", [][]byte{
		[]byte("s"), []byte("g1"), []byte("1-0"),
	}, s)
	if err := cmdXACK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XACK_NonExistentStream(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XACK", [][]byte{
		[]byte("nostream"), []byte("g1"), []byte("1-0"),
	}, s)
	if err := cmdXACK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XACK_NonExistentGroup(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XACK", [][]byte{
		[]byte("s"), []byte("nogroup"), []byte("1-0"),
	}, s)
	if err := cmdXACK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XACK_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XACK", [][]byte{[]byte("s"), []byte("g1")}, s)
	if err := cmdXACK(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XPENDING_Summary(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.CreateGroup("g1", "0")
	group := stream.GetGroup("g1")
	group.GetOrCreateConsumer("c1")
	group.AddPending("1-0", "c1")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XPENDING", [][]byte{
		[]byte("s"), []byte("g1"),
	}, s)
	if err := cmdXPENDING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XPENDING_Empty(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.CreateGroup("g1", "0")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XPENDING", [][]byte{
		[]byte("s"), []byte("g1"),
	}, s)
	if err := cmdXPENDING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XPENDING_WithRange(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.CreateGroup("g1", "0")
	group := stream.GetGroup("g1")
	group.GetOrCreateConsumer("c1")
	group.AddPending("1-0", "c1")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XPENDING", [][]byte{
		[]byte("s"), []byte("g1"), []byte("-"), []byte("+"), []byte("10"),
	}, s)
	if err := cmdXPENDING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XPENDING_WithConsumer(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.CreateGroup("g1", "0")
	group := stream.GetGroup("g1")
	group.GetOrCreateConsumer("c1")
	group.AddPending("1-0", "c1")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XPENDING", [][]byte{
		[]byte("s"), []byte("g1"), []byte("-"), []byte("+"), []byte("10"), []byte("c1"),
	}, s)
	if err := cmdXPENDING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XPENDING_NonExistentStream(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XPENDING", [][]byte{
		[]byte("nostream"), []byte("g1"),
	}, s)
	if err := cmdXPENDING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XPENDING_NoGroup(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XPENDING", [][]byte{
		[]byte("s"), []byte("nogroup"),
	}, s)
	if err := cmdXPENDING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XPENDING_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XPENDING", [][]byte{[]byte("s")}, s)
	if err := cmdXPENDING(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XCLAIM_Basic(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.CreateGroup("g1", "0")
	group := stream.GetGroup("g1")
	group.GetOrCreateConsumer("c1")
	group.AddPending("1-0", "c1")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XCLAIM", [][]byte{
		[]byte("s"), []byte("g1"), []byte("c2"), []byte("0"), []byte("1-0"),
	}, s)
	if err := cmdXCLAIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XCLAIM_WithJUSTID(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.CreateGroup("g1", "0")
	group := stream.GetGroup("g1")
	group.GetOrCreateConsumer("c1")
	group.AddPending("1-0", "c1")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XCLAIM", [][]byte{
		[]byte("s"), []byte("g1"), []byte("c2"), []byte("0"), []byte("1-0"), []byte("JUSTID"),
	}, s)
	if err := cmdXCLAIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XCLAIM_WithFORCE(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.CreateGroup("g1", "0")
	group := stream.GetGroup("g1")
	group.GetOrCreateConsumer("c1")
	group.AddPending("1-0", "c1")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XCLAIM", [][]byte{
		[]byte("s"), []byte("g1"), []byte("c2"), []byte("0"), []byte("1-0"), []byte("FORCE"),
	}, s)
	if err := cmdXCLAIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XCLAIM_NonExistentStream(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XCLAIM", [][]byte{
		[]byte("nostream"), []byte("g1"), []byte("c2"), []byte("0"), []byte("1-0"),
	}, s)
	if err := cmdXCLAIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XCLAIM_NoGroup(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XCLAIM", [][]byte{
		[]byte("s"), []byte("nogroup"), []byte("c2"), []byte("0"), []byte("1-0"),
	}, s)
	if err := cmdXCLAIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XCLAIM_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XCLAIM", [][]byte{
		[]byte("s"), []byte("g1"), []byte("c2"), []byte("0"),
	}, s)
	if err := cmdXCLAIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XAUTOCLAIM_Basic(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.CreateGroup("g1", "0")
	group := stream.GetGroup("g1")
	group.GetOrCreateConsumer("c1")
	group.AddPending("1-0", "c1")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XAUTOCLAIM", [][]byte{
		[]byte("s"), []byte("g1"), []byte("c2"), []byte("0"), []byte("0-0"),
	}, s)
	if err := cmdXAUTOCLAIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XAUTOCLAIM_WithJUSTID(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.CreateGroup("g1", "0")
	group := stream.GetGroup("g1")
	group.GetOrCreateConsumer("c1")
	group.AddPending("1-0", "c1")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XAUTOCLAIM", [][]byte{
		[]byte("s"), []byte("g1"), []byte("c2"), []byte("0"), []byte("0-0"), []byte("JUSTID"),
	}, s)
	if err := cmdXAUTOCLAIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XAUTOCLAIM_WithCount(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	stream.Add("1-0", map[string][]byte{"f": []byte("v")})
	stream.CreateGroup("g1", "0")
	group := stream.GetGroup("g1")
	group.GetOrCreateConsumer("c1")
	group.AddPending("1-0", "c1")
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XAUTOCLAIM", [][]byte{
		[]byte("s"), []byte("g1"), []byte("c2"), []byte("0"), []byte("0-0"),
		[]byte("COUNT"), []byte("5"),
	}, s)
	if err := cmdXAUTOCLAIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XAUTOCLAIM_NonExistentStream(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XAUTOCLAIM", [][]byte{
		[]byte("nostream"), []byte("g1"), []byte("c2"), []byte("0"), []byte("0-0"),
	}, s)
	if err := cmdXAUTOCLAIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XAUTOCLAIM_NoGroup(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XAUTOCLAIM", [][]byte{
		[]byte("s"), []byte("nogroup"), []byte("c2"), []byte("0"), []byte("0-0"),
	}, s)
	if err := cmdXAUTOCLAIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XAUTOCLAIM_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XAUTOCLAIM", [][]byte{
		[]byte("s"), []byte("g1"), []byte("c2"), []byte("0"),
	}, s)
	if err := cmdXAUTOCLAIM(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XSETID_Basic(t *testing.T) {
	s := store.NewStore()
	stream := store.NewStreamValue(0)
	s.Set("s", stream, store.SetOptions{})

	ctx := newDiscardContext("XSETID", [][]byte{
		[]byte("s"), []byte("5-0"),
	}, s)
	if err := cmdXSETID(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XSETID_NonExistent(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XSETID", [][]byte{
		[]byte("nostream"), []byte("5-0"),
	}, s)
	if err := cmdXSETID(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_XSETID_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("XSETID", [][]byte{[]byte("s")}, s)
	if err := cmdXSETID(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// PubSub Commands
// ---------------------------------------------------------------------------

func TestAdvCoverage_SUBSCRIBE_Single(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SUBSCRIBE", [][]byte{[]byte("ch1")}, s)
	if err := cmdSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SUBSCRIBE_Multiple(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SUBSCRIBE", [][]byte{[]byte("ch1"), []byte("ch2"), []byte("ch3")}, s)
	if err := cmdSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SUBSCRIBE_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SUBSCRIBE", nil, s)
	if err := cmdSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_UNSUBSCRIBE_WithChannels(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("UNSUBSCRIBE", [][]byte{[]byte("ch1")}, s)
	if err := cmdUNSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_UNSUBSCRIBE_NoArgs(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("UNSUBSCRIBE", nil, s)
	if err := cmdUNSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PSUBSCRIBE_Single(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("PSUBSCRIBE", [][]byte{[]byte("news.*")}, s)
	if err := cmdPSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PSUBSCRIBE_Multiple(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("PSUBSCRIBE", [][]byte{[]byte("news.*"), []byte("events.*")}, s)
	if err := cmdPSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PSUBSCRIBE_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("PSUBSCRIBE", nil, s)
	if err := cmdPSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PUNSUBSCRIBE_WithPatterns(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("PUNSUBSCRIBE", [][]byte{[]byte("news.*")}, s)
	if err := cmdPUNSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PUNSUBSCRIBE_NoArgs(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("PUNSUBSCRIBE", nil, s)
	if err := cmdPUNSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PUBLISH_Basic(t *testing.T) {
	s := store.NewStore()
	ps := s.GetPubSub()
	sub := store.NewSubscriber(999)
	ps.Subscribe(sub, "ch1")

	ctx := newDiscardContext("PUBLISH", [][]byte{[]byte("ch1"), []byte("hello")}, s)
	if err := cmdPUBLISH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PUBLISH_NoSubscribers(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("PUBLISH", [][]byte{[]byte("ch1"), []byte("hello")}, s)
	if err := cmdPUBLISH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PUBLISH_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("PUBLISH", [][]byte{[]byte("ch1")}, s)
	if err := cmdPUBLISH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PUBSUB_CHANNELS(t *testing.T) {
	s := store.NewStore()
	ps := s.GetPubSub()
	sub := store.NewSubscriber(100)
	ps.Subscribe(sub, "ch1")
	ps.Subscribe(sub, "ch2")

	ctx := newDiscardContext("PUBSUB", [][]byte{[]byte("CHANNELS")}, s)
	if err := cmdPUBSUB(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PUBSUB_CHANNELS_WithPattern(t *testing.T) {
	s := store.NewStore()
	ps := s.GetPubSub()
	sub := store.NewSubscriber(101)
	ps.Subscribe(sub, "news.sports")
	ps.Subscribe(sub, "news.tech")
	ps.Subscribe(sub, "events.local")

	ctx := newDiscardContext("PUBSUB", [][]byte{[]byte("CHANNELS"), []byte("news.*")}, s)
	if err := cmdPUBSUB(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PUBSUB_NUMSUB(t *testing.T) {
	s := store.NewStore()
	ps := s.GetPubSub()
	sub1 := store.NewSubscriber(102)
	sub2 := store.NewSubscriber(103)
	ps.Subscribe(sub1, "ch1")
	ps.Subscribe(sub2, "ch1")

	ctx := newDiscardContext("PUBSUB", [][]byte{[]byte("NUMSUB"), []byte("ch1")}, s)
	if err := cmdPUBSUB(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PUBSUB_NUMSUB_Empty(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("PUBSUB", [][]byte{[]byte("NUMSUB")}, s)
	if err := cmdPUBSUB(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PUBSUB_NUMPAT(t *testing.T) {
	s := store.NewStore()
	ps := s.GetPubSub()
	sub := store.NewSubscriber(104)
	ps.PSubscribe(sub, "news.*")
	ps.PSubscribe(sub, "events.*")

	ctx := newDiscardContext("PUBSUB", [][]byte{[]byte("NUMPAT")}, s)
	if err := cmdPUBSUB(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PUBSUB_UnknownSubCmd(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("PUBSUB", [][]byte{[]byte("INVALID")}, s)
	if err := cmdPUBSUB(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_PUBSUB_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("PUBSUB", nil, s)
	if err := cmdPUBSUB(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SSUBSCRIBE(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SSUBSCRIBE", [][]byte{[]byte("ch1")}, s)
	if err := cmdSSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SSUBSCRIBE_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SSUBSCRIBE", nil, s)
	if err := cmdSSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SUNSUBSCRIBE(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SUNSUBSCRIBE", [][]byte{[]byte("ch1")}, s)
	if err := cmdSUNSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SUNSUBSCRIBE_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SUNSUBSCRIBE", nil, s)
	if err := cmdSUNSUBSCRIBE(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SPUBLISH(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SPUBLISH", [][]byte{[]byte("ch1"), []byte("msg")}, s)
	if err := cmdSPUBLISH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SPUBLISH_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SPUBLISH", [][]byte{[]byte("ch1")}, s)
	if err := cmdSPUBLISH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Transaction Commands
// ---------------------------------------------------------------------------

func TestAdvCoverage_MULTI_Basic(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("MULTI", nil, s)
	if err := cmdMULTI(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.Transaction.IsActive() {
		t.Fatal("expected transaction to be active")
	}
}

func TestAdvCoverage_MULTI_ClearsWatch(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("MULTI", nil, s)
	ctx.Transaction.Watch("key1", 1)
	if err := cmdMULTI(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.Transaction.IsActive() {
		t.Fatal("expected transaction to be active")
	}
}

func TestAdvCoverage_EXEC_WithoutMulti(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_EmptyQueue(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_WithQueuedCommands(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("SET", [][]byte{[]byte("k1"), []byte("v1")})
	ctx.Transaction.Queue("GET", [][]byte{[]byte("k1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_WatchedKeyChanged(t *testing.T) {
	s := store.NewStore()
	s.Set("wk", &store.StringValue{Data: []byte("initial")}, store.SetOptions{})

	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Watch("wk", s.GetVersion("wk"))
	// Simulate a change that bumps the version
	s.Set("wk", &store.StringValue{Data: []byte("changed")}, store.SetOptions{})
	ctx.Transaction.Start()
	ctx.Transaction.Queue("SET", [][]byte{[]byte("wk"), []byte("new")})

	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_WatchedKeyUnchanged(t *testing.T) {
	s := store.NewStore()
	s.Set("wk", &store.StringValue{Data: []byte("initial")}, store.SetOptions{})

	ctx := newDiscardContext("EXEC", nil, s)
	ver := s.GetVersion("wk")
	ctx.Transaction.Watch("wk", ver)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("SET", [][]byte{[]byte("wk"), []byte("new")})

	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_DISCARD_Basic(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("DISCARD", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("SET", [][]byte{[]byte("k"), []byte("v")})
	if err := cmdDISCARD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Transaction.IsActive() {
		t.Fatal("expected transaction to be inactive after DISCARD")
	}
}

func TestAdvCoverage_DISCARD_WithoutMulti(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("DISCARD", nil, s)
	if err := cmdDISCARD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_WATCH_Basic(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})

	ctx := newDiscardContext("WATCH", [][]byte{[]byte("k1")}, s)
	if err := cmdWATCH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctx.Transaction.HasWatchedKeys() {
		t.Fatal("expected watched keys")
	}
}

func TestAdvCoverage_WATCH_MultipleKeys(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("WATCH", [][]byte{[]byte("k1"), []byte("k2"), []byte("k3")}, s)
	if err := cmdWATCH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_WATCH_InsideMulti(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("WATCH", [][]byte{[]byte("k1")}, s)
	ctx.Transaction.Start()
	if err := cmdWATCH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_UNWATCH_Basic(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("UNWATCH", nil, s)
	ctx.Transaction.Watch("k1", 1)
	if err := cmdUNWATCH(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Transaction.HasWatchedKeys() {
		t.Fatal("expected no watched keys after UNWATCH")
	}
}

func TestAdvCoverage_EXEC_QueuedDEL(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("DEL", [][]byte{[]byte("k1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedINCR(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("INCR", [][]byte{[]byte("counter")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedDECR(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("DECR", [][]byte{[]byte("counter")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedUNLINK(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("UNLINK", [][]byte{[]byte("k1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedEXISTS(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("EXISTS", [][]byte{[]byte("k1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedAPPEND(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("hello")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("APPEND", [][]byte{[]byte("k1"), []byte(" world")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedSTRLEN(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("hello")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("STRLEN", [][]byte{[]byte("k1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedMSET(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("MSET", [][]byte{[]byte("k1"), []byte("v1"), []byte("k2"), []byte("v2")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedMGET(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("MGET", [][]byte{[]byte("k1"), []byte("k2")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedSETNX(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("SETNX", [][]byte{[]byte("k1"), []byte("v1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedSETEX(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("SETEX", [][]byte{[]byte("k1"), []byte("10"), []byte("v1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedGETSET(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("old")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("GETSET", [][]byte{[]byte("k1"), []byte("new")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedRENAME(t *testing.T) {
	s := store.NewStore()
	s.Set("old", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("RENAME", [][]byte{[]byte("old"), []byte("new")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedRENAMENX(t *testing.T) {
	s := store.NewStore()
	s.Set("old", &store.StringValue{Data: []byte("v")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("RENAMENX", [][]byte{[]byte("old"), []byte("new")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedHSET(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("HSET", [][]byte{[]byte("h1"), []byte("f1"), []byte("v1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedHGET(t *testing.T) {
	s := store.NewStore()
	hv := &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1")}}
	s.Set("h1", hv, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("HGET", [][]byte{[]byte("h1"), []byte("f1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedHDEL(t *testing.T) {
	s := store.NewStore()
	hv := &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1")}}
	s.Set("h1", hv, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("HDEL", [][]byte{[]byte("h1"), []byte("f1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedHEXISTS(t *testing.T) {
	s := store.NewStore()
	hv := &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1")}}
	s.Set("h1", hv, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("HEXISTS", [][]byte{[]byte("h1"), []byte("f1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedHLEN(t *testing.T) {
	s := store.NewStore()
	hv := &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1")}}
	s.Set("h1", hv, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("HLEN", [][]byte{[]byte("h1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedLPUSH_RPUSH(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("LPUSH", [][]byte{[]byte("list"), []byte("a"), []byte("b")})
	ctx.Transaction.Queue("RPUSH", [][]byte{[]byte("list"), []byte("c")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedLPOP_RPOP(t *testing.T) {
	s := store.NewStore()
	lv := &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}
	s.Set("list", lv, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("LPOP", [][]byte{[]byte("list")})
	ctx.Transaction.Queue("RPOP", [][]byte{[]byte("list")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedLLEN(t *testing.T) {
	s := store.NewStore()
	lv := &store.ListValue{Elements: [][]byte{[]byte("a")}}
	s.Set("list", lv, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("LLEN", [][]byte{[]byte("list")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedSADD_SREM(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("SADD", [][]byte{[]byte("set"), []byte("a"), []byte("b")})
	ctx.Transaction.Queue("SREM", [][]byte{[]byte("set"), []byte("a")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedSCARD_SISMEMBER(t *testing.T) {
	s := store.NewStore()
	sv := &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}
	s.Set("set", sv, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("SCARD", [][]byte{[]byte("set")})
	ctx.Transaction.Queue("SISMEMBER", [][]byte{[]byte("set"), []byte("a")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedZADD_ZREM(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("ZADD", [][]byte{[]byte("zset"), []byte("1"), []byte("a")})
	ctx.Transaction.Queue("ZREM", [][]byte{[]byte("zset"), []byte("a")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedZCARD_ZSCORE(t *testing.T) {
	s := store.NewStore()
	zv := &store.SortedSetValue{Members: map[string]float64{"a": 1.0}}
	s.Set("zset", zv, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("ZCARD", [][]byte{[]byte("zset")})
	ctx.Transaction.Queue("ZSCORE", [][]byte{[]byte("zset"), []byte("a")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedPING(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("PING", nil)
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedECHO(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("ECHO", [][]byte{[]byte("hello")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedDBSIZE(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("DBSIZE", nil)
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedFLUSHDB(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("FLUSHDB", nil)
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedTYPE(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("TYPE", [][]byte{[]byte("k1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedEXPIRE(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("EXPIRE", [][]byte{[]byte("k1"), []byte("60")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedPEXPIRE(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("PEXPIRE", [][]byte{[]byte("k1"), []byte("60000")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedTTL_PTTL(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("TTL", [][]byte{[]byte("k1")})
	ctx.Transaction.Queue("PTTL", [][]byte{[]byte("k1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedPERSIST(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("PERSIST", [][]byte{[]byte("k1")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedINCRBY(t *testing.T) {
	s := store.NewStore()
	s.Set("counter", &store.StringValue{Data: []byte("10")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("INCRBY", [][]byte{[]byte("counter"), []byte("5")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedDECRBY(t *testing.T) {
	s := store.NewStore()
	s.Set("counter", &store.StringValue{Data: []byte("10")}, store.SetOptions{})
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("DECRBY", [][]byte{[]byte("counter"), []byte("3")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EXEC_QueuedUnknown(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("EXEC", nil, s)
	ctx.Transaction.Start()
	ctx.Transaction.Queue("BADCOMMAND", [][]byte{[]byte("x")})
	if err := cmdEXEC(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Script / Lua Commands
// ---------------------------------------------------------------------------

func TestAdvCoverage_EVAL_Simple(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{
		[]byte(`return "hello"`), []byte("0"),
	}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVAL_WithKeys(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{
		[]byte(`return redis.KEYS[1]`), []byte("1"), []byte("mykey"),
	}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVAL_WithArgs(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{
		[]byte(`return redis.ARGV[1]`), []byte("0"), []byte("myarg"),
	}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVAL_ReturnInt(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{
		[]byte(`return 42`), []byte("0"),
	}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVAL_ReturnNil(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{
		[]byte(`return nil`), []byte("0"),
	}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVAL_ReturnBool(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{
		[]byte(`return true`), []byte("0"),
	}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVAL_ReturnTable(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{
		[]byte(`return {1, 2, 3}`), []byte("0"),
	}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVAL_RedisCall_SET_GET(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{
		[]byte(`redis.call("SET", KEYS[1], ARGV[1]); return redis.call("GET", KEYS[1])`),
		[]byte("1"), []byte("testkey"), []byte("testval"),
	}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVAL_RedisCall_INCR(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{
		[]byte(`return redis.call("INCR", KEYS[1])`),
		[]byte("1"), []byte("counter"),
	}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVAL_RedisCall_DEL(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{
		[]byte(`return redis.call("DEL", KEYS[1])`),
		[]byte("1"), []byte("k1"),
	}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVAL_RedisCall_EXISTS(t *testing.T) {
	s := store.NewStore()
	s.Set("k1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{
		[]byte(`return redis.call("EXISTS", KEYS[1])`),
		[]byte("1"), []byte("k1"),
	}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVAL_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{[]byte(`return 1`)}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVAL_InvalidNumKeys(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{[]byte(`return 1`), []byte("notanumber")}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVAL_SyntaxError(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVAL", [][]byte{[]byte(`invalid lua!!!`), []byte("0")}, s)
	if err := cmdEVAL(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVALSHA_Nonexistent(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVALSHA", [][]byte{[]byte("abc123"), []byte("0")}, s)
	if err := cmdEVALSHA(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVALSHA_Valid(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	sha := scriptEngine.ScriptLoad(`return "loaded"`)

	ctx := newDiscardContext("EVALSHA", [][]byte{[]byte(sha), []byte("0")}, s)
	if err := cmdEVALSHA(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVALSHA_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVALSHA", [][]byte{[]byte("abc123")}, s)
	if err := cmdEVALSHA(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_EVALSHA_InvalidNumKeys(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("EVALSHA", [][]byte{[]byte("abc"), []byte("x")}, s)
	if err := cmdEVALSHA(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SCRIPT_LOAD(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("SCRIPT", [][]byte{[]byte("LOAD"), []byte(`return 1+1`)}, s)
	if err := cmdSCRIPT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SCRIPT_EXISTS(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	sha := scriptEngine.ScriptLoad(`return 1`)

	ctx := newDiscardContext("SCRIPT", [][]byte{[]byte("EXISTS"), []byte(sha)}, s)
	if err := cmdSCRIPT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SCRIPT_EXISTS_Multiple(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	sha := scriptEngine.ScriptLoad(`return 1`)

	ctx := newDiscardContext("SCRIPT", [][]byte{[]byte("EXISTS"), []byte(sha), []byte("nonexistent")}, s)
	if err := cmdSCRIPT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SCRIPT_FLUSH(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	scriptEngine.ScriptLoad(`return 1`)

	ctx := newDiscardContext("SCRIPT", [][]byte{[]byte("FLUSH")}, s)
	if err := cmdSCRIPT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SCRIPT_DEBUG(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("SCRIPT", [][]byte{[]byte("DEBUG")}, s)
	if err := cmdSCRIPT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SCRIPT_KILL(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("SCRIPT", [][]byte{[]byte("KILL")}, s)
	if err := cmdSCRIPT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SCRIPT_Unknown(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("SCRIPT", [][]byte{[]byte("BADCMD")}, s)
	if err := cmdSCRIPT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SCRIPT_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("SCRIPT", nil, s)
	if err := cmdSCRIPT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SCRIPT_LOAD_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("SCRIPT", [][]byte{[]byte("LOAD")}, s)
	if err := cmdSCRIPT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SCRIPT_EXISTS_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	scriptEngine = NewScriptEngine(s)
	ctx := newDiscardContext("SCRIPT", [][]byte{[]byte("EXISTS")}, s)
	if err := cmdSCRIPT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_ScriptEngine_Operations(t *testing.T) {
	s := store.NewStore()
	engine := NewScriptEngine(s)

	// ScriptLoad and ScriptExists
	sha := engine.ScriptLoad(`return "test"`)
	if !engine.ScriptExists(sha) {
		t.Fatal("expected script to exist")
	}
	if engine.ScriptExists("nonexistent") {
		t.Fatal("did not expect script to exist")
	}

	// ScriptFlush
	engine.ScriptFlush()
	if engine.ScriptExists(sha) {
		t.Fatal("expected script to not exist after flush")
	}

	// EvalSHA nonexistent
	_, err := engine.EvalSHA("nonexistent", nil, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent SHA")
	}

	// Eval with SET/GET via redis.KEYS / redis.ARGV
	sha = engine.ScriptLoad(`redis.call("SET", redis.KEYS[1], redis.ARGV[1]); return redis.call("GET", redis.KEYS[1])`)
	result, err := engine.EvalSHA(sha, []string{"testkey"}, []string{"testval"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "testval" {
		t.Fatalf("expected 'testval', got %v", result)
	}
}

func TestAdvCoverage_ScriptSHA(t *testing.T) {
	sha1 := ScriptSHA("return 1")
	sha2 := ScriptSHA("return 2")
	if sha1 == sha2 {
		t.Fatal("different scripts should have different SHA")
	}
	sha1b := ScriptSHA("return 1")
	if sha1 != sha1b {
		t.Fatal("same script should produce same SHA")
	}
}

// ---------------------------------------------------------------------------
// Bitmap Commands
// ---------------------------------------------------------------------------

func TestAdvCoverage_SETBIT_Basic(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SETBIT", [][]byte{
		[]byte("bm"), []byte("7"), []byte("1"),
	}, s)
	if err := cmdSETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SETBIT_Clear(t *testing.T) {
	s := store.NewStore()
	// First set a bit, then clear it
	bm := &BitmapValue{Data: []byte{0xFF}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("SETBIT", [][]byte{
		[]byte("bm"), []byte("0"), []byte("0"),
	}, s)
	if err := cmdSETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SETBIT_ExtendData(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SETBIT", [][]byte{
		[]byte("bm"), []byte("100"), []byte("1"),
	}, s)
	if err := cmdSETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SETBIT_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SETBIT", [][]byte{[]byte("bm"), []byte("7")}, s)
	if err := cmdSETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SETBIT_InvalidOffset(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SETBIT", [][]byte{
		[]byte("bm"), []byte("notanumber"), []byte("1"),
	}, s)
	if err := cmdSETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SETBIT_NegativeOffset(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SETBIT", [][]byte{
		[]byte("bm"), []byte("-1"), []byte("1"),
	}, s)
	if err := cmdSETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SETBIT_InvalidBitValue(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("SETBIT", [][]byte{
		[]byte("bm"), []byte("0"), []byte("2"),
	}, s)
	if err := cmdSETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SETBIT_WrongType(t *testing.T) {
	s := store.NewStore()
	s.Set("list", &store.ListValue{Elements: [][]byte{[]byte("a")}}, store.SetOptions{})
	ctx := newDiscardContext("SETBIT", [][]byte{
		[]byte("list"), []byte("0"), []byte("1"),
	}, s)
	if err := cmdSETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_SETBIT_OnStringValue(t *testing.T) {
	s := store.NewStore()
	s.Set("str", &store.StringValue{Data: []byte{0x00}}, store.SetOptions{})
	ctx := newDiscardContext("SETBIT", [][]byte{
		[]byte("str"), []byte("0"), []byte("1"),
	}, s)
	if err := cmdSETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_GETBIT_Basic(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0x01}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("GETBIT", [][]byte{
		[]byte("bm"), []byte("0"),
	}, s)
	if err := cmdGETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_GETBIT_NonExistent(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("GETBIT", [][]byte{
		[]byte("nokey"), []byte("0"),
	}, s)
	if err := cmdGETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_GETBIT_OutOfRange(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0x01}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("GETBIT", [][]byte{
		[]byte("bm"), []byte("100"),
	}, s)
	if err := cmdGETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_GETBIT_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("GETBIT", [][]byte{[]byte("bm")}, s)
	if err := cmdGETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_GETBIT_InvalidOffset(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("GETBIT", [][]byte{
		[]byte("bm"), []byte("notanumber"),
	}, s)
	if err := cmdGETBIT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITCOUNT_Basic(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0xFF, 0x0F}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITCOUNT", [][]byte{[]byte("bm")}, s)
	if err := cmdBITCOUNT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITCOUNT_WithRange(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0xFF, 0x0F, 0x00}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITCOUNT", [][]byte{
		[]byte("bm"), []byte("0"), []byte("1"),
	}, s)
	if err := cmdBITCOUNT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITCOUNT_NegativeRange(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0xFF, 0x0F}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITCOUNT", [][]byte{
		[]byte("bm"), []byte("-2"), []byte("-1"),
	}, s)
	if err := cmdBITCOUNT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITCOUNT_NonExistent(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITCOUNT", [][]byte{[]byte("nokey")}, s)
	if err := cmdBITCOUNT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITCOUNT_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITCOUNT", nil, s)
	if err := cmdBITCOUNT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITCOUNT_InvalidRange(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0xFF}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITCOUNT", [][]byte{
		[]byte("bm"), []byte("notanumber"), []byte("0"),
	}, s)
	if err := cmdBITCOUNT(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITPOS_FindOne(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0x00, 0x01}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITPOS", [][]byte{
		[]byte("bm"), []byte("1"),
	}, s)
	if err := cmdBITPOS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITPOS_FindZero(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0xFF, 0xFE}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITPOS", [][]byte{
		[]byte("bm"), []byte("0"),
	}, s)
	if err := cmdBITPOS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITPOS_WithRange(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0x00, 0x00, 0x01}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITPOS", [][]byte{
		[]byte("bm"), []byte("1"), []byte("1"), []byte("2"),
	}, s)
	if err := cmdBITPOS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITPOS_NonExistent(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITPOS", [][]byte{
		[]byte("nokey"), []byte("1"),
	}, s)
	if err := cmdBITPOS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITPOS_EmptyBitmap(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITPOS", [][]byte{
		[]byte("bm"), []byte("1"),
	}, s)
	if err := cmdBITPOS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITPOS_EmptyFindZero(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITPOS", [][]byte{
		[]byte("bm"), []byte("0"),
	}, s)
	if err := cmdBITPOS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITPOS_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITPOS", [][]byte{[]byte("bm")}, s)
	if err := cmdBITPOS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITPOS_InvalidBit(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITPOS", [][]byte{
		[]byte("bm"), []byte("2"),
	}, s)
	if err := cmdBITPOS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITPOS_AllOnes(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0xFF, 0xFF}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITPOS", [][]byte{
		[]byte("bm"), []byte("0"),
	}, s)
	if err := cmdBITPOS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITPOS_AllZeros(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0x00, 0x00}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITPOS", [][]byte{
		[]byte("bm"), []byte("1"),
	}, s)
	if err := cmdBITPOS(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITOP_AND(t *testing.T) {
	s := store.NewStore()
	s.Set("a", &BitmapValue{Data: []byte{0xFF, 0x0F}}, store.SetOptions{})
	s.Set("b", &BitmapValue{Data: []byte{0x0F, 0xFF}}, store.SetOptions{})

	ctx := newDiscardContext("BITOP", [][]byte{
		[]byte("AND"), []byte("dest"), []byte("a"), []byte("b"),
	}, s)
	if err := cmdBITOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITOP_OR(t *testing.T) {
	s := store.NewStore()
	s.Set("a", &BitmapValue{Data: []byte{0xF0}}, store.SetOptions{})
	s.Set("b", &BitmapValue{Data: []byte{0x0F}}, store.SetOptions{})

	ctx := newDiscardContext("BITOP", [][]byte{
		[]byte("OR"), []byte("dest"), []byte("a"), []byte("b"),
	}, s)
	if err := cmdBITOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITOP_XOR(t *testing.T) {
	s := store.NewStore()
	s.Set("a", &BitmapValue{Data: []byte{0xFF}}, store.SetOptions{})
	s.Set("b", &BitmapValue{Data: []byte{0xFF}}, store.SetOptions{})

	ctx := newDiscardContext("BITOP", [][]byte{
		[]byte("XOR"), []byte("dest"), []byte("a"), []byte("b"),
	}, s)
	if err := cmdBITOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITOP_NOT(t *testing.T) {
	s := store.NewStore()
	s.Set("a", &BitmapValue{Data: []byte{0xF0}}, store.SetOptions{})

	ctx := newDiscardContext("BITOP", [][]byte{
		[]byte("NOT"), []byte("dest"), []byte("a"),
	}, s)
	if err := cmdBITOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITOP_NonExistentSrc(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITOP", [][]byte{
		[]byte("AND"), []byte("dest"), []byte("nokey1"), []byte("nokey2"),
	}, s)
	if err := cmdBITOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITOP_DiffLengths(t *testing.T) {
	s := store.NewStore()
	s.Set("a", &BitmapValue{Data: []byte{0xFF}}, store.SetOptions{})
	s.Set("b", &BitmapValue{Data: []byte{0x0F, 0xFF, 0x00}}, store.SetOptions{})

	ctx := newDiscardContext("BITOP", [][]byte{
		[]byte("OR"), []byte("dest"), []byte("a"), []byte("b"),
	}, s)
	if err := cmdBITOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITOP_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITOP", [][]byte{[]byte("AND"), []byte("dest")}, s)
	if err := cmdBITOP(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_GET(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0xFF, 0x00}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("GET"), []byte("u8"), []byte("0"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_SET(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("SET"), []byte("u8"), []byte("0"), []byte("255"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_INCRBY(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0x00, 0x00}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("INCRBY"), []byte("u8"), []byte("0"), []byte("10"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_OVERFLOW_SAT(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0xFF}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("OVERFLOW"), []byte("SAT"),
		[]byte("INCRBY"), []byte("u8"), []byte("0"), []byte("10"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_OVERFLOW_FAIL(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0xFF}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("OVERFLOW"), []byte("FAIL"),
		[]byte("INCRBY"), []byte("u8"), []byte("0"), []byte("10"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_OVERFLOW_WRAP(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: []byte{0xFF}}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("OVERFLOW"), []byte("WRAP"),
		[]byte("INCRBY"), []byte("u8"), []byte("0"), []byte("10"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_SignedType(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("SET"), []byte("i8"), []byte("0"), []byte("-5"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_16bit(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("SET"), []byte("u16"), []byte("0"), []byte("1000"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_MultipleOps(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"),
		[]byte("SET"), []byte("u8"), []byte("0"), []byte("100"),
		[]byte("GET"), []byte("u8"), []byte("0"),
		[]byte("INCRBY"), []byte("u8"), []byte("0"), []byte("5"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITFIELD", nil, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_InvalidSubCmd(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("BADOP"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_GET_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("GET"), []byte("u8"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_SET_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("SET"), []byte("u8"), []byte("0"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_INCRBY_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("INCRBY"), []byte("u8"), []byte("0"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_OVERFLOW_Invalid(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("OVERFLOW"), []byte("INVALID"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_OVERFLOW_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("OVERFLOW"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_WrongType(t *testing.T) {
	s := store.NewStore()
	s.Set("list", &store.ListValue{Elements: [][]byte{[]byte("a")}}, store.SetOptions{})
	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("list"), []byte("GET"), []byte("u8"), []byte("0"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_InvalidEncoding(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("GET"), []byte("u99"), []byte("0"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_BITFIELD_INCRBY_Signed(t *testing.T) {
	s := store.NewStore()
	bm := &BitmapValue{Data: make([]byte, 4)}
	s.Set("bm", bm, store.SetOptions{})

	ctx := newDiscardContext("BITFIELD", [][]byte{
		[]byte("bm"), []byte("INCRBY"), []byte("i8"), []byte("0"), []byte("-1"),
	}, s)
	if err := cmdBITFIELD(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Lua ScriptEngine executeCommand coverage
// ---------------------------------------------------------------------------

func TestAdvCoverage_ScriptEngine_RedisCommands(t *testing.T) {
	s := store.NewStore()
	engine := NewScriptEngine(s)

	// HSET / HGET / HGETALL / HLEN / HEXISTS / HDEL
	result, err := engine.Eval(`
		redis.call("HSET", "h1", "f1", "v1")
		local v = redis.call("HGET", "h1", "f1")
		redis.call("HGETALL", "h1")
		redis.call("HLEN", "h1")
		redis.call("HEXISTS", "h1", "f1")
		redis.call("HDEL", "h1", "f1")
		return v
	`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "v1" {
		t.Fatalf("expected 'v1', got %v", result)
	}

	// LPUSH / RPUSH / LPOP / RPOP / LLEN / LRANGE / LINDEX
	_, err = engine.Eval(`
		redis.call("LPUSH", "list", "a")
		redis.call("RPUSH", "list", "b")
		redis.call("LLEN", "list")
		redis.call("LRANGE", "list", "0", "-1")
		redis.call("LINDEX", "list", "0")
		redis.call("LPOP", "list")
		redis.call("RPOP", "list")
		return 1
	`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SADD / SISMEMBER / SCARD / SMEMBERS / SREM
	_, err = engine.Eval(`
		redis.call("SADD", "set", "a")
		redis.call("SISMEMBER", "set", "a")
		redis.call("SCARD", "set")
		redis.call("SMEMBERS", "set")
		redis.call("SREM", "set", "a")
		return 1
	`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ZADD / ZSCORE / ZCARD / ZRANGE / ZREM
	_, err = engine.Eval(`
		redis.call("ZADD", "zset", "1", "a")
		redis.call("ZSCORE", "zset", "a")
		redis.call("ZCARD", "zset")
		redis.call("ZRANGE", "zset", "0", "-1")
		redis.call("ZREM", "zset", "a")
		return 1
	`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// TYPE / DBSIZE / FLUSHDB
	_, err = engine.Eval(`
		redis.call("SET", "k1", "v1")
		redis.call("TYPE", "k1")
		redis.call("DBSIZE")
		redis.call("FLUSHDB")
		return 1
	`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_ScriptEngine_StringOps(t *testing.T) {
	s := store.NewStore()
	engine := NewScriptEngine(s)

	// APPEND / STRLEN / MSET / MGET / SETNX / GETSET / RENAME
	_, err := engine.Eval(`
		redis.call("SET", "k1", "hello")
		redis.call("APPEND", "k1", " world")
		redis.call("STRLEN", "k1")
		redis.call("MSET", "a", "1", "b", "2")
		redis.call("MGET", "a", "b")
		redis.call("SETNX", "c", "3")
		redis.call("GETSET", "a", "new")
		redis.call("RENAME", "a", "d")
		return 1
	`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_ScriptEngine_NumericOps(t *testing.T) {
	s := store.NewStore()
	engine := NewScriptEngine(s)

	_, err := engine.Eval(`
		redis.call("SET", "n", "10")
		redis.call("INCR", "n")
		redis.call("DECR", "n")
		redis.call("INCRBY", "n", "5")
		redis.call("DECRBY", "n", "3")
		return redis.call("GET", "n")
	`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_ScriptEngine_TTLOps(t *testing.T) {
	s := store.NewStore()
	engine := NewScriptEngine(s)

	_, err := engine.Eval(`
		redis.call("SET", "k1", "v1")
		redis.call("EXPIRE", "k1", "60")
		redis.call("TTL", "k1")
		redis.call("PERSIST", "k1")
		return 1
	`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_ScriptEngine_HMOps(t *testing.T) {
	s := store.NewStore()
	engine := NewScriptEngine(s)

	_, err := engine.Eval(`
		redis.call("HMSET", "h1", "f1", "v1", "f2", "v2")
		redis.call("HMGET", "h1", "f1", "f2")
		redis.call("HKEYS", "h1")
		redis.call("HVALS", "h1")
		return 1
	`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_ScriptEngine_PCall(t *testing.T) {
	s := store.NewStore()
	engine := NewScriptEngine(s)

	// pcall should not fail even with unknown commands
	_, err := engine.Eval(`
		redis.pcall("SET", "k1", "v1")
		return redis.pcall("GET", "k1")
	`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_ScriptEngine_ErrorReply(t *testing.T) {
	s := store.NewStore()
	engine := NewScriptEngine(s)

	_, err := engine.Eval(`
		return redis.error_reply("something went wrong")
	`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdvCoverage_ScriptEngine_StatusReply(t *testing.T) {
	s := store.NewStore()
	engine := NewScriptEngine(s)

	_, err := engine.Eval(`
		return redis.status_reply("OK")
	`, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// generateStreamID coverage
// ---------------------------------------------------------------------------

func TestAdvCoverage_GenerateStreamID(t *testing.T) {
	// empty lastID
	id := generateStreamID("")
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	// 0-0
	id = generateStreamID("0-0")
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	// valid lastID
	id = generateStreamID("1000-5")
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	// invalid lastID format
	id = generateStreamID("invalid")
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	// invalid parts
	id = generateStreamID("abc-def")
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
}

// ---------------------------------------------------------------------------
// BitmapValue type methods coverage
// ---------------------------------------------------------------------------

func TestAdvCoverage_BitmapValue_Methods(t *testing.T) {
	bm := &BitmapValue{Data: []byte{0xFF, 0x0F}}

	if bm.Type() != store.DataTypeString {
		t.Fatalf("expected string type, got %v", bm.Type())
	}
	if bm.SizeOf() <= 0 {
		t.Fatal("expected positive size")
	}
	if bm.String() == "" {
		// Data is non-printable; just make sure it doesn't panic
	}
	cloned := bm.Clone()
	if cloned == nil {
		t.Fatal("expected non-nil clone")
	}
}

// ---------------------------------------------------------------------------
// Transaction helper coverage
// ---------------------------------------------------------------------------

func TestAdvCoverage_Transaction_CheckWatchedVersions(t *testing.T) {
	tx := NewTransaction()
	tx.Watch("k1", 1)
	tx.Watch("k2", 2)

	// versions match
	ok := tx.CheckWatchedVersions(func(key string) int64 {
		switch key {
		case "k1":
			return 1
		case "k2":
			return 2
		}
		return 0
	})
	if !ok {
		t.Fatal("expected versions to match")
	}

	// versions don't match
	ok = tx.CheckWatchedVersions(func(key string) int64 {
		return 999
	})
	if ok {
		t.Fatal("expected versions to not match")
	}
}

func TestAdvCoverage_Transaction_QueueAndGet(t *testing.T) {
	tx := NewTransaction()
	tx.Start()
	tx.Queue("SET", [][]byte{[]byte("a"), []byte("b")})
	tx.Queue("GET", [][]byte{[]byte("a")})

	queued := tx.GetQueued()
	if len(queued) != 2 {
		t.Fatalf("expected 2 queued, got %d", len(queued))
	}

	tx.Clear()
	queued = tx.GetQueued()
	if len(queued) != 0 {
		t.Fatalf("expected 0 queued after clear, got %d", len(queued))
	}
}

// ---------------------------------------------------------------------------
// writeLuaResult / goValueToResp coverage
// ---------------------------------------------------------------------------

func TestAdvCoverage_WriteLuaResult_Types(t *testing.T) {
	s := store.NewStore()

	tests := []struct {
		name   string
		result interface{}
	}{
		{"nil", nil},
		{"string", "hello"},
		{"int", 42},
		{"int64", int64(42)},
		{"float64_int", float64(42)},
		{"float64_frac", float64(3.14)},
		{"bool_true", true},
		{"bool_false", false},
		{"slice", []interface{}{"a", int64(1), nil, true}},
		{"unknown", struct{ x int }{1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newDiscardContext("EVAL", nil, s)
			if err := writeLuaResult(ctx, tt.result); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAdvCoverage_GoValueToResp_Types(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
	}{
		{"nil", nil},
		{"string", "hello"},
		{"int", 42},
		{"int64", int64(42)},
		{"float64_int", float64(42)},
		{"float64_frac", float64(3.14)},
		{"bool_true", true},
		{"bool_false", false},
		{"slice", []interface{}{"a", int64(1)}},
		{"unknown", struct{ x int }{1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := goValueToResp(tt.value)
			if v == nil {
				t.Fatal("expected non-nil value")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseEncoding coverage
// ---------------------------------------------------------------------------

func TestAdvCoverage_ParseEncoding(t *testing.T) {
	tests := []struct {
		enc  string
		want int
	}{
		{"u8", 8}, {"i8", 8},
		{"u16", 16}, {"i16", 16},
		{"u32", 32}, {"i32", 32},
		{"u64", 64}, {"i64", 64},
		{"u99", 0}, {"", 0}, {"xyz", 0},
	}
	for _, tt := range tests {
		got := parseEncoding(tt.enc)
		if got != tt.want {
			t.Errorf("parseEncoding(%q) = %d, want %d", tt.enc, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Context helper methods coverage
// ---------------------------------------------------------------------------

func TestAdvCoverage_ContextHelpers(t *testing.T) {
	s := store.NewStore()
	ctx := newDiscardContext("TEST", [][]byte{[]byte("a"), []byte("b")}, s)

	// Arg / ArgString / ArgCount
	if string(ctx.Arg(0)) != "a" {
		t.Fatal("expected 'a'")
	}
	if ctx.ArgString(1) != "b" {
		t.Fatal("expected 'b'")
	}
	if ctx.Arg(5) != nil {
		t.Fatal("expected nil for out-of-bounds Arg")
	}
	if ctx.Arg(-1) != nil {
		t.Fatal("expected nil for negative Arg")
	}
	if ctx.ArgCount() != 2 {
		t.Fatalf("expected 2, got %d", ctx.ArgCount())
	}

	// IsAuthenticated / SetAuthenticated
	if ctx.IsAuthenticated() {
		t.Fatal("expected not authenticated")
	}
	ctx.SetAuthenticated(true)
	if !ctx.IsAuthenticated() {
		t.Fatal("expected authenticated")
	}

	// GetTransaction
	tx := ctx.GetTransaction()
	if tx == nil {
		t.Fatal("expected non-nil transaction")
	}

	// GetSubscriber
	sub := ctx.GetSubscriber()
	if sub == nil {
		t.Fatal("expected non-nil subscriber")
	}

	// Write methods
	if err := ctx.WriteOK(); err != nil {
		t.Fatal(err)
	}
	if err := ctx.WriteInteger(42); err != nil {
		t.Fatal(err)
	}
	if err := ctx.WriteNull(); err != nil {
		t.Fatal(err)
	}
	if err := ctx.WriteNullBulkString(); err != nil {
		t.Fatal(err)
	}
	if err := ctx.WriteBulkString("test"); err != nil {
		t.Fatal(err)
	}
	if err := ctx.WriteBulkBytes([]byte("test")); err != nil {
		t.Fatal(err)
	}
	if err := ctx.WriteSimpleString("OK"); err != nil {
		t.Fatal(err)
	}
	if err := ctx.WriteArray([]*resp.Value{resp.IntegerValue(1)}); err != nil {
		t.Fatal(err)
	}
	if err := ctx.WriteError(ErrWrongArgCount); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// mapInt64/mapInt/mapString/mapUint coverage
// ---------------------------------------------------------------------------

func TestAdvCoverage_MapHelpers(t *testing.T) {
	m := map[string]interface{}{
		"i64":  int64(42),
		"i":    int(7),
		"s":    "hello",
		"u":    uint(99),
		"none": nil,
	}

	if mapInt64(m, "i64") != 42 {
		t.Error("expected 42")
	}
	if mapInt64(m, "missing") != 0 {
		t.Error("expected 0 for missing")
	}

	if mapInt(m, "i") != 7 {
		t.Error("expected 7")
	}
	if mapInt(m, "missing") != 0 {
		t.Error("expected 0 for missing")
	}

	if mapString(m, "s") != "hello" {
		t.Error("expected hello")
	}
	if mapString(m, "missing") != "" {
		t.Error("expected empty for missing")
	}

	if mapUint(m, "u") != 99 {
		t.Error("expected 99")
	}
	if mapUint(m, "missing") != 0 {
		t.Error("expected 0 for missing")
	}
}

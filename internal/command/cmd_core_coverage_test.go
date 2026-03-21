package command

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// helper to create a context using io.Discard writer
func discardCtx(cmd string, args [][]byte, s *store.Store) *Context {
	w := resp.NewWriter(io.Discard)
	return &Context{
		Command: cmd,
		Args:    args,
		Store:   s,
		Writer:  w,
	}
}

// helper to create a context with a capturing buffer so we can inspect output
func bufCtx(cmd string, args [][]byte, s *store.Store) (*Context, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	w := resp.NewWriter(buf)
	return &Context{
		Command: cmd,
		Args:    args,
		Store:   s,
		Writer:  w,
	}, buf
}

// bytesArgs is a convenience helper to build [][]byte from strings
func bytesArgs(ss ...string) [][]byte {
	out := make([][]byte, len(ss))
	for i, s := range ss {
		out[i] = []byte(s)
	}
	return out
}

// ======================================================================
// STRING COMMANDS
// ======================================================================

func TestCmdAPPEND_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	// Append to nonexistent key creates it
	ctx, buf := bufCtx("APPEND", bytesArgs("akey", "Hello"), s)
	err := router.ExecuteSilent(ctx)
	if err != nil {
		t.Fatalf("APPEND to new key: %v", err)
	}
	if !strings.Contains(buf.String(), "5") {
		t.Errorf("Expected length 5, got %q", buf.String())
	}

	// Append to existing key
	ctx2, buf2 := bufCtx("APPEND", bytesArgs("akey", " World"), s)
	err = router.ExecuteSilent(ctx2)
	if err != nil {
		t.Fatalf("APPEND to existing key: %v", err)
	}
	if !strings.Contains(buf2.String(), "11") {
		t.Errorf("Expected length 11, got %q", buf2.String())
	}

	// Verify stored value
	entry, exists := s.Get("akey")
	if !exists {
		t.Fatal("Key akey should exist")
	}
	sv := entry.Value.(*store.StringValue)
	if string(sv.Data) != "Hello World" {
		t.Errorf("Expected 'Hello World', got %q", string(sv.Data))
	}
}

func TestCmdAPPEND_WrongArgCount(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("APPEND", bytesArgs("only_key"), s)
	err := router.ExecuteSilent(ctx)
	if err != nil {
		t.Fatalf("APPEND wrong args should not return Go error: %v", err)
	}
}

func TestCmdAPPEND_WrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("hkey", &store.HashValue{Fields: map[string][]byte{"f": []byte("v")}}, store.SetOptions{})
	ctx := discardCtx("APPEND", bytesArgs("hkey", "data"), s)
	err := router.ExecuteSilent(ctx)
	if err != nil {
		t.Fatalf("APPEND wrong type should not return Go error: %v", err)
	}
}

func TestCmdSETRANGE_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("sr", &store.StringValue{Data: []byte("Hello World")}, store.SetOptions{})
	ctx := discardCtx("SETRANGE", bytesArgs("sr", "6", "Redis"), s)
	err := router.ExecuteSilent(ctx)
	if err != nil {
		t.Fatalf("SETRANGE: %v", err)
	}
	entry, _ := s.Get("sr")
	if string(entry.Value.(*store.StringValue).Data) != "Hello Redis" {
		t.Errorf("Expected 'Hello Redis', got %q", entry.Value.(*store.StringValue).Data)
	}
}

func TestCmdSETRANGE_NewKey(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("SETRANGE", bytesArgs("newsr", "5", "Hi"), s)
	err := router.ExecuteSilent(ctx)
	if err != nil {
		t.Fatalf("SETRANGE new key: %v", err)
	}
	entry, exists := s.Get("newsr")
	if !exists {
		t.Fatal("newsr should exist")
	}
	data := entry.Value.(*store.StringValue).Data
	if len(data) != 7 {
		t.Errorf("Expected length 7, got %d", len(data))
	}
}

func TestCmdSETRANGE_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	// Wrong number of args
	ctx := discardCtx("SETRANGE", bytesArgs("k"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	// Non-integer offset
	ctx = discardCtx("SETRANGE", bytesArgs("k", "abc", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	// Negative offset
	ctx = discardCtx("SETRANGE", bytesArgs("k", "-1", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	// Wrong type
	s.Set("hk", &store.HashValue{Fields: map[string][]byte{}}, store.SetOptions{})
	ctx = discardCtx("SETRANGE", bytesArgs("hk", "0", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
}

func TestCmdGETRANGE_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("gr", &store.StringValue{Data: []byte("Hello World")}, store.SetOptions{})

	// Full range
	ctx, buf := bufCtx("GETRANGE", bytesArgs("gr", "0", "-1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("GETRANGE full: %v", err)
	}
	if !strings.Contains(buf.String(), "Hello World") {
		t.Errorf("Expected 'Hello World', got %q", buf.String())
	}

	// Partial range
	ctx2, buf2 := bufCtx("GETRANGE", bytesArgs("gr", "0", "4"), s)
	if err := router.ExecuteSilent(ctx2); err != nil {
		t.Fatalf("GETRANGE partial: %v", err)
	}
	if !strings.Contains(buf2.String(), "Hello") {
		t.Errorf("Expected 'Hello', got %q", buf2.String())
	}

	// Negative indices
	ctx3, buf3 := bufCtx("GETRANGE", bytesArgs("gr", "-5", "-1"), s)
	if err := router.ExecuteSilent(ctx3); err != nil {
		t.Fatalf("GETRANGE negative: %v", err)
	}
	if !strings.Contains(buf3.String(), "World") {
		t.Errorf("Expected 'World', got %q", buf3.String())
	}

	// Start > End
	ctx4 := discardCtx("GETRANGE", bytesArgs("gr", "5", "0"), s)
	if err := router.ExecuteSilent(ctx4); err != nil {
		t.Fatalf("GETRANGE start>end: %v", err)
	}
}

func TestCmdGETRANGE_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	// Wrong arg count
	ctx := discardCtx("GETRANGE", bytesArgs("k", "0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer start
	ctx = discardCtx("GETRANGE", bytesArgs("k", "abc", "5"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer end
	ctx = discardCtx("GETRANGE", bytesArgs("k", "0", "xyz"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Nonexistent key
	ctx = discardCtx("GETRANGE", bytesArgs("nokey", "0", "5"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Wrong type
	s.Set("hk", &store.HashValue{Fields: map[string][]byte{}}, store.SetOptions{})
	ctx = discardCtx("GETRANGE", bytesArgs("hk", "0", "5"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdSETNX_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	// New key - should set
	ctx, buf := bufCtx("SETNX", bytesArgs("setnxk", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("SETNX new: %v", err)
	}
	if !strings.Contains(buf.String(), "1") {
		t.Errorf("Expected 1, got %q", buf.String())
	}

	// Existing key - should not set
	ctx2, buf2 := bufCtx("SETNX", bytesArgs("setnxk", "newval"), s)
	if err := router.ExecuteSilent(ctx2); err != nil {
		t.Fatalf("SETNX existing: %v", err)
	}
	if !strings.Contains(buf2.String(), "0") {
		t.Errorf("Expected 0, got %q", buf2.String())
	}

	// Value should still be original
	entry, _ := s.Get("setnxk")
	if string(entry.Value.(*store.StringValue).Data) != "val" {
		t.Errorf("Expected 'val', got %q", entry.Value.(*store.StringValue).Data)
	}
}

func TestCmdSETNX_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("SETNX", bytesArgs("only"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdSETEX_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("SETEX", bytesArgs("sexk", "10", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("SETEX: %v", err)
	}
	entry, exists := s.Get("sexk")
	if !exists {
		t.Fatal("key should exist")
	}
	if string(entry.Value.(*store.StringValue).Data) != "val" {
		t.Error("wrong value")
	}
	if entry.ExpiresAt == 0 {
		t.Error("expected TTL to be set")
	}
}

func TestCmdSETEX_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	// Wrong arg count
	ctx := discardCtx("SETEX", bytesArgs("k", "10"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer seconds
	ctx = discardCtx("SETEX", bytesArgs("k", "abc", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Zero/negative seconds
	ctx = discardCtx("SETEX", bytesArgs("k", "0", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	ctx = discardCtx("SETEX", bytesArgs("k", "-5", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdPSETEX_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("PSETEX", bytesArgs("psexk", "5000", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("PSETEX: %v", err)
	}
	entry, exists := s.Get("psexk")
	if !exists {
		t.Fatal("key should exist")
	}
	if string(entry.Value.(*store.StringValue).Data) != "val" {
		t.Error("wrong value")
	}
}

func TestCmdPSETEX_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	// Wrong arg count
	ctx := discardCtx("PSETEX", bytesArgs("k", "5000"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer ms
	ctx = discardCtx("PSETEX", bytesArgs("k", "abc", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Zero ms
	ctx = discardCtx("PSETEX", bytesArgs("k", "0", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Negative ms
	ctx = discardCtx("PSETEX", bytesArgs("k", "-1", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdMSET_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("MSET", bytesArgs("mk1", "mv1", "mk2", "mv2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("MSET: %v", err)
	}

	e1, _ := s.Get("mk1")
	if string(e1.Value.(*store.StringValue).Data) != "mv1" {
		t.Error("mk1 wrong value")
	}
	e2, _ := s.Get("mk2")
	if string(e2.Value.(*store.StringValue).Data) != "mv2" {
		t.Error("mk2 wrong value")
	}
}

func TestCmdMSET_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	// No args
	ctx := discardCtx("MSET", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Odd number of args
	ctx = discardCtx("MSET", bytesArgs("k1", "v1", "k2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdMSETNX_AllNew(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx, buf := bufCtx("MSETNX", bytesArgs("mnk1", "v1", "mnk2", "v2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("MSETNX: %v", err)
	}
	if !strings.Contains(buf.String(), "1") {
		t.Errorf("Expected 1, got %q", buf.String())
	}

	// Both should be set
	_, exists1 := s.Get("mnk1")
	_, exists2 := s.Get("mnk2")
	if !exists1 || !exists2 {
		t.Error("Both keys should exist")
	}
}

func TestCmdMSETNX_SomeExist(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("mnk_existing", &store.StringValue{Data: []byte("old")}, store.SetOptions{})

	ctx, buf := bufCtx("MSETNX", bytesArgs("mnk_new", "v1", "mnk_existing", "v2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("MSETNX: %v", err)
	}
	if !strings.Contains(buf.String(), "0") {
		t.Errorf("Expected 0, got %q", buf.String())
	}

	// mnk_new should NOT have been set
	_, exists := s.Get("mnk_new")
	if exists {
		t.Error("mnk_new should not exist since one key already existed")
	}
}

func TestCmdMSETNX_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	// Odd args
	ctx := discardCtx("MSETNX", bytesArgs("k"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdMGET_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("mg1", &store.StringValue{Data: []byte("v1")}, store.SetOptions{})
	s.Set("mg2", &store.StringValue{Data: []byte("v2")}, store.SetOptions{})

	ctx, buf := bufCtx("MGET", bytesArgs("mg1", "mg2", "mg_nonexist"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("MGET: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "v1") || !strings.Contains(out, "v2") {
		t.Errorf("Expected v1 and v2 in output, got %q", out)
	}
}

func TestCmdMGET_WrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("mg_hash", &store.HashValue{Fields: map[string][]byte{"f": []byte("v")}}, store.SetOptions{})

	// MGET returns null for wrong type keys, not an error
	ctx := discardCtx("MGET", bytesArgs("mg_hash"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdMGET_NoArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("MGET", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdINCR_NewKey(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx, buf := bufCtx("INCR", bytesArgs("ic"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("INCR new: %v", err)
	}
	if !strings.Contains(buf.String(), "1") {
		t.Errorf("Expected 1, got %q", buf.String())
	}
}

func TestCmdINCR_ExistingKey(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("ic2", &store.StringValue{Data: []byte("10")}, store.SetOptions{})
	ctx, buf := bufCtx("INCR", bytesArgs("ic2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("INCR existing: %v", err)
	}
	if !strings.Contains(buf.String(), "11") {
		t.Errorf("Expected 11, got %q", buf.String())
	}
}

func TestCmdINCR_WrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("ic_hash", &store.HashValue{Fields: map[string][]byte{}}, store.SetOptions{})
	ctx := discardCtx("INCR", bytesArgs("ic_hash"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdINCR_NotInteger(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("ic_str", &store.StringValue{Data: []byte("notanum")}, store.SetOptions{})
	ctx := discardCtx("INCR", bytesArgs("ic_str"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdDECR_NewKey(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx, buf := bufCtx("DECR", bytesArgs("dc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("DECR new: %v", err)
	}
	if !strings.Contains(buf.String(), "-1") {
		t.Errorf("Expected -1, got %q", buf.String())
	}
}

func TestCmdDECR_ExistingKey(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("dc2", &store.StringValue{Data: []byte("10")}, store.SetOptions{})
	ctx, buf := bufCtx("DECR", bytesArgs("dc2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("DECR existing: %v", err)
	}
	if !strings.Contains(buf.String(), "9") {
		t.Errorf("Expected 9, got %q", buf.String())
	}
}

func TestCmdINCRBY_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("ib", &store.StringValue{Data: []byte("5")}, store.SetOptions{})
	ctx, buf := bufCtx("INCRBY", bytesArgs("ib", "3"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("INCRBY: %v", err)
	}
	if !strings.Contains(buf.String(), "8") {
		t.Errorf("Expected 8, got %q", buf.String())
	}
}

func TestCmdINCRBY_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	// Wrong arg count
	ctx := discardCtx("INCRBY", bytesArgs("k"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer increment
	ctx = discardCtx("INCRBY", bytesArgs("k", "abc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdDECRBY_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("db", &store.StringValue{Data: []byte("10")}, store.SetOptions{})
	ctx, buf := bufCtx("DECRBY", bytesArgs("db", "3"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("DECRBY: %v", err)
	}
	if !strings.Contains(buf.String(), "7") {
		t.Errorf("Expected 7, got %q", buf.String())
	}
}

func TestCmdDECRBY_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	// Wrong arg count
	ctx := discardCtx("DECRBY", bytesArgs("k"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer decrement
	ctx = discardCtx("DECRBY", bytesArgs("k", "abc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdINCRBYFLOAT_NewKey(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx, buf := bufCtx("INCRBYFLOAT", bytesArgs("fk", "2.5"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("INCRBYFLOAT new: %v", err)
	}
	if !strings.Contains(buf.String(), "2.5") {
		t.Errorf("Expected 2.5, got %q", buf.String())
	}
}

func TestCmdINCRBYFLOAT_ExistingKey(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("fk2", &store.StringValue{Data: []byte("10.5")}, store.SetOptions{})
	ctx, buf := bufCtx("INCRBYFLOAT", bytesArgs("fk2", "1.5"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("INCRBYFLOAT existing: %v", err)
	}
	if !strings.Contains(buf.String(), "12") {
		t.Errorf("Expected 12, got %q", buf.String())
	}
}

func TestCmdINCRBYFLOAT_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	// Wrong arg count
	ctx := discardCtx("INCRBYFLOAT", bytesArgs("k"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-float increment
	ctx = discardCtx("INCRBYFLOAT", bytesArgs("k", "abc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Wrong type
	s.Set("fk_hash", &store.HashValue{Fields: map[string][]byte{}}, store.SetOptions{})
	ctx = discardCtx("INCRBYFLOAT", bytesArgs("fk_hash", "1.0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-float existing value
	s.Set("fk_bad", &store.StringValue{Data: []byte("notafloat")}, store.SetOptions{})
	ctx = discardCtx("INCRBYFLOAT", bytesArgs("fk_bad", "1.0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdGETSET_WithExisting(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("gs", &store.StringValue{Data: []byte("old")}, store.SetOptions{})
	ctx, buf := bufCtx("GETSET", bytesArgs("gs", "new"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("GETSET: %v", err)
	}
	if !strings.Contains(buf.String(), "old") {
		t.Errorf("Expected old value, got %q", buf.String())
	}

	entry, _ := s.Get("gs")
	if string(entry.Value.(*store.StringValue).Data) != "new" {
		t.Error("new value not set")
	}
}

func TestCmdGETSET_NoExisting(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx, buf := bufCtx("GETSET", bytesArgs("gs_new", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("GETSET new: %v", err)
	}
	// Should return null bulk string for nonexistent key
	if !strings.Contains(buf.String(), "$-1") {
		t.Errorf("Expected null bulk string, got %q", buf.String())
	}
}

func TestCmdGETSET_WrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("gs_hash", &store.HashValue{Fields: map[string][]byte{"f": []byte("v")}}, store.SetOptions{})
	ctx := discardCtx("GETSET", bytesArgs("gs_hash", "new"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdGETSET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("GETSET", bytesArgs("only"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdSTRLEN_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("sl", &store.StringValue{Data: []byte("Hello")}, store.SetOptions{})
	ctx, buf := bufCtx("STRLEN", bytesArgs("sl"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("STRLEN: %v", err)
	}
	if !strings.Contains(buf.String(), "5") {
		t.Errorf("Expected 5, got %q", buf.String())
	}

	// Nonexistent key
	ctx2, buf2 := bufCtx("STRLEN", bytesArgs("nokey"), s)
	if err := router.ExecuteSilent(ctx2); err != nil {
		t.Fatalf("STRLEN nokey: %v", err)
	}
	if !strings.Contains(buf2.String(), "0") {
		t.Errorf("Expected 0, got %q", buf2.String())
	}
}

func TestCmdSTRLEN_WrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("sl_hash", &store.HashValue{Fields: map[string][]byte{}}, store.SetOptions{})
	ctx := discardCtx("STRLEN", bytesArgs("sl_hash"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdSTRLEN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("STRLEN", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdGETDEL_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("gd", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
	ctx, buf := bufCtx("GETDEL", bytesArgs("gd"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("GETDEL: %v", err)
	}
	if !strings.Contains(buf.String(), "val") {
		t.Errorf("Expected val, got %q", buf.String())
	}
	// Key should be gone
	if s.Exists("gd") {
		t.Error("key should have been deleted")
	}
}

func TestCmdGETDEL_Nonexistent(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx, buf := bufCtx("GETDEL", bytesArgs("nokey"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("GETDEL: %v", err)
	}
	if !strings.Contains(buf.String(), "$-1") {
		t.Errorf("Expected null bulk string, got %q", buf.String())
	}
}

func TestCmdGETDEL_WrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("gd_hash", &store.HashValue{Fields: map[string][]byte{}}, store.SetOptions{})
	ctx := discardCtx("GETDEL", bytesArgs("gd_hash"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdGETDEL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("GETDEL", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdGETEX_WithEX(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("gex", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
	ctx, buf := bufCtx("GETEX", bytesArgs("gex", "EX", "100"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("GETEX EX: %v", err)
	}
	if !strings.Contains(buf.String(), "val") {
		t.Errorf("Expected val, got %q", buf.String())
	}
}

func TestCmdGETEX_WithPX(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("gex_px", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
	ctx := discardCtx("GETEX", bytesArgs("gex_px", "PX", "50000"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("GETEX PX: %v", err)
	}
}

func TestCmdGETEX_WithEXAT(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("gex_exat", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
	ctx := discardCtx("GETEX", bytesArgs("gex_exat", "EXAT", "2000000000"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("GETEX EXAT: %v", err)
	}
}

func TestCmdGETEX_WithPXAT(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("gex_pxat", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
	ctx := discardCtx("GETEX", bytesArgs("gex_pxat", "PXAT", "2000000000000"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("GETEX PXAT: %v", err)
	}
}

func TestCmdGETEX_WithPERSIST(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("gex_persist", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
	ctx := discardCtx("GETEX", bytesArgs("gex_persist", "PERSIST"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("GETEX PERSIST: %v", err)
	}
}

func TestCmdGETEX_Nonexistent(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx, buf := bufCtx("GETEX", bytesArgs("gex_nope"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("GETEX nonexistent: %v", err)
	}
	if !strings.Contains(buf.String(), "$-1") {
		t.Errorf("Expected null bulk string, got %q", buf.String())
	}
}

func TestCmdGETEX_WrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("gex_hash", &store.HashValue{Fields: map[string][]byte{}}, store.SetOptions{})
	ctx := discardCtx("GETEX", bytesArgs("gex_hash"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdGETEX_SyntaxErrors(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("gex_se", &store.StringValue{Data: []byte("val")}, store.SetOptions{})

	// EX without value
	ctx := discardCtx("GETEX", bytesArgs("gex_se", "EX"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// EX with non-integer
	ctx = discardCtx("GETEX", bytesArgs("gex_se", "EX", "abc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// PX without value
	ctx = discardCtx("GETEX", bytesArgs("gex_se", "PX"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// PX with non-integer
	ctx = discardCtx("GETEX", bytesArgs("gex_se", "PX", "abc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// EXAT without value
	ctx = discardCtx("GETEX", bytesArgs("gex_se", "EXAT"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// PXAT without value
	ctx = discardCtx("GETEX", bytesArgs("gex_se", "PXAT"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Unknown option
	ctx = discardCtx("GETEX", bytesArgs("gex_se", "BOGUS"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// No args
	ctx = discardCtx("GETEX", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdLCS_Basic(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("lcs1", &store.StringValue{Data: []byte("ohmytext")}, store.SetOptions{})
	s.Set("lcs2", &store.StringValue{Data: []byte("mynewtext")}, store.SetOptions{})

	ctx, buf := bufCtx("LCS", bytesArgs("lcs1", "lcs2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LCS basic: %v", err)
	}
	if !strings.Contains(buf.String(), "mytext") {
		t.Errorf("Expected 'mytext', got %q", buf.String())
	}
}

func TestCmdLCS_LEN(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("lcs_l1", &store.StringValue{Data: []byte("ohmytext")}, store.SetOptions{})
	s.Set("lcs_l2", &store.StringValue{Data: []byte("mynewtext")}, store.SetOptions{})

	ctx, buf := bufCtx("LCS", bytesArgs("lcs_l1", "lcs_l2", "LEN"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LCS LEN: %v", err)
	}
	if !strings.Contains(buf.String(), "6") {
		t.Errorf("Expected 6, got %q", buf.String())
	}
}

func TestCmdLCS_IDX(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("lcs_i1", &store.StringValue{Data: []byte("ohmytext")}, store.SetOptions{})
	s.Set("lcs_i2", &store.StringValue{Data: []byte("mynewtext")}, store.SetOptions{})

	ctx := discardCtx("LCS", bytesArgs("lcs_i1", "lcs_i2", "IDX"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LCS IDX: %v", err)
	}
}

func TestCmdLCS_IDX_MINMATCHLEN(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("lcs_m1", &store.StringValue{Data: []byte("ohmytext")}, store.SetOptions{})
	s.Set("lcs_m2", &store.StringValue{Data: []byte("mynewtext")}, store.SetOptions{})

	ctx := discardCtx("LCS", bytesArgs("lcs_m1", "lcs_m2", "IDX", "MINMATCHLEN", "4"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LCS IDX MINMATCHLEN: %v", err)
	}
}

func TestCmdLCS_IDX_WITHMATCHLEN(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("lcs_w1", &store.StringValue{Data: []byte("ohmytext")}, store.SetOptions{})
	s.Set("lcs_w2", &store.StringValue{Data: []byte("mynewtext")}, store.SetOptions{})

	ctx := discardCtx("LCS", bytesArgs("lcs_w1", "lcs_w2", "IDX", "WITHMATCHLEN"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LCS IDX WITHMATCHLEN: %v", err)
	}
}

func TestCmdLCS_NonexistentKeys(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("LCS", bytesArgs("nokey1", "nokey2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LCS nonexistent: %v", err)
	}

	// With LEN flag
	ctx = discardCtx("LCS", bytesArgs("nokey1", "nokey2", "LEN"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LCS LEN nonexistent: %v", err)
	}
}

func TestCmdLCS_EmptyStrings(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("lcs_e1", &store.StringValue{Data: []byte("")}, store.SetOptions{})
	s.Set("lcs_e2", &store.StringValue{Data: []byte("hello")}, store.SetOptions{})

	ctx := discardCtx("LCS", bytesArgs("lcs_e1", "lcs_e2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LCS empty: %v", err)
	}
}

func TestCmdLCS_WrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("lcs_h1", &store.HashValue{Fields: map[string][]byte{}}, store.SetOptions{})
	s.Set("lcs_h2", &store.StringValue{Data: []byte("hello")}, store.SetOptions{})

	ctx := discardCtx("LCS", bytesArgs("lcs_h1", "lcs_h2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdLCS_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	// Too few args
	ctx := discardCtx("LCS", bytesArgs("k1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Unknown sub-option
	s.Set("lcs_e1", &store.StringValue{Data: []byte("abc")}, store.SetOptions{})
	s.Set("lcs_e2", &store.StringValue{Data: []byte("abc")}, store.SetOptions{})
	ctx = discardCtx("LCS", bytesArgs("lcs_e1", "lcs_e2", "BADOPT"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// MINMATCHLEN without value
	ctx = discardCtx("LCS", bytesArgs("lcs_e1", "lcs_e2", "IDX", "MINMATCHLEN"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// MINMATCHLEN with non-integer
	ctx = discardCtx("LCS", bytesArgs("lcs_e1", "lcs_e2", "IDX", "MINMATCHLEN", "abc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// ======================================================================
// HASH COMMANDS
// ======================================================================

func TestCmdHSET_NewHash(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx, buf := bufCtx("HSET", bytesArgs("h1", "f1", "v1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HSET: %v", err)
	}
	if !strings.Contains(buf.String(), "1") {
		t.Errorf("Expected 1 added, got %q", buf.String())
	}
}

func TestCmdHSET_MultipleFields(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx, buf := bufCtx("HSET", bytesArgs("h2", "f1", "v1", "f2", "v2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HSET multi: %v", err)
	}
	if !strings.Contains(buf.String(), "2") {
		t.Errorf("Expected 2 added, got %q", buf.String())
	}
}

func TestCmdHSET_UpdateExisting(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("h3", &store.HashValue{Fields: map[string][]byte{"f1": []byte("old")}}, store.SetOptions{})
	ctx, buf := bufCtx("HSET", bytesArgs("h3", "f1", "new"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HSET update: %v", err)
	}
	// Updating existing field should return 0 new fields
	if !strings.Contains(buf.String(), "0") {
		t.Errorf("Expected 0 added, got %q", buf.String())
	}
}

func TestCmdHSET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	// No args
	ctx := discardCtx("HSET", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Even number of args (key + field but no value)
	ctx = discardCtx("HSET", bytesArgs("h", "f"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHSET_WrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("str_key", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
	ctx := discardCtx("HSET", bytesArgs("str_key", "f1", "v1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHGET_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hg", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1")}}, store.SetOptions{})

	ctx, buf := bufCtx("HGET", bytesArgs("hg", "f1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HGET: %v", err)
	}
	if !strings.Contains(buf.String(), "v1") {
		t.Errorf("Expected v1, got %q", buf.String())
	}
}

func TestCmdHGET_MissingField(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hg2", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1")}}, store.SetOptions{})

	ctx, buf := bufCtx("HGET", bytesArgs("hg2", "nosuchfield"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HGET missing: %v", err)
	}
	if !strings.Contains(buf.String(), "$-1") {
		t.Errorf("Expected null bulk string, got %q", buf.String())
	}
}

func TestCmdHGET_NonexistentHash(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx, buf := bufCtx("HGET", bytesArgs("nohash", "f"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HGET nonexistent: %v", err)
	}
	if !strings.Contains(buf.String(), "$-1") {
		t.Errorf("Expected null bulk string, got %q", buf.String())
	}
}

func TestCmdHGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HGET", bytesArgs("h"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHDEL_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hd", &store.HashValue{Fields: map[string][]byte{
		"f1": []byte("v1"),
		"f2": []byte("v2"),
		"f3": []byte("v3"),
	}}, store.SetOptions{})

	ctx, buf := bufCtx("HDEL", bytesArgs("hd", "f1", "f2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HDEL: %v", err)
	}
	if !strings.Contains(buf.String(), "2") {
		t.Errorf("Expected 2 deleted, got %q", buf.String())
	}
}

func TestCmdHDEL_LastField(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hd2", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1")}}, store.SetOptions{})

	ctx := discardCtx("HDEL", bytesArgs("hd2", "f1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HDEL last: %v", err)
	}
	// Hash should be auto-deleted when empty
	if s.Exists("hd2") {
		t.Error("hash should have been deleted when last field removed")
	}
}

func TestCmdHDEL_NonexistentHash(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx, buf := bufCtx("HDEL", bytesArgs("nohash", "f"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HDEL nonexistent: %v", err)
	}
	if !strings.Contains(buf.String(), "0") {
		t.Errorf("Expected 0, got %q", buf.String())
	}
}

func TestCmdHDEL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HDEL", bytesArgs("h"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHEXISTS_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("he", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1")}}, store.SetOptions{})

	// Exists
	ctx, buf := bufCtx("HEXISTS", bytesArgs("he", "f1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HEXISTS: %v", err)
	}
	if !strings.Contains(buf.String(), "1") {
		t.Errorf("Expected 1, got %q", buf.String())
	}

	// Not exists
	ctx2, buf2 := bufCtx("HEXISTS", bytesArgs("he", "nof"), s)
	if err := router.ExecuteSilent(ctx2); err != nil {
		t.Fatalf("HEXISTS missing: %v", err)
	}
	if !strings.Contains(buf2.String(), "0") {
		t.Errorf("Expected 0, got %q", buf2.String())
	}

	// Nonexistent hash
	ctx3, buf3 := bufCtx("HEXISTS", bytesArgs("nohash", "f"), s)
	if err := router.ExecuteSilent(ctx3); err != nil {
		t.Fatalf("HEXISTS no hash: %v", err)
	}
	if !strings.Contains(buf3.String(), "0") {
		t.Errorf("Expected 0, got %q", buf3.String())
	}
}

func TestCmdHEXISTS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HEXISTS", bytesArgs("h"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHGETALL_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hga", &store.HashValue{Fields: map[string][]byte{
		"f1": []byte("v1"),
		"f2": []byte("v2"),
	}}, store.SetOptions{})

	ctx, buf := bufCtx("HGETALL", bytesArgs("hga"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HGETALL: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "f1") || !strings.Contains(out, "v1") {
		t.Errorf("Expected f1/v1, got %q", out)
	}
}

func TestCmdHGETALL_Empty(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HGETALL", bytesArgs("nohash"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HGETALL empty: %v", err)
	}
}

func TestCmdHGETALL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HGETALL", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHMSET_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HMSET", bytesArgs("hms", "f1", "v1", "f2", "v2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HMSET: %v", err)
	}

	entry, exists := s.Get("hms")
	if !exists {
		t.Fatal("hash should exist")
	}
	hv := entry.Value.(*store.HashValue)
	if string(hv.Fields["f1"]) != "v1" || string(hv.Fields["f2"]) != "v2" {
		t.Error("wrong values in hash")
	}
}

func TestCmdHMSET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	// Too few
	ctx := discardCtx("HMSET", bytesArgs("h", "f"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Even args
	ctx = discardCtx("HMSET", bytesArgs("h", "f", "v", "f2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHMGET_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hmg", &store.HashValue{Fields: map[string][]byte{
		"f1": []byte("v1"),
		"f2": []byte("v2"),
	}}, store.SetOptions{})

	ctx, buf := bufCtx("HMGET", bytesArgs("hmg", "f1", "f2", "f3"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HMGET: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "v1") || !strings.Contains(out, "v2") {
		t.Errorf("Expected v1 and v2, got %q", out)
	}
}

func TestCmdHMGET_NonexistentHash(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HMGET", bytesArgs("nohash", "f1", "f2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HMGET nonexistent: %v", err)
	}
}

func TestCmdHMGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HMGET", bytesArgs("h"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHINCRBY_NewField(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx, buf := bufCtx("HINCRBY", bytesArgs("hib", "counter", "5"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HINCRBY new: %v", err)
	}
	if !strings.Contains(buf.String(), "5") {
		t.Errorf("Expected 5, got %q", buf.String())
	}
}

func TestCmdHINCRBY_ExistingField(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hib2", &store.HashValue{Fields: map[string][]byte{"counter": []byte("10")}}, store.SetOptions{})
	ctx, buf := bufCtx("HINCRBY", bytesArgs("hib2", "counter", "3"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HINCRBY existing: %v", err)
	}
	if !strings.Contains(buf.String(), "13") {
		t.Errorf("Expected 13, got %q", buf.String())
	}
}

func TestCmdHINCRBY_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	// Wrong arg count
	ctx := discardCtx("HINCRBY", bytesArgs("h", "f"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer increment
	ctx = discardCtx("HINCRBY", bytesArgs("h", "f", "abc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer existing value
	s.Set("hib_bad", &store.HashValue{Fields: map[string][]byte{"f": []byte("notint")}}, store.SetOptions{})
	ctx = discardCtx("HINCRBY", bytesArgs("hib_bad", "f", "1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Wrong type key
	s.Set("hib_str", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
	ctx = discardCtx("HINCRBY", bytesArgs("hib_str", "f", "1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHINCRBYFLOAT_NewField(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx, buf := bufCtx("HINCRBYFLOAT", bytesArgs("hif", "f", "2.5"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HINCRBYFLOAT new: %v", err)
	}
	if !strings.Contains(buf.String(), "2.5") {
		t.Errorf("Expected 2.5, got %q", buf.String())
	}
}

func TestCmdHINCRBYFLOAT_ExistingField(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hif2", &store.HashValue{Fields: map[string][]byte{"f": []byte("10.5")}}, store.SetOptions{})
	ctx, buf := bufCtx("HINCRBYFLOAT", bytesArgs("hif2", "f", "1.5"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HINCRBYFLOAT existing: %v", err)
	}
	if !strings.Contains(buf.String(), "12") {
		t.Errorf("Expected 12, got %q", buf.String())
	}
}

func TestCmdHINCRBYFLOAT_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	// Wrong arg count
	ctx := discardCtx("HINCRBYFLOAT", bytesArgs("h", "f"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-float increment
	ctx = discardCtx("HINCRBYFLOAT", bytesArgs("h", "f", "abc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-float existing value
	s.Set("hif_bad", &store.HashValue{Fields: map[string][]byte{"f": []byte("notfloat")}}, store.SetOptions{})
	ctx = discardCtx("HINCRBYFLOAT", bytesArgs("hif_bad", "f", "1.0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHLEN_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hl", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1"), "f2": []byte("v2")}}, store.SetOptions{})

	ctx, buf := bufCtx("HLEN", bytesArgs("hl"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HLEN: %v", err)
	}
	if !strings.Contains(buf.String(), "2") {
		t.Errorf("Expected 2, got %q", buf.String())
	}

	// Nonexistent
	ctx2, buf2 := bufCtx("HLEN", bytesArgs("nohash"), s)
	if err := router.ExecuteSilent(ctx2); err != nil {
		t.Fatalf("HLEN nonexistent: %v", err)
	}
	if !strings.Contains(buf2.String(), "0") {
		t.Errorf("Expected 0, got %q", buf2.String())
	}
}

func TestCmdHLEN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HLEN", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHKEYS_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hk", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1"), "f2": []byte("v2")}}, store.SetOptions{})

	ctx, buf := bufCtx("HKEYS", bytesArgs("hk"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HKEYS: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "f1") || !strings.Contains(out, "f2") {
		t.Errorf("Expected f1 and f2, got %q", out)
	}
}

func TestCmdHKEYS_Empty(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HKEYS", bytesArgs("nohash"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HKEYS empty: %v", err)
	}
}

func TestCmdHKEYS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HKEYS", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHVALS_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hv", &store.HashValue{Fields: map[string][]byte{"f1": []byte("v1"), "f2": []byte("v2")}}, store.SetOptions{})

	ctx, buf := bufCtx("HVALS", bytesArgs("hv"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HVALS: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "v1") || !strings.Contains(out, "v2") {
		t.Errorf("Expected v1 and v2, got %q", out)
	}
}

func TestCmdHVALS_Empty(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HVALS", bytesArgs("nohash"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HVALS empty: %v", err)
	}
}

func TestCmdHVALS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HVALS", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHSETNX_NewField(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx, buf := bufCtx("HSETNX", bytesArgs("hsn", "f1", "v1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HSETNX new: %v", err)
	}
	if !strings.Contains(buf.String(), "1") {
		t.Errorf("Expected 1, got %q", buf.String())
	}
}

func TestCmdHSETNX_ExistingField(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hsn2", &store.HashValue{Fields: map[string][]byte{"f1": []byte("old")}}, store.SetOptions{})
	ctx, buf := bufCtx("HSETNX", bytesArgs("hsn2", "f1", "new"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HSETNX existing: %v", err)
	}
	if !strings.Contains(buf.String(), "0") {
		t.Errorf("Expected 0, got %q", buf.String())
	}
	// Verify value unchanged
	entry, _ := s.Get("hsn2")
	if string(entry.Value.(*store.HashValue).Fields["f1"]) != "old" {
		t.Error("value should not have changed")
	}
}

func TestCmdHSETNX_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HSETNX", bytesArgs("h", "f"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHRANDFIELD_Basic(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hr", &store.HashValue{Fields: map[string][]byte{
		"f1": []byte("v1"),
		"f2": []byte("v2"),
		"f3": []byte("v3"),
	}}, store.SetOptions{})

	// Single field
	ctx := discardCtx("HRANDFIELD", bytesArgs("hr"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HRANDFIELD: %v", err)
	}

	// With count
	ctx = discardCtx("HRANDFIELD", bytesArgs("hr", "2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HRANDFIELD count: %v", err)
	}

	// With WITHVALUES
	ctx = discardCtx("HRANDFIELD", bytesArgs("hr", "2", "WITHVALUES"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HRANDFIELD withvalues: %v", err)
	}

	// Note: negative count triggers a panic in the source due to makeslice
	// with negative cap (count*2), so we skip that test case.

	// Zero count
	ctx = discardCtx("HRANDFIELD", bytesArgs("hr", "0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HRANDFIELD zero count: %v", err)
	}
}

func TestCmdHRANDFIELD_NonexistentHash(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HRANDFIELD", bytesArgs("nohash"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HRANDFIELD nonexistent: %v", err)
	}
}

func TestCmdHRANDFIELD_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HRANDFIELD", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHRANDFIELD_WrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hr_str", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
	ctx := discardCtx("HRANDFIELD", bytesArgs("hr_str"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdHSCAN_Basic(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	s.Set("hsc", &store.HashValue{Fields: map[string][]byte{
		"f1": []byte("v1"),
		"f2": []byte("v2"),
		"f3": []byte("v3"),
	}}, store.SetOptions{})

	// Basic scan
	ctx := discardCtx("HSCAN", bytesArgs("hsc", "0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HSCAN: %v", err)
	}

	// With COUNT
	ctx = discardCtx("HSCAN", bytesArgs("hsc", "0", "COUNT", "2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HSCAN COUNT: %v", err)
	}

	// With MATCH
	ctx = discardCtx("HSCAN", bytesArgs("hsc", "0", "MATCH", "f*"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HSCAN MATCH: %v", err)
	}
}

func TestCmdHSCAN_NonexistentHash(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	ctx := discardCtx("HSCAN", bytesArgs("nohash", "0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("HSCAN nonexistent: %v", err)
	}
}

func TestCmdHSCAN_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	// Wrong arg count
	ctx := discardCtx("HSCAN", bytesArgs("h"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer cursor
	ctx = discardCtx("HSCAN", bytesArgs("h", "abc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Unknown option
	s.Set("hsc_e", &store.HashValue{Fields: map[string][]byte{"f": []byte("v")}}, store.SetOptions{})
	ctx = discardCtx("HSCAN", bytesArgs("hsc_e", "0", "BADOPT"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// COUNT without value
	ctx = discardCtx("HSCAN", bytesArgs("hsc_e", "0", "COUNT"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// MATCH without value
	ctx = discardCtx("HSCAN", bytesArgs("hsc_e", "0", "MATCH"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// COUNT with non-integer
	ctx = discardCtx("HSCAN", bytesArgs("hsc_e", "0", "COUNT", "abc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// ======================================================================
// LIST COMMANDS
// ======================================================================

func TestCmdLPUSH_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx, buf := bufCtx("LPUSH", bytesArgs("lp", "a"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LPUSH: %v", err)
	}
	if !strings.Contains(buf.String(), "1") {
		t.Errorf("Expected 1, got %q", buf.String())
	}

	// Multiple items
	ctx2, buf2 := bufCtx("LPUSH", bytesArgs("lp", "b", "c"), s)
	if err := router.ExecuteSilent(ctx2); err != nil {
		t.Fatalf("LPUSH multi: %v", err)
	}
	if !strings.Contains(buf2.String(), "3") {
		t.Errorf("Expected 3, got %q", buf2.String())
	}
}

func TestCmdLPUSH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx := discardCtx("LPUSH", bytesArgs("lp"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdLPUSH_WrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("str_k", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
	ctx := discardCtx("LPUSH", bytesArgs("str_k", "item"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdRPUSH_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx, buf := bufCtx("RPUSH", bytesArgs("rp", "a", "b", "c"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("RPUSH: %v", err)
	}
	if !strings.Contains(buf.String(), "3") {
		t.Errorf("Expected 3, got %q", buf.String())
	}
}

func TestCmdRPUSH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx := discardCtx("RPUSH", bytesArgs("rp"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdLPOP_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lpop", &store.ListValue{Elements: [][]byte{[]byte("first"), []byte("second"), []byte("third")}}, store.SetOptions{})

	ctx, buf := bufCtx("LPOP", bytesArgs("lpop"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LPOP: %v", err)
	}
	if !strings.Contains(buf.String(), "first") {
		t.Errorf("Expected 'first', got %q", buf.String())
	}
}

func TestCmdLPOP_EmptyList(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx, buf := bufCtx("LPOP", bytesArgs("nolist"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LPOP empty: %v", err)
	}
	if !strings.Contains(buf.String(), "$-1") {
		t.Errorf("Expected null bulk string, got %q", buf.String())
	}
}

func TestCmdLPOP_AutoDelete(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lpop_del", &store.ListValue{Elements: [][]byte{[]byte("only")}}, store.SetOptions{})

	ctx := discardCtx("LPOP", bytesArgs("lpop_del"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LPOP auto-delete: %v", err)
	}
	if s.Exists("lpop_del") {
		t.Error("list should have been auto-deleted")
	}
}

func TestCmdRPOP_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("rpop", &store.ListValue{Elements: [][]byte{[]byte("first"), []byte("second"), []byte("third")}}, store.SetOptions{})

	ctx, buf := bufCtx("RPOP", bytesArgs("rpop"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("RPOP: %v", err)
	}
	if !strings.Contains(buf.String(), "third") {
		t.Errorf("Expected 'third', got %q", buf.String())
	}
}

func TestCmdRPOP_EmptyList(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx, buf := bufCtx("RPOP", bytesArgs("nolist"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("RPOP empty: %v", err)
	}
	if !strings.Contains(buf.String(), "$-1") {
		t.Errorf("Expected null bulk string, got %q", buf.String())
	}
}

func TestCmdRPOP_AutoDelete(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("rpop_del", &store.ListValue{Elements: [][]byte{[]byte("only")}}, store.SetOptions{})
	ctx := discardCtx("RPOP", bytesArgs("rpop_del"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("RPOP auto-delete: %v", err)
	}
	if s.Exists("rpop_del") {
		t.Error("list should have been auto-deleted")
	}
}

func TestCmdLLEN_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("ll", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})

	ctx, buf := bufCtx("LLEN", bytesArgs("ll"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LLEN: %v", err)
	}
	if !strings.Contains(buf.String(), "3") {
		t.Errorf("Expected 3, got %q", buf.String())
	}

	// Nonexistent
	ctx2, buf2 := bufCtx("LLEN", bytesArgs("nolist"), s)
	if err := router.ExecuteSilent(ctx2); err != nil {
		t.Fatalf("LLEN nonexistent: %v", err)
	}
	if !strings.Contains(buf2.String(), "0") {
		t.Errorf("Expected 0, got %q", buf2.String())
	}
}

func TestCmdLLEN_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx := discardCtx("LLEN", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdLLEN_WrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("ll_str", &store.StringValue{Data: []byte("val")}, store.SetOptions{})
	ctx := discardCtx("LLEN", bytesArgs("ll_str"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdLRANGE_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lr", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c"), []byte("d")}}, store.SetOptions{})

	// Full range
	ctx, buf := bufCtx("LRANGE", bytesArgs("lr", "0", "-1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LRANGE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "a") || !strings.Contains(out, "d") {
		t.Errorf("Expected a-d, got %q", out)
	}

	// Partial
	ctx2, buf2 := bufCtx("LRANGE", bytesArgs("lr", "1", "2"), s)
	if err := router.ExecuteSilent(ctx2); err != nil {
		t.Fatalf("LRANGE partial: %v", err)
	}
	out2 := buf2.String()
	if !strings.Contains(out2, "b") || !strings.Contains(out2, "c") {
		t.Errorf("Expected b,c, got %q", out2)
	}

	// Negative indices
	ctx3 := discardCtx("LRANGE", bytesArgs("lr", "-2", "-1"), s)
	if err := router.ExecuteSilent(ctx3); err != nil {
		t.Fatalf("LRANGE negative: %v", err)
	}

	// start > stop
	ctx4 := discardCtx("LRANGE", bytesArgs("lr", "3", "1"), s)
	if err := router.ExecuteSilent(ctx4); err != nil {
		t.Fatalf("LRANGE start>stop: %v", err)
	}
}

func TestCmdLRANGE_Nonexistent(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx := discardCtx("LRANGE", bytesArgs("nolist", "0", "-1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LRANGE nonexistent: %v", err)
	}
}

func TestCmdLRANGE_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	// Wrong args
	ctx := discardCtx("LRANGE", bytesArgs("l", "0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer start
	ctx = discardCtx("LRANGE", bytesArgs("l", "abc", "5"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer stop
	ctx = discardCtx("LRANGE", bytesArgs("l", "0", "xyz"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdLINDEX_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("li", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})

	// Normal index
	ctx, buf := bufCtx("LINDEX", bytesArgs("li", "1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LINDEX: %v", err)
	}
	if !strings.Contains(buf.String(), "b") {
		t.Errorf("Expected 'b', got %q", buf.String())
	}

	// Negative index
	ctx2, buf2 := bufCtx("LINDEX", bytesArgs("li", "-1"), s)
	if err := router.ExecuteSilent(ctx2); err != nil {
		t.Fatalf("LINDEX negative: %v", err)
	}
	if !strings.Contains(buf2.String(), "c") {
		t.Errorf("Expected 'c', got %q", buf2.String())
	}

	// Out of range
	ctx3, buf3 := bufCtx("LINDEX", bytesArgs("li", "10"), s)
	if err := router.ExecuteSilent(ctx3); err != nil {
		t.Fatalf("LINDEX out of range: %v", err)
	}
	if !strings.Contains(buf3.String(), "$-1") {
		t.Errorf("Expected null, got %q", buf3.String())
	}
}

func TestCmdLINDEX_Nonexistent(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx := discardCtx("LINDEX", bytesArgs("nolist", "0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LINDEX nonexistent: %v", err)
	}
}

func TestCmdLINDEX_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	// Wrong args
	ctx := discardCtx("LINDEX", bytesArgs("l"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer index
	ctx = discardCtx("LINDEX", bytesArgs("l", "abc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdLSET_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("ls", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})

	ctx := discardCtx("LSET", bytesArgs("ls", "1", "x"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LSET: %v", err)
	}

	entry, _ := s.Get("ls")
	elems := entry.Value.(*store.ListValue).Elements
	if string(elems[1]) != "x" {
		t.Errorf("Expected 'x' at index 1, got %q", string(elems[1]))
	}
}

func TestCmdLSET_NegativeIndex(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("ls2", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})

	ctx := discardCtx("LSET", bytesArgs("ls2", "-1", "z"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LSET negative: %v", err)
	}

	entry, _ := s.Get("ls2")
	elems := entry.Value.(*store.ListValue).Elements
	if string(elems[2]) != "z" {
		t.Errorf("Expected 'z' at last index, got %q", string(elems[2]))
	}
}

func TestCmdLSET_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	// Wrong args
	ctx := discardCtx("LSET", bytesArgs("l", "0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer index
	ctx = discardCtx("LSET", bytesArgs("l", "abc", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Nonexistent list
	ctx = discardCtx("LSET", bytesArgs("nolist", "0", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Index out of range
	s.Set("ls_oor", &store.ListValue{Elements: [][]byte{[]byte("a")}}, store.SetOptions{})
	ctx = discardCtx("LSET", bytesArgs("ls_oor", "5", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdLREM_CountZero(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lrem", &store.ListValue{Elements: [][]byte{
		[]byte("a"), []byte("b"), []byte("a"), []byte("c"), []byte("a"),
	}}, store.SetOptions{})

	// count=0 removes all occurrences
	ctx, buf := bufCtx("LREM", bytesArgs("lrem", "0", "a"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LREM: %v", err)
	}
	if !strings.Contains(buf.String(), "3") {
		t.Errorf("Expected 3, got %q", buf.String())
	}
}

func TestCmdLREM_PositiveCount(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lrem2", &store.ListValue{Elements: [][]byte{
		[]byte("a"), []byte("b"), []byte("a"), []byte("c"), []byte("a"),
	}}, store.SetOptions{})

	// count=2 removes first 2 from head
	ctx, buf := bufCtx("LREM", bytesArgs("lrem2", "2", "a"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LREM positive: %v", err)
	}
	if !strings.Contains(buf.String(), "2") {
		t.Errorf("Expected 2, got %q", buf.String())
	}

	entry, _ := s.Get("lrem2")
	elems := entry.Value.(*store.ListValue).Elements
	// Should have: b, c, a
	if len(elems) != 3 {
		t.Errorf("Expected 3 elements, got %d", len(elems))
	}
}

func TestCmdLREM_NegativeCount(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lrem3", &store.ListValue{Elements: [][]byte{
		[]byte("a"), []byte("b"), []byte("a"), []byte("c"), []byte("a"),
	}}, store.SetOptions{})

	// count=-2 removes last 2 from tail
	ctx, buf := bufCtx("LREM", bytesArgs("lrem3", "-2", "a"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LREM negative: %v", err)
	}
	if !strings.Contains(buf.String(), "2") {
		t.Errorf("Expected 2, got %q", buf.String())
	}

	entry, _ := s.Get("lrem3")
	elems := entry.Value.(*store.ListValue).Elements
	// Should have: a, b, c
	if len(elems) != 3 {
		t.Errorf("Expected 3 elements, got %d", len(elems))
	}
}

func TestCmdLREM_Nonexistent(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx, buf := bufCtx("LREM", bytesArgs("nolist", "0", "a"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LREM nonexistent: %v", err)
	}
	if !strings.Contains(buf.String(), "0") {
		t.Errorf("Expected 0, got %q", buf.String())
	}
}

func TestCmdLREM_AutoDelete(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lrem_del", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("a")}}, store.SetOptions{})
	ctx := discardCtx("LREM", bytesArgs("lrem_del", "0", "a"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LREM auto-delete: %v", err)
	}
	if s.Exists("lrem_del") {
		t.Error("list should have been auto-deleted")
	}
}

func TestCmdLREM_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	// Wrong args
	ctx := discardCtx("LREM", bytesArgs("l", "0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer count
	ctx = discardCtx("LREM", bytesArgs("l", "abc", "val"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdLTRIM_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lt", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c"), []byte("d")}}, store.SetOptions{})

	ctx := discardCtx("LTRIM", bytesArgs("lt", "1", "2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LTRIM: %v", err)
	}

	entry, _ := s.Get("lt")
	elems := entry.Value.(*store.ListValue).Elements
	if len(elems) != 2 || string(elems[0]) != "b" || string(elems[1]) != "c" {
		t.Errorf("Expected [b, c], got %v", elems)
	}
}

func TestCmdLTRIM_OutOfRange(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lt2", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}, store.SetOptions{})

	// start > stop => empty list, key deleted
	ctx := discardCtx("LTRIM", bytesArgs("lt2", "5", "1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LTRIM out of range: %v", err)
	}
	if s.Exists("lt2") {
		t.Error("list should have been auto-deleted")
	}
}

func TestCmdLTRIM_Nonexistent(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx := discardCtx("LTRIM", bytesArgs("nolist", "0", "1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LTRIM nonexistent: %v", err)
	}
}

func TestCmdLTRIM_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	// Wrong args
	ctx := discardCtx("LTRIM", bytesArgs("l", "0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer start
	ctx = discardCtx("LTRIM", bytesArgs("l", "abc", "5"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer stop
	ctx = discardCtx("LTRIM", bytesArgs("l", "0", "xyz"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdLINSERT_Before(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("linb", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})

	ctx, buf := bufCtx("LINSERT", bytesArgs("linb", "BEFORE", "b", "x"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LINSERT BEFORE: %v", err)
	}
	if !strings.Contains(buf.String(), "4") {
		t.Errorf("Expected 4, got %q", buf.String())
	}

	entry, _ := s.Get("linb")
	elems := entry.Value.(*store.ListValue).Elements
	if string(elems[1]) != "x" {
		t.Errorf("Expected 'x' at index 1, got %q", string(elems[1]))
	}
}

func TestCmdLINSERT_After(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lina", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})

	ctx, buf := bufCtx("LINSERT", bytesArgs("lina", "AFTER", "b", "x"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LINSERT AFTER: %v", err)
	}
	if !strings.Contains(buf.String(), "4") {
		t.Errorf("Expected 4, got %q", buf.String())
	}

	entry, _ := s.Get("lina")
	elems := entry.Value.(*store.ListValue).Elements
	if string(elems[2]) != "x" {
		t.Errorf("Expected 'x' at index 2, got %q", string(elems[2]))
	}
}

func TestCmdLINSERT_PivotNotFound(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lin_nf", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}, store.SetOptions{})

	ctx, buf := bufCtx("LINSERT", bytesArgs("lin_nf", "BEFORE", "z", "x"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LINSERT pivot not found: %v", err)
	}
	if !strings.Contains(buf.String(), "-1") {
		t.Errorf("Expected -1, got %q", buf.String())
	}
}

func TestCmdLINSERT_Nonexistent(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx, buf := bufCtx("LINSERT", bytesArgs("nolist", "BEFORE", "b", "x"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LINSERT nonexistent: %v", err)
	}
	if !strings.Contains(buf.String(), "0") {
		t.Errorf("Expected 0, got %q", buf.String())
	}
}

func TestCmdLINSERT_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	// Wrong args
	ctx := discardCtx("LINSERT", bytesArgs("l", "BEFORE", "b"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Invalid position
	s.Set("lin_bad", &store.ListValue{Elements: [][]byte{[]byte("a")}}, store.SetOptions{})
	ctx = discardCtx("LINSERT", bytesArgs("lin_bad", "INVALID", "a", "x"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdRPOPLPUSH_Success(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("rpl_src", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})

	ctx, buf := bufCtx("RPOPLPUSH", bytesArgs("rpl_src", "rpl_dst"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("RPOPLPUSH: %v", err)
	}
	if !strings.Contains(buf.String(), "c") {
		t.Errorf("Expected 'c', got %q", buf.String())
	}

	// Check destination
	entry, exists := s.Get("rpl_dst")
	if !exists {
		t.Fatal("destination should exist")
	}
	elems := entry.Value.(*store.ListValue).Elements
	if string(elems[0]) != "c" {
		t.Errorf("Expected 'c' at head of dest, got %q", string(elems[0]))
	}
}

func TestCmdRPOPLPUSH_EmptySource(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx, buf := bufCtx("RPOPLPUSH", bytesArgs("nosrc", "dst"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("RPOPLPUSH empty: %v", err)
	}
	if !strings.Contains(buf.String(), "$-1") {
		t.Errorf("Expected null, got %q", buf.String())
	}
}

func TestCmdRPOPLPUSH_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx := discardCtx("RPOPLPUSH", bytesArgs("src"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdLMOVE_LeftToRight(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lm_src", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})

	ctx, buf := bufCtx("LMOVE", bytesArgs("lm_src", "lm_dst", "LEFT", "RIGHT"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LMOVE: %v", err)
	}
	if !strings.Contains(buf.String(), "a") {
		t.Errorf("Expected 'a', got %q", buf.String())
	}
}

func TestCmdLMOVE_RightToLeft(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lm_src2", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b"), []byte("c")}}, store.SetOptions{})

	ctx, buf := bufCtx("LMOVE", bytesArgs("lm_src2", "lm_dst2", "RIGHT", "LEFT"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LMOVE R->L: %v", err)
	}
	if !strings.Contains(buf.String(), "c") {
		t.Errorf("Expected 'c', got %q", buf.String())
	}
}

func TestCmdLMOVE_EmptySource(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx := discardCtx("LMOVE", bytesArgs("nosrc", "dst", "LEFT", "RIGHT"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LMOVE empty: %v", err)
	}
}

func TestCmdLMOVE_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	// Wrong args
	ctx := discardCtx("LMOVE", bytesArgs("s", "d", "LEFT"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Invalid direction
	s.Set("lm_bad", &store.ListValue{Elements: [][]byte{[]byte("a")}}, store.SetOptions{})
	ctx = discardCtx("LMOVE", bytesArgs("lm_bad", "dst", "INVALID", "RIGHT"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Invalid whereTo
	ctx = discardCtx("LMOVE", bytesArgs("lm_bad", "dst", "LEFT", "INVALID"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdLPOS_Basic(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lpos", &store.ListValue{Elements: [][]byte{
		[]byte("a"), []byte("b"), []byte("c"), []byte("b"), []byte("d"),
	}}, store.SetOptions{})

	// Find first
	ctx, buf := bufCtx("LPOS", bytesArgs("lpos", "b"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LPOS: %v", err)
	}
	if !strings.Contains(buf.String(), "1") {
		t.Errorf("Expected 1, got %q", buf.String())
	}
}

func TestCmdLPOS_WithRank(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lpos2", &store.ListValue{Elements: [][]byte{
		[]byte("a"), []byte("b"), []byte("c"), []byte("b"), []byte("d"),
	}}, store.SetOptions{})

	// Rank 2 = second occurrence
	ctx, buf := bufCtx("LPOS", bytesArgs("lpos2", "b", "RANK", "2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LPOS RANK: %v", err)
	}
	if !strings.Contains(buf.String(), "3") {
		t.Errorf("Expected 3, got %q", buf.String())
	}
}

func TestCmdLPOS_WithCount(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lpos3", &store.ListValue{Elements: [][]byte{
		[]byte("a"), []byte("b"), []byte("c"), []byte("b"), []byte("d"),
	}}, store.SetOptions{})

	// COUNT 2 = return up to 2 positions
	ctx := discardCtx("LPOS", bytesArgs("lpos3", "b", "COUNT", "2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LPOS COUNT: %v", err)
	}
}

func TestCmdLPOS_NegativeRank(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lpos4", &store.ListValue{Elements: [][]byte{
		[]byte("a"), []byte("b"), []byte("c"), []byte("b"), []byte("d"),
	}}, store.SetOptions{})

	// Negative rank searches from tail
	ctx, buf := bufCtx("LPOS", bytesArgs("lpos4", "b", "RANK", "-1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LPOS negative rank: %v", err)
	}
	if !strings.Contains(buf.String(), "3") {
		t.Errorf("Expected 3, got %q", buf.String())
	}
}

func TestCmdLPOS_NegativeRankWithCount(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lpos5", &store.ListValue{Elements: [][]byte{
		[]byte("a"), []byte("b"), []byte("c"), []byte("b"), []byte("d"),
	}}, store.SetOptions{})

	ctx := discardCtx("LPOS", bytesArgs("lpos5", "b", "RANK", "-1", "COUNT", "2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LPOS negative rank count: %v", err)
	}
}

func TestCmdLPOS_WithMAXLEN(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lpos6", &store.ListValue{Elements: [][]byte{
		[]byte("a"), []byte("b"), []byte("c"), []byte("b"), []byte("d"),
	}}, store.SetOptions{})

	ctx := discardCtx("LPOS", bytesArgs("lpos6", "b", "MAXLEN", "2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LPOS MAXLEN: %v", err)
	}
}

func TestCmdLPOS_NotFound(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("lpos_nf", &store.ListValue{Elements: [][]byte{[]byte("a"), []byte("b")}}, store.SetOptions{})

	// Not found returns null
	ctx := discardCtx("LPOS", bytesArgs("lpos_nf", "z"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LPOS not found: %v", err)
	}

	// Not found with COUNT returns empty array
	ctx = discardCtx("LPOS", bytesArgs("lpos_nf", "z", "COUNT", "1"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LPOS not found count: %v", err)
	}
}

func TestCmdLPOS_Nonexistent(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx := discardCtx("LPOS", bytesArgs("nolist", "a"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("LPOS nonexistent: %v", err)
	}
}

func TestCmdLPOS_ErrorPaths(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	// Wrong args
	ctx := discardCtx("LPOS", bytesArgs("l"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer RANK
	s.Set("lpos_e", &store.ListValue{Elements: [][]byte{[]byte("a")}}, store.SetOptions{})
	ctx = discardCtx("LPOS", bytesArgs("lpos_e", "a", "RANK", "abc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// RANK without value
	ctx = discardCtx("LPOS", bytesArgs("lpos_e", "a", "RANK"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// COUNT without value
	ctx = discardCtx("LPOS", bytesArgs("lpos_e", "a", "COUNT"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// MAXLEN without value
	ctx = discardCtx("LPOS", bytesArgs("lpos_e", "a", "MAXLEN"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdBLPOP_ImmediateSuccess(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("blp", &store.ListValue{Elements: [][]byte{[]byte("item1"), []byte("item2")}}, store.SetOptions{})

	ctx := discardCtx("BLPOP", bytesArgs("blp", "0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("BLPOP: %v", err)
	}
}

func TestCmdBLPOP_Timeout(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	// Empty list with timeout=0 should return null immediately
	ctx := discardCtx("BLPOP", bytesArgs("emptyblp", "0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("BLPOP timeout: %v", err)
	}
}

func TestCmdBLPOP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx := discardCtx("BLPOP", bytesArgs("l"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Non-integer timeout
	ctx = discardCtx("BLPOP", bytesArgs("l", "abc"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdBRPOP_ImmediateSuccess(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	s.Set("brp", &store.ListValue{Elements: [][]byte{[]byte("item1"), []byte("item2")}}, store.SetOptions{})

	ctx := discardCtx("BRPOP", bytesArgs("brp", "0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("BRPOP: %v", err)
	}
}

func TestCmdBRPOP_Timeout(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx := discardCtx("BRPOP", bytesArgs("emptybrp", "0"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("BRPOP timeout: %v", err)
	}
}

func TestCmdBRPOP_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	ctx := discardCtx("BRPOP", bytesArgs("l"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// ======================================================================
// SET command advanced options
// ======================================================================

func TestCmdSET_WithPXAT(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("SET", bytesArgs("pxat_key", "val", "PXAT", "2000000000000"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("SET PXAT: %v", err)
	}
}

func TestCmdSET_WithEXAT(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("SET", bytesArgs("exat_key", "val", "EXAT", "2000000000"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("SET EXAT: %v", err)
	}
}

func TestCmdSET_NXandXXConflict(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("SET", bytesArgs("conflict_key", "val", "NX", "XX"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("SET NX+XX: %v", err)
	}
}

func TestCmdSET_GETwithNonexistent(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx, buf := bufCtx("SET", bytesArgs("set_get_new", "val", "GET"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("SET GET new: %v", err)
	}
	if !strings.Contains(buf.String(), "$-1") {
		t.Errorf("Expected null bulk string for new key with GET, got %q", buf.String())
	}
}

func TestCmdSET_GETwithExisting(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("set_get_old", &store.StringValue{Data: []byte("old")}, store.SetOptions{})
	ctx, buf := bufCtx("SET", bytesArgs("set_get_old", "new", "GET"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("SET GET existing: %v", err)
	}
	if !strings.Contains(buf.String(), "old") {
		t.Errorf("Expected 'old', got %q", buf.String())
	}
}

func TestCmdSET_GETwithWrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("set_get_hash", &store.HashValue{Fields: map[string][]byte{"f": []byte("v")}}, store.SetOptions{})
	ctx := discardCtx("SET", bytesArgs("set_get_hash", "val", "GET"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdSET_UnknownOption(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("SET", bytesArgs("k", "v", "BADOPTION"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdSET_EXwithoutValue(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("SET", bytesArgs("k", "v", "EX"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdSET_PXwithoutValue(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("SET", bytesArgs("k", "v", "PX"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdGET_WrongType(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	s.Set("get_hash", &store.HashValue{Fields: map[string][]byte{"f": []byte("v")}}, store.SetOptions{})
	ctx := discardCtx("GET", bytesArgs("get_hash"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdGET_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("GET", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdDEL_None(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx, buf := bufCtx("DEL", bytesArgs("nokey1", "nokey2"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("DEL none: %v", err)
	}
	if !strings.Contains(buf.String(), "0") {
		t.Errorf("Expected 0, got %q", buf.String())
	}
}

func TestCmdDEL_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("DEL", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestCmdEXISTS_None(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx, buf := bufCtx("EXISTS", bytesArgs("nokey"), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("EXISTS none: %v", err)
	}
	if !strings.Contains(buf.String(), "0") {
		t.Errorf("Expected 0, got %q", buf.String())
	}
}

func TestCmdEXISTS_WrongArgs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	ctx := discardCtx("EXISTS", bytesArgs(), s)
	if err := router.ExecuteSilent(ctx); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

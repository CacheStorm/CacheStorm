package command

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/search"
	"github.com/cachestorm/cachestorm/internal/store"
)

// FT.SUGADD, FT.SUGGET, FT.SUGDEL and FT.SUGLEN keep auto-complete
// dictionaries in keys of RediSearch's trie type, with its scoring.

func RegisterSuggestCommands(router *Router) {
	for name, handler := range map[string]func(*Context) error{
		"FT.SUGADD": cmdFTSUGADD, "FT.SUGGET": cmdFTSUGGET, "FT.SUGDEL": cmdFTSUGDEL, "FT.SUGLEN": cmdFTSUGLEN,
	} {
		router.Register(&CommandDef{Name: name, Handler: handler})
	}
}

type suggestValue struct {
	mu   sync.Mutex
	trie *search.Trie
}

func (v *suggestValue) Type() store.DataType { return store.DataTypeSuggest }
func (v *suggestValue) String() string       { return "trie" }
func (v *suggestValue) ExtensionType() byte  { return store.ExtensionSuggest }

func (v *suggestValue) SizeOf() int64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.trie.Bytes()
}

func (v *suggestValue) Clone() store.Value {
	v.mu.Lock()
	defer v.mu.Unlock()
	return &suggestValue{trie: v.trie.Clone()}
}

// MarshalBinary writes the entries as RediSearch saves them, in iteration
// order; loading inserts them again.
func (v *suggestValue) MarshalBinary() ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	var body []byte
	n := uint64(0)
	v.trie.Each(func(s []byte, score float32, payload []byte) {
		body = binary.LittleEndian.AppendUint64(body, uint64(len(s)))
		body = append(body, s...)
		body = binary.LittleEndian.AppendUint64(body, math.Float64bits(float64(score)))
		body = binary.LittleEndian.AppendUint64(body, uint64(len(payload)))
		body = append(body, payload...)
		n++
	})
	return append(binary.LittleEndian.AppendUint64(nil, n), body...), nil
}

func decodeSuggest(data []byte) (store.Value, error) {
	take := func(n uint64) ([]byte, bool) {
		if n > uint64(len(data)) {
			return nil, false
		}
		b := data[:n]
		data = data[n:]
		return b, true
	}
	u64 := func() (uint64, bool) {
		b, ok := take(8)
		if !ok {
			return 0, false
		}
		return binary.LittleEndian.Uint64(b), true
	}
	count, ok := u64()
	if !ok {
		return nil, store.ErrExtensionFormat
	}
	t := search.NewTrie()
	for range count {
		n, ok := u64()
		s, ok2 := take(n)
		bits, ok3 := u64()
		pn, ok4 := u64()
		payload, ok5 := take(pn)
		if !ok || !ok2 || !ok3 || !ok4 || !ok5 {
			return nil, store.ErrExtensionFormat
		}
		t.Insert(s, math.Float64frombits(bits), false, payload)
	}
	if len(data) != 0 {
		return nil, store.ErrExtensionFormat
	}
	return &suggestValue{trie: t}, nil
}

func init() { store.RegisterExtension(store.ExtensionSuggest, decodeSuggest) }

func moduleArityError(ctx *Context) error {
	return errors.New("ERR wrong number of arguments for '" + ctx.Command + "' command")
}

func getSuggest(ctx *Context, key string) (*suggestValue, error) {
	entry, exists := ctx.Store.Get(key)
	if !exists {
		return nil, nil
	}
	v, ok := entry.Value.(*suggestValue)
	if !ok {
		return nil, store.ErrWrongType
	}
	return v, nil
}

// cmdFTSUGADD is FT.SUGADD key string score [INCR] [PAYLOAD payload]. Like
// RediSearch it creates the key even when the string is not added (a zero
// score), and replies with the dictionary's size.
func cmdFTSUGADD(ctx *Context) error {
	if n := ctx.ArgCount(); n < 3 || n > 6 {
		return ctx.WriteError(moduleArityError(ctx))
	}
	incr := false
	var payload []byte
	for i := 3; i < ctx.ArgCount(); i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "INCR":
			incr = true
		case "PAYLOAD":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(errors.New("Invalid payload: Expected an argument, but none provided")) //nolint:staticcheck // RediSearch's reply text
			}
			payload = ctx.Arg(i + 1)
			i++
		default:
			return ctx.WriteError(fmt.Errorf("Unknown argument `%s`", ctx.ArgString(i))) //nolint:staticcheck // RediSearch's reply text
		}
	}
	key := ctx.ArgString(0)
	v, err := getSuggest(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	score, err := strconv.ParseFloat(ctx.ArgString(2), 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) || math.IsNaN(score) || strings.TrimSpace(ctx.ArgString(2)) != ctx.ArgString(2) {
		return ctx.WriteError(errors.New("ERR invalid score"))
	}
	if v == nil {
		v = &suggestValue{trie: search.NewTrie()}
		if err := ctx.Store.Set(key, v, store.SetOptions{}); err != nil {
			return ctx.WriteError(err)
		}
	}
	v.mu.Lock()
	v.trie.Insert(ctx.Arg(1), score, incr, payload)
	size := v.trie.Size()
	v.mu.Unlock()
	valueWritten(ctx, key)
	return ctx.WriteInteger(int64(size))
}

func cmdFTSUGLEN(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(moduleArityError(ctx))
	}
	v, err := getSuggest(ctx, ctx.ArgString(0))
	if err != nil || v == nil {
		if err != nil {
			return ctx.WriteError(err)
		}
		return ctx.WriteInteger(0)
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return ctx.WriteInteger(int64(v.trie.Size()))
}

// cmdFTSUGDEL deletes a string; the key goes with its last one.
func cmdFTSUGDEL(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(moduleArityError(ctx))
	}
	key := ctx.ArgString(0)
	v, err := getSuggest(ctx, key)
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil {
		return ctx.WriteInteger(0)
	}
	v.mu.Lock()
	rc := v.trie.Delete(ctx.Arg(1))
	empty := v.trie.Size() == 0
	v.mu.Unlock()
	if empty {
		ctx.Store.Delete(key)
	} else {
		valueWritten(ctx, key)
	}
	return ctx.WriteInteger(int64(rc))
}

// cmdFTSUGGET is FT.SUGGET key prefix [FUZZY] [MAX n] [WITHSCORES] [TRIM]
// [OPTIMIZE] [WITHPAYLOADS].
func cmdFTSUGGET(ctx *Context) error {
	if n := ctx.ArgCount(); n < 2 || n > 9 {
		return ctx.WriteError(moduleArityError(ctx))
	}
	prefix := ctx.Arg(1)
	if len(prefix) >= 200 {
		return ctx.WriteError(errors.New("Invalid query length")) //nolint:staticcheck // RediSearch's reply text
	}
	num, fuzzy, withScores, withPayloads, trim := 5, false, false, false, false
	for i := 2; i < ctx.ArgCount(); i++ {
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "FUZZY":
			fuzzy = true
		case "WITHSCORES":
			withScores = true
		case "WITHPAYLOADS":
			withPayloads = true
		case "TRIM":
			trim = true
		case "OPTIMIZE":
		case "MAX":
			if i+1 >= ctx.ArgCount() {
				return ctx.WriteError(errors.New("MAX: Expected an argument, but none provided"))
			}
			m, err := strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil {
				return ctx.WriteError(errors.New("MAX: Could not convert argument to expected type"))
			}
			if m < 1 || m > math.MaxUint32 {
				return ctx.WriteError(errors.New("MAX: Value is outside acceptable bounds"))
			}
			num = int(m)
			i++
		default:
			return ctx.WriteError(errors.New("Unrecognized argument: " + ctx.ArgString(i))) //nolint:staticcheck // RediSearch's reply text
		}
	}
	v, err := getSuggest(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	if v == nil {
		return ctx.WriteArray([]*resp.Value{})
	}
	dist := 0
	if fuzzy {
		dist = 1
	}
	v.mu.Lock()
	found, ok := v.trie.Search(prefix, num, dist, trim)
	v.mu.Unlock()
	if !ok {
		return ctx.WriteError(errors.New("Invalid query")) //nolint:staticcheck // RediSearch's reply text
	}
	out := make([]*resp.Value, 0, len(found)*3)
	for _, s := range found {
		out = append(out, resp.BulkBytes(s.Str))
		if withScores {
			out = append(out, tdigestDouble(ctx, float64(s.Score)))
		}
		if withPayloads {
			if s.Payload == nil {
				out = append(out, resp.NullBulkString())
			} else {
				out = append(out, resp.BulkBytes(s.Payload))
			}
		}
	}
	return ctx.WriteArray(out)
}

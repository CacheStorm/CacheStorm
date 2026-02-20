package command

import (
	"bytes"
	"testing"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

func newTestContext(cmd string, args [][]byte, s *store.Store) *Context {
	buf := &bytes.Buffer{}
	w := resp.NewWriter(buf)
	return &Context{
		Command: cmd,
		Args:    args,
		Store:   s,
		Writer:  w,
	}
}

func TestStringCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)

	t.Run("SET", func(t *testing.T) {
		ctx := newTestContext("SET", [][]byte{[]byte("key"), []byte("value")}, s)
		handler, ok := router.Get("SET")
		if !ok {
			t.Fatal("SET command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("SET error: %v", err)
		}
	})

	t.Run("GET", func(t *testing.T) {
		ctx := newTestContext("GET", [][]byte{[]byte("key")}, s)
		handler, ok := router.Get("GET")
		if !ok {
			t.Fatal("GET command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("GET error: %v", err)
		}
	})

	t.Run("INCR", func(t *testing.T) {
		ctx := newTestContext("INCR", [][]byte{[]byte("counter")}, s)
		handler, ok := router.Get("INCR")
		if !ok {
			t.Fatal("INCR command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("INCR error: %v", err)
		}
	})
}

func TestHashCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterHashCommands(router)

	t.Run("HSET", func(t *testing.T) {
		ctx := newTestContext("HSET", [][]byte{[]byte("hash"), []byte("field"), []byte("value")}, s)
		handler, ok := router.Get("HSET")
		if !ok {
			t.Fatal("HSET command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("HSET error: %v", err)
		}
	})

	t.Run("HGET", func(t *testing.T) {
		ctx := newTestContext("HGET", [][]byte{[]byte("hash"), []byte("field")}, s)
		handler, ok := router.Get("HGET")
		if !ok {
			t.Fatal("HGET command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("HGET error: %v", err)
		}
	})

	t.Run("HLEN", func(t *testing.T) {
		ctx := newTestContext("HLEN", [][]byte{[]byte("hash")}, s)
		handler, ok := router.Get("HLEN")
		if !ok {
			t.Fatal("HLEN command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("HLEN error: %v", err)
		}
	})
}

func TestListCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterListCommands(router)

	t.Run("LPUSH", func(t *testing.T) {
		ctx := newTestContext("LPUSH", [][]byte{[]byte("list"), []byte("item1")}, s)
		handler, ok := router.Get("LPUSH")
		if !ok {
			t.Fatal("LPUSH command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("LPUSH error: %v", err)
		}
	})

	t.Run("LLEN", func(t *testing.T) {
		ctx := newTestContext("LLEN", [][]byte{[]byte("list")}, s)
		handler, ok := router.Get("LLEN")
		if !ok {
			t.Fatal("LLEN command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("LLEN error: %v", err)
		}
	})
}

func TestSetCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSetCommands(router)

	t.Run("SADD", func(t *testing.T) {
		ctx := newTestContext("SADD", [][]byte{[]byte("set"), []byte("member1")}, s)
		handler, ok := router.Get("SADD")
		if !ok {
			t.Fatal("SADD command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("SADD error: %v", err)
		}
	})

	t.Run("SCARD", func(t *testing.T) {
		ctx := newTestContext("SCARD", [][]byte{[]byte("set")}, s)
		handler, ok := router.Get("SCARD")
		if !ok {
			t.Fatal("SCARD command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("SCARD error: %v", err)
		}
	})
}

func TestSortedSetCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterSortedSetCommands(router)

	t.Run("ZADD", func(t *testing.T) {
		ctx := newTestContext("ZADD", [][]byte{[]byte("zset"), []byte("1"), []byte("member1")}, s)
		handler, ok := router.Get("ZADD")
		if !ok {
			t.Fatal("ZADD command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("ZADD error: %v", err)
		}
	})

	t.Run("ZCARD", func(t *testing.T) {
		ctx := newTestContext("ZCARD", [][]byte{[]byte("zset")}, s)
		handler, ok := router.Get("ZCARD")
		if !ok {
			t.Fatal("ZCARD command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("ZCARD error: %v", err)
		}
	})
}

func TestKeyCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterStringCommands(router)
	RegisterKeyCommands(router)

	s.Set("exists", &store.StringValue{Data: []byte("value")}, store.SetOptions{})

	t.Run("EXISTS", func(t *testing.T) {
		ctx := newTestContext("EXISTS", [][]byte{[]byte("exists")}, s)
		handler, ok := router.Get("EXISTS")
		if !ok {
			t.Fatal("EXISTS command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("EXISTS error: %v", err)
		}
	})

	t.Run("TYPE", func(t *testing.T) {
		ctx := newTestContext("TYPE", [][]byte{[]byte("exists")}, s)
		handler, ok := router.Get("TYPE")
		if !ok {
			t.Fatal("TYPE command not found")
		}
		if err := handler.Handler(ctx); err != nil {
			t.Errorf("TYPE error: %v", err)
		}
	})
}

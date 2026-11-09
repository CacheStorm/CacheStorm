package persistence

import (
	"bufio"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/cachestorm/cachestorm/internal/store"
)

// pieceBuffer is an io.Writer that keeps what is written as pieces;
// WriteSnapshot hands it its encoded pieces without copying them.
type pieceBuffer struct {
	pieces [][]byte
}

func (b *pieceBuffer) Write(p []byte) (int, error) {
	b.pieces = append(b.pieces, append([]byte(nil), p...))
	return len(p), nil
}

// chunkWriter collects writes into chunks that grow up to rdbChunkMax; cut
// ends the current piece. The pieces never share bytes with later writes.
type chunkWriter struct {
	chunk  []byte
	start  int
	pieces [][]byte
}

func (c *chunkWriter) Write(p []byte) (int, error) {
	if cap(c.chunk)-len(c.chunk) < len(p) {
		c.cut()
		c.chunk = make([]byte, 0, max(min(2*cap(c.chunk), rdbChunkMax), rdbChunkMin, len(p)))
		c.start = 0
	}
	c.chunk = append(c.chunk, p...)
	return len(p), nil
}

func (c *chunkWriter) cut() {
	if len(c.chunk) > c.start {
		c.pieces = append(c.pieces, c.chunk[c.start:len(c.chunk):len(c.chunk)])
		c.start = len(c.chunk)
	}
}

// encodeSnapshotShards encodes what WriteSnapshot writes for src, each shard
// on a free core, and returns it in pieces.
func encodeSnapshotShards(src MultiDBSource, shards []EntryShard) ([][]byte, error) {
	var prelude chunkWriter
	bw := bufio.NewWriterSize(&prelude, 64*1024)
	if err := (&AOFRewriter{store: multiDBAdapter{src}}).writePrelude(bw); err != nil {
		return nil, err
	}
	if err := bw.Flush(); err != nil {
		return nil, err
	}
	prelude.cut()
	type shardOut struct {
		pieces [][]byte
		err    error
	}
	encoded := make([]shardOut, len(shards))
	var next atomic.Int64
	var failed atomic.Bool
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), len(shards)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rw := &AOFRewriter{}
			var sink chunkWriter
			bw := bufio.NewWriterSize(&sink, 32*1024)
			for !failed.Load() {
				i := int(next.Add(1) - 1)
				if i >= len(shards) {
					return
				}
				sink.pieces = nil
				out := &encoded[i]
				out.err = shards[i].Each(func(key string, entry *store.Entry) error {
					return rw.writeEntry(bw, key, entry)
				})
				if out.err == nil {
					out.err = bw.Flush()
				}
				if out.err != nil {
					failed.Store(true)
					continue
				}
				sink.cut()
				out.pieces = sink.pieces
			}
		}()
	}
	wg.Wait()
	pieces := prelude.pieces
	selected := 0
	selectDB := func(db int) {
		pieces = append(pieces, appendCommand(nil, "SELECT", [][]byte{strconv.AppendInt(nil, int64(db), 10)}))
		selected = db
	}
	for i := range encoded {
		if encoded[i].err != nil {
			return nil, encoded[i].err
		}
		if len(encoded[i].pieces) == 0 {
			continue
		}
		if db := shards[i].DB; db != selected {
			selectDB(db)
		}
		pieces = append(pieces, encoded[i].pieces...)
	}
	if selected != 0 {
		selectDB(0)
	}
	return pieces, nil
}

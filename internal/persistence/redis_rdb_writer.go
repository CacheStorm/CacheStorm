package persistence

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

// RedisRDBInfo is what an RDB in Redis' format carries besides the keys: the
// function libraries' code and the replication aux fields.
type RedisRDBInfo struct {
	Functions []string
	// StreamDB is the database the replication stream continues in, saved
	// as repl-stream-db so a replica selects it before applying the stream.
	StreamDB int
	ReplID   string
	Offset   int64
	// Extensions keeps what Redis has no type for, for a snapshot CacheStorm
	// loads again: JSON documents, time series and the probabilistic
	// structures as module values, and key tags as an aux field before the
	// key, which Redis ignores. Without it
	// those keys are left out and the tags dropped.
	Extensions bool
	// Aux holds extra aux fields; Redis ignores the ones it does not know.
	Aux map[string]string
	// IdleTimes saves each key's idle time, LFU its LFU counter, as Redis
	// does under an LRU or LFU maxmemory policy.
	IdleTimes, LFU bool
}

// The module types CacheStorm's extension values are saved as. A module type
// id is its 9-character name in a 64-symbol alphabet followed by a 10-bit
// encoding version; Redis refuses a file with module values it has no module
// for, naming the type.
var rdbModules = map[byte]uint64{
	dumpTypeJSON:           rdbModuleID("CStorm-JS", 1),
	dumpTypeTimeSeries:     rdbModuleID("CStorm-TS", 1),
	store.ExtensionBloom:   rdbModuleID("CStorm-BF", 1),
	store.ExtensionCuckoo:  rdbModuleID("CStorm-CF", 1),
	store.ExtensionCMS:     rdbModuleID("CStorm-CM", 1),
	store.ExtensionTopK:    rdbModuleID("CStorm-TK", 1),
	store.ExtensionTDigest: rdbModuleID("CStorm-TD", 1),
	store.ExtensionSuggest: rdbModuleID("CStorm-SG", 1),
}

const (
	rdbTypeModule2       = 7
	rdbModuleOpcodeEOF   = 0
	rdbModuleOpcodeStr   = 5
	rdbTagsAux           = "cachestorm-tags"
	rdbModuleIDAlphabet  = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	redisRDBHeader       = "REDIS0011"
	redisRDBTrailerBytes = 9
)

func rdbModuleID(name string, encver uint64) uint64 {
	var id uint64
	for i := 0; i < len(name); i++ {
		for j := 0; j < len(rdbModuleIDAlphabet); j++ {
			if rdbModuleIDAlphabet[j] == name[i] {
				id = id<<6 | uint64(j)
			}
		}
	}
	return id<<10 | encver
}

// WriteRedisRDB writes src as an RDB file in Redis' own format, which Redis
// and its tools (redis-check-rdb, replicas, migration tools) load. Values
// Redis has no type for (JSON documents, time series) are left out unless
// info.Extensions is set; it returns how many keys it left out. The version
// is 11, or 12 when a hash has field expirations, which only Redis 7.4 can
// load.
func WriteRedisRDB(w io.Writer, src EntryStreamer, info RedisRDBInfo) (int, error) {
	parts, skipped, err := EncodeRedisRDBParts(src, info)
	if err != nil {
		return skipped, err
	}
	return skipped, parts.writeSealed(w)
}

// EncodeRedisRDB encodes src like WriteRedisRDB but leaves the checksum
// trailer zero, for SealRDBChecksum to fill once the dataset is released.
func EncodeRedisRDB(src EntryStreamer, info RedisRDBInfo) ([]byte, int, error) {
	parts, skipped, err := EncodeRedisRDBParts(src, info)
	if err != nil {
		return nil, skipped, err
	}
	return parts.Join(), skipped, nil
}

// RDBParts is an encoded RDB file in pieces; Join puts them together.
type RDBParts struct {
	pieces [][]byte
	size   int
}

// Join returns the file, its checksum trailer zero. The pieces are released.
func (p *RDBParts) Join() []byte {
	out := make([]byte, 0, p.size)
	for i, piece := range p.pieces {
		out = append(out, piece...)
		p.pieces[i] = nil
	}
	p.pieces = nil
	return out
}

// writeSealed writes the file to w with its checksum trailer filled.
func (p *RDBParts) writeSealed(w io.Writer) error {
	body := p.size - 8
	var crc uint64
	for _, piece := range p.pieces {
		if body <= 0 {
			break
		}
		piece = piece[:min(len(piece), body)]
		crc = rdbChecksum(crc, piece)
		if _, err := w.Write(piece); err != nil {
			return err
		}
		body -= len(piece)
	}
	_, err := w.Write(binary.LittleEndian.AppendUint64(nil, crc))
	return err
}

func (p *RDBParts) add(piece []byte) {
	if len(piece) > 0 {
		p.pieces = append(p.pieces, piece)
		p.size += len(piece)
	}
}

// EncodeRedisRDBParts encodes src like EncodeRedisRDB without joining the
// pieces, so a snapshot holds the write gate only while the dataset is
// encoded. A src that is a ShardedEntrySource is encoded on every core.
func EncodeRedisRDBParts(src EntryStreamer, info RedisRDBInfo) (*RDBParts, int, error) {
	out := []byte(redisRDBHeader)
	aux := func(k, v string) {
		out = append(out, 0xFA)
		out = appendRDBString(out, k)
		out = appendRDBString(out, v)
	}
	aux("redis-ver", "7.4.0")
	aux("redis-bits", "64")
	aux("ctime", strconv.FormatInt(time.Now().Unix(), 10))
	aux("aof-base", "0")
	if info.ReplID != "" {
		aux("repl-stream-db", strconv.Itoa(max(info.StreamDB, 0)))
		aux("repl-id", info.ReplID)
		aux("repl-offset", strconv.FormatInt(info.Offset, 10))
	}
	extra := make([]string, 0, len(info.Aux))
	for k := range info.Aux {
		extra = append(extra, k)
	}
	sort.Strings(extra)
	for _, k := range extra {
		aux(k, info.Aux[k])
	}
	for _, code := range info.Functions {
		out = append(out, rdbOpcodeFunction2)
		out = appendRDBString(out, code)
	}
	parts := &RDBParts{}
	var (
		version uint16
		skipped int
		err     error
	)
	if sharded, ok := src.(ShardedEntrySource); ok {
		parts.add(out)
		version, skipped, err = encodeRDBShards(parts, sharded.EntryShards(), &info)
		out = nil
	} else {
		out, version, skipped, err = encodeRDBSerial(out, src, &info)
	}
	if err != nil {
		return nil, skipped, err
	}
	out = append(out, 0xFF)
	out = append(out, make([]byte, redisRDBTrailerBytes-1)...)
	parts.add(out)
	copy(parts.pieces[0][5:9], fmt.Sprintf("%04d", version))
	return parts, skipped, nil
}

// ShardedEntrySource is an EntryStreamer whose entries split into parts that
// can be walked concurrently; the parts in order hold every entry once.
type ShardedEntrySource interface {
	EntryStreamer
	EntryShards() []EntryShard
}

// EntryShard is one part of a ShardedEntrySource, all in database DB.
type EntryShard struct {
	DB   int
	Each func(fn func(key string, entry *store.Entry) error) error
}

func encodeRDBSerial(out []byte, src EntryStreamer, info *RedisRDBInfo) ([]byte, uint16, int, error) {
	version, skipped, selected := uint16(11), 0, -1
	var scratch []byte
	err := src.ForEachEntry(func(db int, key string, e interface{}) error {
		entry, ok := e.(*store.Entry)
		if !ok {
			return nil
		}
		mark := len(out)
		if db != selected {
			out = append(out, 0xFE)
			out = appendDumpLength(out, uint64(db))
		}
		var v uint16
		var skip bool
		var err error
		out, scratch, v, skip, err = appendRDBEntry(out, scratch, key, entry, info)
		if err != nil {
			return err
		}
		if skip {
			out = out[:mark]
			skipped++
			return nil
		}
		selected = db
		version = max(version, v)
		return nil
	})
	return out, version, skipped, err
}

// rdbShard is one shard's encoding: pieces of its worker's chunks.
type rdbShard struct {
	pieces  [][]byte
	version uint16
	skipped int
	err     error
}

const (
	rdbChunkMin   = 64 << 10
	rdbChunkMax   = 4 << 20
	rdbChunkSlack = 4 << 10
)

func encodeRDBShards(parts *RDBParts, shards []EntryShard, info *RedisRDBInfo) (uint16, int, error) {
	encoded := make([]rdbShard, len(shards))
	var next atomic.Int64
	var failed atomic.Bool
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), len(shards)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Each worker appends into chunks that grow up to rdbChunkMax, so
			// the encoding allocates about its own size and a small
			// dataset little.
			var scratch []byte
			chunk := make([]byte, 0, rdbChunkMin)
			for !failed.Load() {
				i := int(next.Add(1) - 1)
				if i >= len(shards) {
					return
				}
				p := &encoded[i]
				p.version = 11
				start := len(chunk)
				p.err = shards[i].Each(func(key string, entry *store.Entry) error {
					mark := len(chunk)
					var v uint16
					var skip bool
					var err error
					chunk, scratch, v, skip, err = appendRDBEntry(chunk, scratch, key, entry, info)
					if err != nil {
						return err
					}
					if skip {
						chunk = chunk[:mark]
						p.skipped++
						return nil
					}
					p.version = max(p.version, v)
					if cap(chunk)-len(chunk) < rdbChunkSlack {
						if len(chunk) > start {
							p.pieces = append(p.pieces, chunk[start:len(chunk):len(chunk)])
						}
						chunk = make([]byte, 0, min(2*cap(chunk), rdbChunkMax))
						start = 0
					}
					return nil
				})
				if len(chunk) > start {
					p.pieces = append(p.pieces, chunk[start:len(chunk):len(chunk)])
				}
				if p.err != nil {
					failed.Store(true)
				}
			}
		}()
	}
	wg.Wait()
	version, skipped, selected := uint16(11), 0, -1
	for i := range encoded {
		p := &encoded[i]
		if p.err != nil {
			return 0, skipped, p.err
		}
		version = max(version, p.version)
		skipped += p.skipped
		if len(p.pieces) == 0 {
			continue
		}
		if db := shards[i].DB; db != selected {
			parts.add(appendDumpLength([]byte{0xFE}, uint64(db)))
			selected = db
		}
		for _, piece := range p.pieces {
			parts.add(piece)
		}
	}
	return version, skipped, nil
}

// appendRDBEntry appends key's RDB record to out, using scratch to encode
// the value. skip reports a key the RDB leaves out: an empty collection, or
// a value Redis has no type for without info.Extensions; out may then hold a
// partial record the caller drops.
func appendRDBEntry(out, scratch []byte, key string, entry *store.Entry, info *RedisRDBInfo) (_, _ []byte, version uint16, skip bool, err error) {
	scratch, version, err = appendDumpValue(scratch[:0], entry.Value)
	if err != nil {
		// An empty collection, or a hash whose fields all expired, is no
		// key.
		if info.Extensions && !errors.Is(err, ErrDumpFormat) {
			return out, scratch, 0, false, fmt.Errorf("key %q: %w", key, err)
		}
		return out, scratch, 0, true, nil
	}
	typ := scratch[0]
	module := rdbModules[typ]
	if module != 0 && !info.Extensions {
		return out, scratch, 0, true, nil
	}
	if tags := entry.TagList(); info.Extensions && len(tags) > 0 {
		enc := appendDumpLength(nil, uint64(len(tags)))
		for _, tag := range tags {
			enc = appendRDBString(enc, tag)
		}
		out = append(out, 0xFA)
		out = appendRDBString(out, rdbTagsAux)
		out = appendRDBString(out, string(enc))
	}
	if at := entry.ExpiresAt.Load(); at > 0 {
		out = append(out, 0xFC)
		out = binary.LittleEndian.AppendUint64(out, uint64(at))
	}
	if info.LFU {
		out = append(out, 0xF9, entry.LFUCount())
	} else if info.IdleTimes {
		out = append(out, 0xF8)
		out = appendDumpLength(out, uint64(entry.IdleSeconds()))
	}
	body := scratch[1:]
	if module != 0 {
		out = append(out, rdbTypeModule2)
		out = appendRDBRawString(out, key)
		out = appendDumpLength(out, module)
		out = appendDumpLength(out, rdbModuleOpcodeStr)
		out = append(out, body...)
		out = appendDumpLength(out, rdbModuleOpcodeEOF)
		return out, scratch, 0, false, nil
	}
	out = append(out, typ)
	out = appendRDBRawString(out, key)
	out = append(out, body...)
	return out, scratch, version, false, nil
}

func appendRDBString(dst []byte, s string) []byte {
	return append(appendDumpLength(dst, uint64(len(s))), s...)
}

// moduleValue reads a module value of CacheStorm's own module types; it
// returns nil without an error for another module's.
func (d *dumpDecoder) moduleValue() store.Value {
	id, _ := d.length()
	var kind byte
	for typ, module := range rdbModules {
		if module == id {
			kind = typ
		}
	}
	if kind == 0 {
		return nil
	}
	if op, _ := d.length(); op != rdbModuleOpcodeStr {
		d.err = ErrDumpFormat
		return nil
	}
	value := d.valueOf(kind)
	if op, _ := d.length(); op != rdbModuleOpcodeEOF {
		d.err = ErrDumpFormat
		return nil
	}
	return value
}

package persistence

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
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
	out, skipped, err := EncodeRedisRDB(src, info)
	if err != nil {
		return skipped, err
	}
	if err := SealRDBChecksum(out); err != nil {
		return skipped, err
	}
	_, err = w.Write(out)
	return skipped, err
}

// EncodeRedisRDB encodes src like WriteRedisRDB but leaves the checksum
// trailer zero, for SealRDBChecksum to fill once the dataset is released.
func EncodeRedisRDB(src EntryStreamer, info RedisRDBInfo) ([]byte, int, error) {
	out := []byte(redisRDBHeader)
	version := uint16(11)
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
	skipped, selected := 0, -1
	err := src.ForEachEntry(func(db int, key string, e interface{}) error {
		entry, ok := e.(*store.Entry)
		if !ok {
			return nil
		}
		payload, err := EncodeDump(entry.Value)
		if err != nil {
			// An empty collection, or a hash whose fields all expired, is no
			// key.
			if info.Extensions && !errors.Is(err, ErrDumpFormat) {
				return fmt.Errorf("key %q: %w", key, err)
			}
			skipped++
			return nil
		}
		typ := payload[0]
		module := rdbModules[typ]
		if module != 0 && !info.Extensions {
			skipped++
			return nil
		}
		if db != selected {
			out = append(out, 0xFE)
			out = appendDumpLength(out, uint64(db))
			selected = db
		}
		if tags := entry.TagList(); info.Extensions && len(tags) > 0 {
			enc := appendDumpLength(nil, uint64(len(tags)))
			for _, tag := range tags {
				enc = appendRDBString(enc, tag)
			}
			aux(rdbTagsAux, string(enc))
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
		body := payload[1 : len(payload)-10]
		if module != 0 {
			out = append(out, rdbTypeModule2)
			out = appendRDBString(out, key)
			out = appendDumpLength(out, module)
			out = appendDumpLength(out, rdbModuleOpcodeStr)
			out = append(out, body...)
			out = appendDumpLength(out, rdbModuleOpcodeEOF)
			return nil
		}
		version = max(version, binary.LittleEndian.Uint16(payload[len(payload)-10:]))
		out = append(out, typ)
		out = appendRDBString(out, key)
		out = append(out, body...)
		return nil
	})
	if err != nil {
		return nil, skipped, err
	}
	copy(out[5:9], fmt.Sprintf("%04d", version))
	out = append(out, 0xFF)
	out = append(out, make([]byte, redisRDBTrailerBytes-1)...)
	return out, skipped, nil
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

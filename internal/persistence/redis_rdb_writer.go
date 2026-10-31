package persistence

import (
	"encoding/binary"
	"io"
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
}

// WriteRedisRDB writes src as an RDB file in Redis' own format, which Redis
// and its tools (redis-check-rdb, replicas, migration tools) load. Values
// Redis has no type for (JSON documents, time series) are left out; it
// returns how many keys it left out. The version is 11, or 12 when a hash
// has field expirations, which only Redis 7.4 can load.
func WriteRedisRDB(w io.Writer, src EntryStreamer, info RedisRDBInfo) (int, error) {
	var body []byte
	version := uint16(11)
	aux := func(k, v string) {
		body = append(body, 0xFA)
		body = appendRDBString(body, k)
		body = appendRDBString(body, v)
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
	for _, code := range info.Functions {
		body = append(body, rdbOpcodeFunction2)
		body = appendRDBString(body, code)
	}
	skipped, selected := 0, -1
	err := src.ForEachEntry(func(db int, key string, e interface{}) error {
		entry, ok := e.(*store.Entry)
		if !ok {
			return nil
		}
		payload, err := EncodeDump(entry.Value)
		if err != nil || len(payload) < 11 || payload[0] >= dumpTypeStream {
			skipped++
			return nil
		}
		if db != selected {
			body = append(body, 0xFE)
			body = appendDumpLength(body, uint64(db))
			selected = db
		}
		if at := entry.ExpiresAt.Load(); at > 0 {
			body = append(body, 0xFC)
			body = binary.LittleEndian.AppendUint64(body, uint64(at))
		}
		version = max(version, binary.LittleEndian.Uint16(payload[len(payload)-10:]))
		body = append(body, payload[0])
		body = appendRDBString(body, key)
		body = append(body, payload[1:len(payload)-10]...)
		return nil
	})
	if err != nil {
		return skipped, err
	}
	out := make([]byte, 0, len(body)+18)
	out = append(out, "REDIS00"...)
	out = strconv.AppendUint(out, uint64(version), 10)
	out = append(out, body...)
	out = append(out, 0xFF)
	out = binary.LittleEndian.AppendUint64(out, rdbChecksum(0, out))
	_, err = w.Write(out)
	return skipped, err
}

func appendRDBString(dst []byte, s string) []byte {
	return append(appendDumpLength(dst, uint64(len(s))), s...)
}

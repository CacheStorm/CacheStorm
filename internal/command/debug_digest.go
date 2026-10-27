package command

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"math"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

type digest [sha1.Size]byte

func (d *digest) xor(o digest) {
	for i := range d {
		d[i] ^= o[i]
	}
}

func sum(parts ...[]byte) digest {
	h := sha1.New()
	for _, p := range parts {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(p)))
		h.Write(n[:])
		h.Write(p)
	}
	var d digest
	copy(d[:], h.Sum(nil))
	return d
}

// valueDigest summarises a value independently of the iteration order of
// unordered collections: their elements are digested one by one and XORed.
func valueDigest(v store.Value) digest {
	var d digest
	switch val := v.(type) {
	case *store.StringValue:
		return sum(val.Data)
	case *store.ListValue:
		val.RLock()
		defer val.RUnlock()
		h := sha1.New()
		for _, el := range val.Elements {
			var n [8]byte
			binary.BigEndian.PutUint64(n[:], uint64(len(el)))
			h.Write(n[:])
			h.Write(el)
		}
		copy(d[:], h.Sum(nil))
	case *store.SetValue:
		val.RLock()
		defer val.RUnlock()
		for m := range val.Members {
			d.xor(sum([]byte(m)))
		}
	case *store.HashValue:
		val.RLock()
		defer val.RUnlock()
		for f, fv := range val.Fields {
			d.xor(sum([]byte(f), fv))
		}
	case *store.SortedSetValue:
		val.RLock()
		defer val.RUnlock()
		for m, score := range val.Members {
			var b [8]byte
			binary.BigEndian.PutUint64(b[:], math.Float64bits(score))
			d.xor(sum([]byte(m), b[:]))
		}
	default:
		if payload, err := persistence.EncodeDump(v); err == nil {
			return sum(payload)
		}
		return sum([]byte(redisTypeName(v)))
	}
	return d
}

// datasetDigest is DEBUG DIGEST: a digest of every key, its type, value and
// expiry in every database, which two servers holding the same data agree on.
// An empty dataset digests to zeros, as in Redis.
func datasetDigest(ctx *Context) string {
	var total digest
	var any bool
	dbs := ctx.databases()
	for i := 0; i < dbs.Count(); i++ {
		st := dbs.Existing(i)
		if st == nil {
			continue
		}
		db := []byte{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)}
		//nolint:errcheck // the callback never fails
		st.ForEachLive(func(key string, e *store.Entry) error {
			vd := valueDigest(e.Value)
			var exp [8]byte
			binary.BigEndian.PutUint64(exp[:], uint64(e.ExpiresAt.Load()))
			total.xor(sum(db, []byte(key), []byte(redisTypeName(e.Value)), vd[:], exp[:]))
			any = true
			return nil
		})
	}
	if !any {
		return "0000000000000000000000000000000000000000"
	}
	return hex.EncodeToString(total[:])
}

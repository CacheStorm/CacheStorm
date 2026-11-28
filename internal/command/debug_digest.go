package command

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/persistence"
	"github.com/cachestorm/cachestorm/internal/store"
)

// digest is Redis' DEBUG DIGEST accumulator (debug.c), so a CacheStorm
// server and a Redis server holding the same data report the same digest.
type digest [sha1.Size]byte

func (d *digest) xor(o digest) {
	for i := range d {
		d[i] ^= o[i]
	}
}

// xorBytes is Redis' xorDigest: the SHA1 of p XORed into d, which makes the
// result independent of the order elements are added in.
func (d *digest) xorBytes(p []byte) { d.xor(sha1.Sum(p)) }

// mix is Redis' mixDigest: xorBytes followed by replacing d with its own
// SHA1, which makes the result depend on the order.
func (d *digest) mix(p []byte) {
	d.xorBytes(p)
	*d = sha1.Sum(d[:])
}

func (d *digest) mixString(s string) { d.mix([]byte(s)) }

// Redis' object types, which the digest mixes in.
const (
	objString uint32 = iota
	objList
	objSet
	objZSet
	objHash
	objModule
	objStream
)

// objectDigest is Redis' xorObjectDigest: the value's type, its contents
// and whether it has an expiry, mixed into d.
func objectDigest(d *digest, e *store.Entry) {
	v := e.Value
	var aux [4]byte
	typ := objModule
	data, isString := stringValueOf(v)
	switch {
	case isString:
		typ = objString
	default:
		switch v.(type) {
		case *store.ListValue:
			typ = objList
		case *store.SetValue:
			typ = objSet
		case *store.SortedSetValue:
			typ = objZSet
		case *store.HashValue:
			typ = objHash
		case *store.StreamValue:
			typ = objStream
		}
	}
	binary.BigEndian.PutUint32(aux[:], typ)
	d.mix(aux[:])

	switch val := v.(type) {
	case *store.ListValue:
		val.RLock()
		for _, el := range val.Elements() {
			d.mix(el)
		}
		val.RUnlock()
	case *store.SetValue:
		val.RLock()
		for m := range val.All() {
			d.xorBytes([]byte(m))
		}
		val.RUnlock()
	case *store.SortedSetValue:
		val.RLock()
		for m, score := range val.All() {
			var ed digest
			ed.mixString(m)
			ed.mixString(fpconvDtoa(score))
			d.xorBytes(ed[:])
		}
		val.RUnlock()
	case *store.HashValue:
		val.RLock()
		for f, fv := range val.All() {
			var ed digest
			ed.mixString(f)
			ed.mix(fv)
			if val.FieldExpireAt(f) != 0 {
				ed.xorBytes([]byte("!!hexpire!!"))
			}
			d.xorBytes(ed[:])
		}
		val.RUnlock()
	case *store.StreamValue:
		val.RLock()
		for _, entry := range val.EntriesLocked() {
			d.mixString(strings.Replace(entry.ID, "-", ".", 1))
			for _, p := range entry.Pairs {
				d.mixString(p.Name)
				d.mix(p.Value)
			}
		}
		val.RUnlock()
	case *store.VectorSetValue:
		d.xorBytes(vectorSetDigest(val))
	default:
		switch {
		case isString:
			d.mix(data)
		case moduleWithoutDigest(v.Type()):
			// The Redis 8 modules of these types register no digest
			// callback, so Redis digests only their type and expiry.
		default:
			// A type only CacheStorm has: its serialized form, so values
			// that differ digest differently.
			if payload, err := persistence.EncodeDump(v); err == nil {
				d.xorBytes(payload)
			}
		}
	}
	if e.ExpiresAt.Load() > 0 {
		d.xorBytes([]byte("!!expire!!"))
	}
}

// keyDigest is DEBUG DIGEST-VALUE for one key: the value's digest without
// the key name, zeros for a missing key. A logically expired key that is
// still stored is digested, as in Redis.
func keyDigest(ctx *Context, key string) string {
	var d digest
	if e, ok := ctx.Store.PeekStored(key); ok {
		objectDigest(&d, e)
	}
	return hex.EncodeToString(d[:])
}

// datasetDigest is DEBUG DIGEST, Redis' computeDatasetDigest: for each
// database with keys its index is mixed in, then the digest of every key's
// name and value is XORed in. An empty dataset digests to zeros.
func datasetDigest(ctx *Context) string {
	var total digest
	dbs := ctx.databases()
	for i := 0; i < dbs.Count(); i++ {
		st := dbs.Existing(i)
		if st == nil || st.KeyCount() == 0 {
			continue
		}
		var db [4]byte
		binary.BigEndian.PutUint32(db[:], uint32(i))
		total.mix(db[:])
		//nolint:errcheck // the callback never fails
		st.ForEachLive(func(key string, e *store.Entry) error {
			var d digest
			d.mixString(key)
			objectDigest(&d, e)
			total.xorBytes(d[:])
			return nil
		})
	}
	return hex.EncodeToString(total[:])
}

// moduleWithoutDigest reports the module types whose Redis 8 module has no
// digest callback: JSON, time series, the probabilistic types and
// FT.SUG dictionaries.
func moduleWithoutDigest(t store.DataType) bool {
	switch t {
	case store.DataTypeJSON, store.DataTypeTimeSeries, store.DataTypeBloom, store.DataTypeCuckoo,
		store.DataTypeCMS, store.DataTypeTopK, store.DataTypeTDigest, store.DataTypeSuggest:
		return true
	}
	return false
}

// vectorSetDigest is Redis' VectorSetDigest (modules/vector-sets/vset.c)
// over the module digest API: the node count and dimension, then each
// node's quantized vector, element and attributes as one sequence, the
// sequences XORed so their order does not matter. It returns the module
// digest's accumulator, which xorObjectDigest hashes into the key's.
func vectorSetDigest(v *store.VectorSetValue) []byte {
	var x, o digest
	end := func() {
		x.xorBytes(o[:])
		o = digest{}
	}
	v.RLock()
	defer v.RUnlock()
	o.mixString(strconv.FormatUint(v.Index.Len(), 10))
	o.mixString(strconv.FormatUint(uint64(v.Index.Dim()), 10))
	end()
	for n := v.Index.Head(); n != nil; n = n.Next() {
		o.mix(n.Raw())
		if e, ok := n.Value.(*store.VectorElem); ok {
			o.mixString(e.Name)
			if e.Attr != nil {
				o.mix(e.Attr)
			}
		}
		end()
	}
	return x[:]
}

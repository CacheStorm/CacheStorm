package persistence

import (
	"slices"
	"sync"
	"sync/atomic"
)

// rdbCompression is rdbcompression: whether RDB files and DUMP payloads
// compress strings longer than 20 bytes with LZF, as Redis does.
var rdbCompression atomic.Bool

func init() { rdbCompression.Store(true) }

// SetRDBCompression applies CONFIG SET rdbcompression.
func SetRDBCompression(on bool) { rdbCompression.Store(on) }

const (
	lzfHashLog = 14
	lzfMaxLit  = 1 << 5
	lzfMaxOff  = 1 << 13
	lzfMaxRef  = (1 << 8) + (1 << 3)
)

// lzfTable holds positions of earlier input by the hash of the three bytes
// there. It is never cleared: a stale position is only a guess, checked
// against the input before it is used.
type lzfTable [1 << lzfHashLog]uint32

var lzfTables = sync.Pool{New: func() any { return new(lzfTable) }}

// lzfCompress writes in compressed in liblzf's format (what Redis' lzf
// decompressor reads) to out and returns its length, or 0 when it does not
// fit in len(out) bytes.
func lzfCompress[T string | []byte](out []byte, in T) int {
	n, end := len(in), len(out)
	if n < 3 || end < 2 {
		return 0
	}
	htab := lzfTables.Get().(*lzfTable)
	defer lzfTables.Put(htab)
	op := 1 // after the first literal run's length byte
	lit := 0
	ip := 0
	hval := uint32(in[0])<<8 | uint32(in[1])
	for ip < n-2 {
		hval = hval<<8 | uint32(in[ip+2])
		slot := ((hval >> (3*8 - lzfHashLog)) - hval*5) & (1<<lzfHashLog - 1)
		ref := int(htab[slot])
		htab[slot] = uint32(ip)
		if off := ip - ref - 1; ref > 0 && ref < ip && off < lzfMaxOff &&
			in[ref] == in[ip] && in[ref+1] == in[ip+1] && in[ref+2] == in[ip+2] {
			if lit == 0 {
				op-- // no literal run before the reference
			}
			// The reference takes up to three bytes, and the next literal
			// run's length one more.
			if op+3+1 >= end {
				return 0
			}
			if lit > 0 {
				out[op-lit-1] = byte(lit - 1)
			}
			maxLen := min(n-ip-2, lzfMaxRef)
			length := 2
			for {
				length++
				if length >= maxLen || in[ref+length] != in[ip+length] {
					break
				}
			}
			length -= 2
			ip++
			if length < 7 {
				out[op] = byte(off>>8 + length<<5)
				op++
			} else {
				out[op] = byte(off>>8 + 7<<5)
				out[op+1] = byte(length - 7)
				op += 2
			}
			out[op] = byte(off)
			op += 2
			lit = 0
			ip += length + 1
			if ip >= n-2 {
				break
			}
			ip--
			hval = uint32(in[ip])<<8 | uint32(in[ip+1])
			hval = hval<<8 | uint32(in[ip+2])
			htab[((hval>>(3*8-lzfHashLog))-hval*5)&(1<<lzfHashLog-1)] = uint32(ip)
			ip++
			continue
		}
		if op >= end {
			return 0
		}
		lit++
		out[op] = in[ip]
		op++
		ip++
		if lit == lzfMaxLit {
			out[op-lit-1] = byte(lit - 1)
			lit = 0
			op++
		}
	}
	for ip < n {
		if op >= end {
			return 0
		}
		lit++
		out[op] = in[ip]
		op++
		ip++
		if lit == lzfMaxLit {
			out[op-lit-1] = byte(lit - 1)
			lit = 0
			op++
		}
	}
	if lit > 0 {
		out[op-lit-1] = byte(lit - 1)
	} else {
		op--
	}
	return op
}

// lzfHeaderMax is the most bytes the header of an LZF string takes: the
// encoding byte and two 64-bit lengths.
const lzfHeaderMax = 1 + 9 + 9

// appendRDBRawString appends s as Redis' rdbSaveRawString does: as an
// integer when it is the canonical decimal form of a 32-bit integer, LZF
// compressed when rdbcompression is on, it is longer than 20 bytes and the
// result takes at most len(s)-4 bytes, and as length and bytes otherwise.
func appendRDBRawString[T string | []byte](dst []byte, s T) []byte {
	if len(s) <= 11 && len(s) > 0 {
		if v, ok := rdbIntegerString(s); ok {
			switch {
			case v >= -1<<7 && v < 1<<7:
				return append(dst, 0xC0, byte(int8(v)))
			case v >= -1<<15 && v < 1<<15:
				return append(dst, 0xC1, byte(v), byte(v>>8))
			default:
				return append(dst, 0xC2, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
			}
		}
	}
	if len(s) > 20 && rdbCompression.Load() {
		// Compress past room for the header, then move the result next to
		// the header once its length is known.
		mark := len(dst)
		dst = slices.Grow(dst, lzfHeaderMax+len(s))
		window := dst[mark+lzfHeaderMax : mark+lzfHeaderMax+len(s)-4]
		if n := lzfCompress(window, s); n > 0 {
			dst = append(dst, 0xC3)
			dst = appendDumpLength(dst, uint64(n))
			dst = appendDumpLength(dst, uint64(len(s)))
			at := len(dst)
			dst = dst[:at+n]
			copy(dst[at:], window[:n])
			return dst
		}
	}
	dst = appendDumpLength(dst, uint64(len(s)))
	return append(dst, s...)
}

// rdbIntegerString reports whether s is the canonical decimal form of a
// 32-bit integer (no sign but '-', no leading zeros, not "-0"), as Redis'
// rdbTryIntegerEncoding checks. s is at most 11 bytes.
func rdbIntegerString[T string | []byte](s T) (int64, bool) {
	i, neg := 0, false
	if s[0] == '-' {
		i, neg = 1, true
	}
	if i == len(s) || (s[i] == '0' && (len(s) > i+1 || neg)) {
		return 0, false
	}
	var v int64
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + int64(c-'0')
	}
	if neg {
		v = -v
	}
	if v < -1<<31 || v >= 1<<31 {
		return 0, false
	}
	return v, true
}

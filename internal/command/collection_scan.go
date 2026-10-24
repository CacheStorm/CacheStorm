package command

import (
	"container/heap"
	"errors"
	"hash/fnv"
	"math"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/resp"
)

// SSCAN, HSCAN and ZSCAN walk a collection in the order of a fixed 64-bit
// hash of each member. The cursor is the smallest hash the next call still
// has to return, so a member present for the whole scan is returned exactly
// once whatever is added or removed in between: its hash never changes and
// the cursor only moves forward. A page never splits a group of members that
// share a hash, so a collision cannot hide a member behind the cursor.
//
// Collections of at most collectionScanSmall members are returned whole with
// cursor 0, the way Redis answers for its compact listpack/intset encodings.

const collectionScanSmall = 128

var (
	errInvalidCursor  = errors.New("ERR invalid cursor")
	errNoValuesOption = errors.New("ERR NOVALUES option can only be used in HSCAN")
)

type collectionScanOpts struct {
	count    int
	pattern  string
	match    bool
	noValues bool
}

// parseCollectionScanCursor parses an unsigned SCAN cursor as Redis'
// string2ull does: a leading '+' is accepted, a negative value is not.
func parseCollectionScanCursor(arg string) (uint64, error) {
	if arg == "-0" {
		return 0, nil
	}
	arg = strings.TrimPrefix(arg, "+")
	if arg == "" || arg[0] == '+' || arg[0] == '-' {
		return 0, errInvalidCursor
	}
	c, err := strconv.ParseUint(arg, 10, 64)
	if err != nil {
		return 0, errInvalidCursor
	}
	return c, nil
}

// parseCollectionScanOptions parses "[MATCH pattern] [COUNT count]" (and
// NOVALUES for HSCAN) starting at argument from.
func parseCollectionScanOptions(ctx *Context, from int, isHash bool) (collectionScanOpts, error) {
	opts := collectionScanOpts{count: 10}
	n := ctx.ArgCount()
	for i := from; i < n; {
		left := n - i
		switch strings.ToUpper(ctx.ArgString(i)) {
		case "COUNT":
			if left < 2 {
				return opts, ErrSyntaxError
			}
			c, err := strconv.ParseInt(ctx.ArgString(i+1), 10, 64)
			if err != nil {
				return opts, ErrNotInteger
			}
			if c < 1 {
				return opts, ErrSyntaxError
			}
			if c > math.MaxInt32 {
				c = math.MaxInt32
			}
			opts.count = int(c)
			i += 2
		case "MATCH":
			if left < 2 {
				return opts, ErrSyntaxError
			}
			opts.pattern = ctx.ArgString(i + 1)
			opts.match = opts.pattern != "*"
			i += 2
		case "NOVALUES":
			if !isHash {
				return opts, errNoValuesOption
			}
			opts.noValues = true
			i++
		default:
			return opts, ErrSyntaxError
		}
	}
	return opts, nil
}

func collectionScanHash(member string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(member))
	return h.Sum64()
}

// hashMaxHeap is a max-heap of member hashes.
type hashMaxHeap []uint64

func (h hashMaxHeap) Len() int           { return len(h) }
func (h hashMaxHeap) Less(i, j int) bool { return h[i] > h[j] }
func (h hashMaxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *hashMaxHeap) Push(x any)        { *h = append(*h, x.(uint64)) }
func (h *hashMaxHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}

// collectionScanPage selects the members of one SCAN page. each must call its
// argument once for every member of the collection, and the caller holds the
// collection's lock across the call. It returns the members of the page and
// the next cursor (0 once the scan is complete).
//
// The page is every member whose hash lies in [cursor, limit], where limit is
// the count-th smallest hash at or after the cursor, so members sharing a hash
// always land on the same page.
func collectionScanPage(size int, cursor uint64, count int, each func(func(string))) ([]string, uint64) {
	if size <= collectionScanSmall {
		members := make([]string, 0, size)
		each(func(m string) { members = append(members, m) })
		return members, 0
	}

	smallest := make(hashMaxHeap, 0, min(count, size))
	each(func(m string) {
		h := collectionScanHash(m)
		if h < cursor {
			return
		}
		if len(smallest) < count {
			heap.Push(&smallest, h)
		} else if h < smallest[0] {
			smallest[0] = h
			heap.Fix(&smallest, 0)
		}
	})
	if len(smallest) == 0 {
		return nil, 0
	}
	limit := uint64(math.MaxUint64)
	if len(smallest) == count {
		limit = smallest[0]
	}

	members := make([]string, 0, len(smallest))
	more := false
	each(func(m string) {
		h := collectionScanHash(m)
		if h < cursor {
			return
		}
		if h <= limit {
			members = append(members, m)
		} else {
			more = true
		}
	})
	if !more || limit == math.MaxUint64 {
		return members, 0
	}
	return members, limit + 1
}

// writeCollectionScanReply writes the [cursor, items] SCAN reply.
func writeCollectionScanReply(ctx *Context, cursor uint64, items []*resp.Value) error {
	return ctx.WriteArray([]*resp.Value{
		resp.BulkString(strconv.FormatUint(cursor, 10)),
		resp.ArrayValue(items),
	})
}

// writeEmptyCollectionScan is the reply for a scan over a missing key.
func writeEmptyCollectionScan(ctx *Context) error {
	return writeCollectionScanReply(ctx, 0, []*resp.Value{})
}

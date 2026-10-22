package command

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

// snowflakeUniquenessID issues one SNOWFLAKE.NEXT through the real router and
// returns the integer ID it replied with.
func snowflakeUniquenessID(t *testing.T, router *Router, s *store.Store) int64 {
	t.Helper()

	ctx, buf := bufCtx("SNOWFLAKE.NEXT", bytesArgs("1"), s)
	if err := router.Execute(ctx); err != nil {
		t.Fatalf("SNOWFLAKE.NEXT returned an error: %v", err)
	}

	reply := buf.String()
	if !strings.HasPrefix(reply, "$") {
		t.Fatalf("SNOWFLAKE.NEXT replied %q, want a bulk string", reply)
	}
	// Bulk string: "$<len>\r\n<payload>\r\n"
	parts := strings.SplitN(reply, "\r\n", 2)
	if len(parts) != 2 {
		t.Fatalf("SNOWFLAKE.NEXT replied %q, want a bulk string", reply)
	}
	payload := strings.TrimSuffix(parts[1], "\r\n")

	id, err := strconv.ParseInt(payload, 10, 64)
	if err != nil {
		t.Fatalf("SNOWFLAKE.NEXT replied %q, which is not an integer ID: %v", payload, err)
	}
	return id
}

// snowflakeUniquenessMillis extracts the millisecond bucket an ID was minted
// in. The snowflake layout is (millis-epoch)<<22 | node<<12 | sequence, so the
// high bits are the millisecond and the low 12 bits are the sequence.
func snowflakeUniquenessMillis(id int64) int64 {
	return id >> 22
}

// snowflakeUniquenessSeq extracts the intra-millisecond sequence number.
func snowflakeUniquenessSeq(id int64) int64 {
	return id & 0xFFF
}

// snowflakeUniquenessBuckets issues SNOWFLAKE.NEXT repeatedly and groups the
// returned IDs by the millisecond they were minted in.
//
// It keeps calling until some millisecond holds at least two IDs, so the
// caller can compare IDs that really were minted within the same
// millisecond. The wall-clock deadline bounds the wait: on a heavily loaded
// machine a single call can take longer than a millisecond, and a fixed
// attempt count could then expire before two calls ever shared a millisecond.
func snowflakeUniquenessBuckets(t *testing.T, router *Router, s *store.Store) map[int64][]int64 {
	t.Helper()

	buckets := make(map[int64][]int64)
	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		id := snowflakeUniquenessID(t, router, s)
		millis := snowflakeUniquenessMillis(id)
		buckets[millis] = append(buckets[millis], id)

		for _, ids := range buckets {
			if len(ids) >= 2 {
				return buckets
			}
		}
	}

	return buckets
}

// TestSnowflakeNextIssuesUniqueIDs pins the contract that SNOWFLAKE.NEXT
// never hands out the same ID twice.
//
// A snowflake ID exists to be a globally unique identifier. The generator's
// 12-bit sequence field is the part that makes IDs unique for requests minted
// inside the SAME millisecond, so two IDs sharing a millisecond must still
// differ.
//
// The bug this pins: the command built a brand-new generator per call, so
// that generator's sequence always restarted at 0 and the ID became a pure
// function of the millisecond. Two replies inside one millisecond were then
// byte-identical.
//
// The test collects IDs until it can compare two that were minted in the same
// millisecond, rather than assuming the wall clock ticks between two calls.
func TestSnowflakeNextIssuesUniqueIDs(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityCommands(router)

	// A reused generator is the reference behaviour this command must match:
	// it advances the sequence, so same-millisecond IDs stay distinct. This
	// control passes before and after the fix and proves the store layer is
	// not the defect.
	ref := store.NewSnowflakeIDGenerator(1)
	refA, refB := ref.Next(), ref.Next()
	if refA == refB {
		t.Fatalf("control failed: a reused generator issued the same ID twice (%d)", refA)
	}

	buckets := snowflakeUniquenessBuckets(t, router, s)

	for millis, ids := range buckets {
		if len(ids) < 2 {
			continue
		}
		for i := 1; i < len(ids); i++ {
			if ids[i] == ids[0] {
				t.Fatalf("SNOWFLAKE.NEXT issued the same ID %d twice within "+
					"millisecond %d (sequence %d both times) — a snowflake ID "+
					"must be unique even within one millisecond",
					ids[0], millis, snowflakeUniquenessSeq(ids[0]))
			}
		}
		// Compared two IDs minted in the same millisecond: they differ.
		return
	}

	t.Fatalf("could not mint two SNOWFLAKE.NEXT replies within one millisecond, " +
		"so uniqueness was never exercised")
}

// TestSnowflakeNextSequenceAdvances is the secondary branch check: within one
// millisecond the sequence field must advance rather than staying pinned at
// zero, which is the mechanism that produces distinct IDs.
//
// It collects the same same-millisecond IDs as the test above, but it asserts
// on the sequence field itself, so it catches the defect even in the case
// where two colliding IDs somehow differed in another field.
func TestSnowflakeNextSequenceAdvances(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterUtilityCommands(router)

	buckets := snowflakeUniquenessBuckets(t, router, s)

	for millis, ids := range buckets {
		if len(ids) < 2 {
			continue
		}
		for i := 1; i < len(ids); i++ {
			if snowflakeUniquenessSeq(ids[i]) == snowflakeUniquenessSeq(ids[0]) {
				t.Fatalf("two SNOWFLAKE.NEXT IDs minted in millisecond %d both "+
					"carry sequence %d — the intra-millisecond sequence did not "+
					"advance, so the IDs are not unique",
					millis, snowflakeUniquenessSeq(ids[0]))
			}
		}
		return
	}

	t.Fatalf("could not mint two SNOWFLAKE.NEXT replies within one millisecond, " +
		"so the sequence was never exercised")
}

package store

import "testing"

// TestGetSnowflakeIDGeneratorReusesOneGenerator pins the property that makes
// a snowflake ID unique: the generator holds the intra-millisecond sequence
// counter, so the SAME generator must be handed out for the same node.
//
// A caller that built a fresh generator per ID got a sequence that restarted
// at zero every time, so every ID minted in one millisecond was identical.
// That is exactly what the SNOWFLAKE.NEXT command did.
func TestGetSnowflakeIDGeneratorReusesOneGenerator(t *testing.T) {
	gen := GetSnowflakeIDGenerator(1)
	if gen == nil {
		t.Fatal("GetSnowflakeIDGenerator returned nil")
	}

	again := GetSnowflakeIDGenerator(1)
	if again != gen {
		t.Fatal("GetSnowflakeIDGenerator returned a different generator for the " +
			"same node; the intra-millisecond sequence counter would restart and " +
			"the IDs would repeat within one millisecond")
	}

	// The reused generator must actually advance its sequence.
	first, second := gen.Next(), gen.Next()
	if first == second {
		t.Fatalf("a reused generator issued the same ID twice (%d)", first)
	}
}

// TestGetSnowflakeIDGeneratorNormalizesInvalidNode covers the secondary
// branch: an out-of-range node ID is clamped to 0, exactly as
// NewSnowflakeIDGenerator does.
//
// The registry key must be normalized the same way. Otherwise two DIFFERENT
// invalid node IDs (say -1 and 99999) would get separate generators that
// share the same effective node bits, each with its own sequence counter, and
// would mint duplicate IDs for the same millisecond.
func TestGetSnowflakeIDGeneratorNormalizesInvalidNode(t *testing.T) {
	neg := GetSnowflakeIDGenerator(-1)
	tooBig := GetSnowflakeIDGenerator(99999)
	zero := GetSnowflakeIDGenerator(0)

	if neg != zero {
		t.Error("a negative node ID did not resolve to the node-0 generator")
	}
	if tooBig != zero {
		t.Error("an out-of-range node ID did not resolve to the node-0 generator")
	}

	// Distinct valid nodes must still get distinct generators, otherwise
	// nodes would emit identical IDs for the same millisecond.
	if GetSnowflakeIDGenerator(1) == GetSnowflakeIDGenerator(2) {
		t.Error("two different valid node IDs shared one generator")
	}
}

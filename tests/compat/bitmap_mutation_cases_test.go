//go:build compat

package compat

func bitmapMutationCases() []Case {
	return []Case{
		k("bitmap", "signed-set-extreme-saturation", c("BITFIELD", "k", "OVERFLOW", "SAT", "SET", "i8", "0", "-9223372036854775808", "GET", "i8", "0", "SET", "i39", "8", "-9223372036854775808", "GET", "i39", "8"), c("GET", "k")),
		k("bitmap", "setbit-validation", c("SETBIT", "k", "+1", "1"), c("SETBIT", "k", "01", "1"), c("SETBIT", "k", "-1", "1"), c("SETBIT", "k", "0", "+1"), c("SETBIT", "k", "0", "2"), c("SETBIT", "k", "0", "invalid"), c("EXISTS", "k")),
		k("bitmap", "read-missing", c("BITFIELD", "missing", "GET", "u8", "100"), c("EXISTS", "missing"), c("BITFIELD_RO", "missing", "GET", "i64", "0"), c("EXISTS", "missing")),
		k("bitmap", "invalid-suffix-atomic", c("SET", "k", "original"), c("BITFIELD", "k", "SET", "u8", "0", "7", "INVALID"), c("GET", "k")),
		k("bitmap", "encoding-validation", c("BITFIELD", "k", "SET", "u64", "0", "1"), c("BITFIELD", "k", "GET", "I8", "0"), c("BITFIELD", "k", "GET", "U8", "0"), c("BITFIELD", "k", "GET", "u08", "0"), c("BITFIELD", "k", "GET", "i65", "0"), c("EXISTS", "k")),
		k("bitmap", "scaled-offset", c("BITFIELD", "k", "SET", "u8", "#1", "65", "GET", "u8", "8", "INCRBY", "i8", "#1", "-1"), c("GET", "k")),
		k("bitmap", "offset-validation", c("BITFIELD", "k", "GET", "u8", "9223372036854775807"), c("BITFIELD", "k", "GET", "i64", "#9223372036854775807"), c("BITFIELD", "k", "GET", "u8", "#-1"), c("BITFIELD", "k", "GET", "u8", "#01"), c("EXISTS", "k")),
		k("bitmap", "read-last-offset", c("BITFIELD", "k", "GET", "i64", "4294967295"), c("EXISTS", "k")),
		k("bitmap", "overflow-set", c("BITFIELD", "k", "OVERFLOW", "SAT", "SET", "u8", "0", "-1", "GET", "u8", "0", "SET", "i8", "8", "-999", "GET", "i8", "8", "OVERFLOW", "FAIL", "SET", "u8", "0", "256", "GET", "u8", "0"), c("GET", "k")),
		k("bitmap", "overflow-controls", c("BITFIELD", "k", "OVERFLOW", "WRONG"), c("BITFIELD", "k", "OVERFLOW", "FAIL", "SET", "u8", "100", "256"), c("STRLEN", "k"), c("BITFIELD", "k", "GET", "u8", "100")),
		k("bitmap", "signed64-overflow", c("BITFIELD", "k", "SET", "i64", "0", "9223372036854775807", "OVERFLOW", "FAIL", "INCRBY", "i64", "0", "1", "OVERFLOW", "SAT", "INCRBY", "i64", "0", "1", "OVERFLOW", "WRAP", "INCRBY", "i64", "0", "1", "GET", "i64", "0")),
		k("bitmap", "readonly-write-rejected", c("BITFIELD_RO", "k", "SET", "u8", "0", "1"), c("EXISTS", "k"), c("BITFIELD_RO", "k", "OVERFLOW", "SAT")),
		k("bitmap", "empty-and-wrongtype", c("BITFIELD", "missing"), c("EXISTS", "missing"), c("RPUSH", "k", "x"), c("BITFIELD", "k"), c("BITFIELD", "k", "GET", "u8", "0")),
		k("bitmap", "mixed-overlap", c("SET", "k", "\xff"), c("BITFIELD", "k", "GET", "i16", "4", "SET", "u5", "7", "23", "GET", "u8", "0", "GET", "u8", "8"), c("GET", "k")),
		k("bitmap", "no-op-watch", c("SETBIT", "k", "0", "1"), on(1, "WATCH", "k"), c("SETBIT", "k", "0", "1"), on(1, "MULTI"), on(1, "GET", "k"), on(1, "EXEC")),
		k("bitmap", "read-only-watch", c("SET", "k", "v"), on(1, "WATCH", "k"), c("BITFIELD", "k", "GET", "u8", "0"), on(1, "MULTI"), on(1, "GET", "k"), on(1, "EXEC")),
	}
}

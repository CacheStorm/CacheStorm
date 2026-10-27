//go:build compat

package compat

func hashGrowthCases() []Case {
	return []Case{
		k("hashes", "hsetnx-conflict-watch", c("HSET", "h", "f", "kept"), on(1, "WATCH", "h"), c("HSETNX", "h", "f", "wrong"), on(1, "MULTI"), on(1, "HGET", "h", "f"), on(1, "EXEC")),
		k("hashes", "hincrby-invalid-field-watch", c("HSET", "h", "f", "text"), on(1, "WATCH", "h"), c("HINCRBY", "h", "f", "1"), on(1, "MULTI"), on(1, "HGET", "h", "f"), on(1, "EXEC")),
		k("hashes", "hincrbyfloat-invalid-field-watch", c("HSET", "h", "f", "text"), on(1, "WATCH", "h"), c("HINCRBYFLOAT", "h", "f", "1"), on(1, "MULTI"), on(1, "HGET", "h", "f"), on(1, "EXEC")),
		k("hashes", "hincrby-overflow-watch", c("HSET", "h", "f", "9223372036854775807"), on(1, "WATCH", "h"), c("HINCRBY", "h", "f", "1"), on(1, "MULTI"), on(1, "HGET", "h", "f"), on(1, "EXEC")),
		k("hashes", "hash-invalid-input-missing-watch", on(1, "WATCH", "h"), c("HINCRBY", "h", "f", "invalid"), c("HINCRBYFLOAT", "h", "f", "NaN"), on(1, "MULTI"), on(1, "EXISTS", "h"), on(1, "EXEC")),
		k("hashes", "hset-duplicate-final-fields", c("HSET", "h", "a", "first", "b", "other", "a", "last"), c("HMGET", "h", "a", "b"), c("HMSET", "h", "a", "changed", "a", "final"), c("HGET", "h", "a")),
		k("hashes", "hash-growth-preserves-ttl", c("HSET", "h", "f", "99"), c("PEXPIREAT", "h", "4102444800000"), c("HSET", "h", "g", "value"), c("HMSET", "h", "other", "value"), c("HSETNX", "h", "nx", "value"), c("HINCRBY", "h", "f", "1"), c("HINCRBYFLOAT", "h", "f", "0.5"), c("PEXPIRETIME", "h"), c("HGET", "h", "f")),
	}
}

//go:build compat

package compat

func msetBatchCases() []Case {
	return []Case{
		k("strings", "mset-final-duplicate", c("MSET", "a", "first", "b", "other", "a", "last"), c("MGET", "a", "b")),
		k("strings", "msetnx-final-duplicate", c("MSETNX", "a", "first", "a", "last", "b", "other"), c("MGET", "a", "b")),
		k("strings", "mset-clears-ttl", c("SET", "a", "old", "PXAT", "4102444800000"), c("SET", "b", "old", "PXAT", "4102444800000"), c("MSET", "a", "new", "b", ""), c("MGET", "a", "b"), c("PEXPIRETIME", "a"), c("PEXPIRETIME", "b")),
		k("strings", "msetnx-conflict-watch", c("SET", "a", "old"), on(1, "WATCH", "a", "b"), c("MSETNX", "b", "new", "a", "replace"), on(1, "MULTI"), on(1, "MGET", "a", "b"), on(1, "EXEC")),
		k("strings", "mset-destination-watch", c("SET", "a", "old"), on(1, "WATCH", "a", "b"), c("MSET", "a", "old", "b", "new"), on(1, "MULTI"), on(1, "MGET", "a", "b"), on(1, "EXEC")),
		k("strings", "mset-replaces-collection", c("RPUSH", "a", "one"), c("HSET", "b", "field", "value"), c("MSET", "a", "string", "b", "other"), c("MGET", "a", "b")),
	}
}

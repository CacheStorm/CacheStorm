//go:build compat

package compat

func entryTransferCases() []Case {
	return []Case{
		k("transfer", "copy-source-watch", c("SET", "s", "v"), on(1, "WATCH", "s"), c("COPY", "s", "d"), on(1, "MULTI"), on(1, "GET", "s"), on(1, "EXEC")),
		k("transfer", "copy-destination-watch", c("SET", "s", "v"), on(1, "WATCH", "d"), c("COPY", "s", "d"), on(1, "MULTI"), on(1, "GET", "d"), on(1, "EXEC")),
		k("transfer", "copy-noop-watch", c("MSET", "s", "source", "d", "dest"), on(1, "WATCH", "s", "d"), c("COPY", "s", "d"), on(1, "MULTI"), on(1, "MGET", "s", "d"), on(1, "EXEC")),
		k("transfer", "rename-self-watch", c("SET", "s", "v"), on(1, "WATCH", "s"), c("RENAME", "s", "s"), on(1, "MULTI"), on(1, "GET", "s"), on(1, "EXEC")),
		k("transfer", "renamenx-noop-watch", c("MSET", "s", "source", "d", "dest"), on(1, "WATCH", "s", "d"), c("RENAMENX", "s", "d"), on(1, "MULTI"), on(1, "MGET", "s", "d"), on(1, "EXEC")),
		k("transfer", "rename-overwrite-watch", c("MSET", "s", "source", "d", "dest"), on(1, "WATCH", "s", "d"), c("RENAME", "s", "d"), on(1, "MULTI"), on(1, "GET", "d"), on(1, "EXEC"), c("MGET", "s", "d")),
		k("transfer", "empty-destination", c("SET", "s", "source"), c("RENAME", "s", ""), c("GET", ""), c("COPY", "", "other"), c("GET", "other")),
		k("transfer", "copy-replace-ttl", c("SET", "s", "source", "PXAT", "4102444800000"), c("SET", "d", "dest", "PXAT", "4202444800000"), c("COPY", "s", "d", "REPLACE"), c("PEXPIRETIME", "s"), c("PEXPIRETIME", "d")),
		k("transfer", "copy-hash-independent", c("HSET", "s", "field", "source"), c("COPY", "s", "d"), c("HSET", "d", "field", "dest"), c("HGET", "s", "field"), c("HGET", "d", "field")),
		k("transfer", "copy-stream-pending-independent", c("XADD", "s", "1-0", "field", "v"), c("XGROUP", "CREATE", "s", "g", "0"), c("XREADGROUP", "GROUP", "g", "consumer", "STREAMS", "s", ">"), c("COPY", "s", "d"), c("XACK", "d", "g", "1-0"), c("XPENDING", "s", "g"), c("XPENDING", "d", "g")),
		k("db", "move-noop-source-watch", c("SET", "s", "zero"), c("SELECT", "1"), c("SET", "s", "one"), c("SELECT", "0"), on(1, "WATCH", "s"), c("MOVE", "s", "1"), on(1, "MULTI"), on(1, "GET", "s"), on(1, "EXEC")),
		k("db", "copy-transfer-cross-db", c("SET", "s", "source", "PXAT", "4102444800000"), c("COPY", "s", "d", "DB", "1"), c("SELECT", "1"), c("GET", "d"), c("PEXPIRETIME", "d"), c("MOVE", "d", "0"), c("SELECT", "0"), c("GET", "d"), c("GET", "s")),
	}
}

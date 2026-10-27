//go:build compat

package compat

func readIsolationCases() []Case {
	var cases []Case
	for _, name := range []string{"SUNIONSTORE", "SINTERSTORE", "SDIFFSTORE"} {
		cases = append(cases,
			k("sets", name+"-read-source-watch", c("SADD", "s", "m"), on(1, "WATCH", "s"), c(name, "d", "s"), on(1, "MULTI"), on(1, "SCARD", "s"), on(1, "EXEC"), srt("SMEMBERS", "d")),
			k("sets", name+"-empty-noop-watch", on(1, "WATCH", "d"), c(name, "d", "missing"), on(1, "MULTI"), on(1, "SCARD", "d"), on(1, "EXEC")),
			k("sets", name+"-error-watch", c("SADD", "s", "m"), c("SET", "wrong", "text"), c("SET", "d", "kept"), on(1, "WATCH", "s", "d"), c(name, "d", "s", "wrong"), on(1, "MULTI"), on(1, "GET", "d"), on(1, "EXEC")),
		)
	}
	return append(cases,
		k("strings", "mget-duplicate-key", c("SET", "k", "value"), c("MGET", "k", "k", "missing", "k")),
		k("sets", "plain-numeric-key-list", c("SADD", "0", "a"), c("SADD", "2", "b"), srt("SUNION", "0", "2"), srt("SDIFF", "0", "2"), srt("SINTER", "0", "2"), c("SUNIONSTORE", "d", "0", "2"), srt("SMEMBERS", "d"), c("SINTERCARD", "2", "0", "2")),
	)
}

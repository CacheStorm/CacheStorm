//go:build compat

package compat

func setAdmissionCases() []Case {
	return []Case{
		k("sets", "sadd-noop-watch", c("SADD", "s", "m"), on(1, "WATCH", "s"), c("SADD", "s", "m", "m"), on(1, "MULTI"), on(1, "SCARD", "s"), on(1, "EXEC")),
		k("sets", "smove-absent-watch", c("SADD", "s", "m"), on(1, "WATCH", "s", "d"), c("SMOVE", "s", "d", "absent"), on(1, "MULTI"), on(1, "SCARD", "s"), on(1, "EXEC")),
		k("sets", "smove-self-watch", c("SADD", "s", "m"), on(1, "WATCH", "s"), c("SMOVE", "s", "s", "m"), on(1, "MULTI"), on(1, "SCARD", "s"), on(1, "EXEC")),
		k("sets", "smove-duplicate-dest-watch", c("SADD", "s", "m", "stay"), c("SADD", "d", "m"), on(1, "WATCH", "d"), c("SMOVE", "s", "d", "m"), on(1, "MULTI"), on(1, "SCARD", "d"), on(1, "EXEC"), srt("SMEMBERS", "s")),
		k("sets", "smove-source-watch", c("SADD", "s", "m"), on(1, "WATCH", "s"), c("SMOVE", "s", "d", "m"), on(1, "MULTI"), on(1, "SCARD", "s"), on(1, "EXEC")),
		k("sets", "smove-existing-dest-ttl", c("SADD", "s", "m"), c("SADD", "d", "other"), c("PEXPIREAT", "s", "4102444800000"), c("PEXPIREAT", "d", "4202444800000"), c("SMOVE", "s", "d", "m"), c("PEXPIRETIME", "s"), c("PEXPIRETIME", "d")),
		k("sets", "smove-new-dest-ttl", c("SADD", "s", "m"), c("PEXPIREAT", "s", "4102444800000"), c("SMOVE", "s", "d", "m"), c("PEXPIRETIME", "d")),
		k("sets", "sadd-preserves-ttl", c("SADD", "s", "old"), c("PEXPIREAT", "s", "4102444800000"), c("SADD", "s", "m", "m"), c("PEXPIRETIME", "s"), c("SADD", "s", "m"), c("SCARD", "s")),
		k("sets", "set-existing-empty-name", c("SADD", "s", "m"), c("RENAME", "s", ""), c("SADD", "", "new"), c("SMOVE", "", "d", "new"), c("SMOVE", "d", "", "new"), c("SCARD", ""), c("SCARD", "d")),
		k("sets", "smove-wrongtype-watch", c("SADD", "s", "m"), c("SET", "d", "string"), on(1, "WATCH", "s", "d"), c("SMOVE", "s", "d", "m"), on(1, "MULTI"), on(1, "SCARD", "s"), on(1, "EXEC")),
	}
}

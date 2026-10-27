//go:build compat

package compat

func hllCases() []Case {
	cached := "HYLL\x01\x00\x00\x00\x07\x00\x00\x00\x00\x00\x00\x00"
	return []Case{
		k("hll", "empty", c("PFADD", "h"), c("PFCOUNT", "h"), c("PFADD", "h"), c("TYPE", "h"), c("STRLEN", "h")),
		k("hll", "add-duplicates", c("PFADD", "h", "a", "b", "c"), c("PFCOUNT", "h"), c("PFADD", "h", "a", "b", "c"), c("PFCOUNT", "h")),
		k("hll", "binary-elements", c("PFADD", "h", "", "\x00", "a\x00b", "çözüm"), c("PFCOUNT", "h"), c("PFADD", "h", ""), c("PFCOUNT", "h")),
		k("hll", "union", c("PFADD", "a", "x", "y"), c("PFADD", "b", "y", "z"), c("PFCOUNT", "a", "b", "missing"), c("PFCOUNT", "a", "a"), c("PFCOUNT", "missing")),
		k("hll", "merge-destination", c("PFADD", "dest", "old"), c("PFADD", "src", "new"), c("PFMERGE", "dest", "src"), c("PFCOUNT", "dest"), c("PFMERGE", "dest", "dest", "src"), c("PFCOUNT", "dest")),
		k("hll", "merge-empty", c("PFMERGE", "h"), c("PFCOUNT", "h"), c("TYPE", "h")),
		k("hll", "ttl", c("PFADD", "h", "a"), c("PEXPIREAT", "h", "4102444800000"), c("PFADD", "h", "b"), c("PEXPIRETIME", "h"), c("PFCOUNT", "h"), c("PEXPIRETIME", "h"), c("PFMERGE", "h"), c("PEXPIRETIME", "h")),
		k("hll", "wrongtype", c("SET", "s", "invalid"), c("RPUSH", "l", "x"), c("PFADD", "s", "a"), c("PFCOUNT", "s"), c("PFMERGE", "s"), c("PFCOUNT", "l"), c("PFADD", "l", "a")),
		k("hll", "invalid-source-atomic", c("PFADD", "dest", "a"), c("SET", "bad", "invalid"), c("PFMERGE", "dest", "bad"), c("PFCOUNT", "dest"), c("PFMERGE", "missing", "bad"), c("EXISTS", "missing")),
		k("hll", "cached-header", c("SET", "h", cached), c("PFCOUNT", "h"), c("PFADD", "h"), c("PFCOUNT", "h", "missing"), c("PFADD", "h", "element")),
		k("hll", "count-cache-watch", c("PFADD", "h", "a"), on(1, "WATCH", "h"), c("PFCOUNT", "h"), on(1, "MULTI"), on(1, "GET", "h"), on(1, "EXEC")),
		k("hll", "duplicate-watch", c("PFADD", "h", "a"), on(1, "WATCH", "h"), c("PFADD", "h", "a"), on(1, "MULTI"), on(1, "TYPE", "h"), on(1, "EXEC")),
	}
}

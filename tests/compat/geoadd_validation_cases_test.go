//go:build compat

package compat

func geoAddValidationCases() []Case {
	return []Case{
		k("geoadd", "invalid-batch-atomic", c("GEOADD", "geo", "0", "0", "seed"), c("GEOADD", "geo", "1", "1", "partial", "200", "0", "bad"), c("GEOPOS", "geo", "partial"), c("EXISTS", "geo")),
		k("geoadd", "invalid-first-missing", c("GEOADD", "geo", "200", "0", "bad"), c("EXISTS", "geo")),
		k("geoadd", "invalid-numeric-atomic", c("GEOADD", "geo", "1", "1", "partial", "bad", "0", "invalid"), c("EXISTS", "geo")),
		k("geoadd", "nan-infinity", c("GEOADD", "a", "nan", "0", "bad"), c("GEOADD", "b", "0", "nan", "bad"), c("GEOADD", "c", "inf", "0", "bad"), c("GEOADD", "d", "0", "-inf", "bad"), c("EXISTS", "a", "b", "c", "d")),
		k("geoadd", "conflicting-options", c("GEOADD", "geo", "NX", "XX", "0", "0", "a"), c("EXISTS", "geo")),
		k("geoadd", "xx-still-validates", c("GEOADD", "geo", "XX", "200", "0", "bad"), c("EXISTS", "geo")),
		k("geoadd", "no-op-watch", c("GEOADD", "geo", "0", "0", "a"), on(1, "WATCH", "geo"), c("GEOADD", "geo", "NX", "1", "1", "a"), on(1, "MULTI"), on(1, "EXISTS", "geo"), on(1, "EXEC")),
		k("geoadd", "invalid-watch", c("GEOADD", "geo", "0", "0", "a"), on(1, "WATCH", "geo"), c("GEOADD", "geo", "1", "1", "partial", "200", "0", "bad"), on(1, "MULTI"), on(1, "EXISTS", "geo"), on(1, "EXEC")),
	}
}

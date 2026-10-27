//go:build compat

package compat

func geoZsetCases() []Case {
	seed := c("GEOADD", "geo", "13.361389", "38.115556", "Palermo", "15.087269", "37.502669", "Catania")
	return []Case{
		k("geo", "readonly-radius", seed, c("GEORADIUS_RO", "geo", "15", "37", "200", "km", "WITHDIST", "ASC"), c("GEORADIUSBYMEMBER_RO", "geo", "Catania", "200", "km", "ASC"), c("GEORADIUS_RO", "geo", "15", "37", "200", "km", "STORE", "dest"), c("EXISTS", "dest")),
		k("geo", "radius-read-watch", seed, on(1, "WATCH", "geo"), c("GEORADIUS", "geo", "15", "37", "200", "km", "COUNT", "1"), c("GEORADIUSBYMEMBER", "geo", "Catania", "200", "km", "COUNT", "1"), on(1, "MULTI"), on(1, "EXISTS", "geo"), on(1, "EXEC")),
		k("geo", "missing-center-preserves", seed, c("SET", "dest", "retained"), c("GEOSEARCHSTORE", "dest", "geo", "FROMMEMBER", "absent", "BYRADIUS", "200", "km"), c("GET", "dest"), c("GEOSEARCH", "geo", "FROMMEMBER", "absent", "BYRADIUS", "200", "km"), c("GEORADIUSBYMEMBER", "geo", "absent", "200", "km")),
		k("geo", "invalid-geometry-preserves", seed, c("SET", "dest", "retained"), c("GEOSEARCHSTORE", "dest", "geo", "FROMLONLAT", "0", "0", "BYRADIUS", "nan", "km"), c("GEOSEARCHSTORE", "dest", "geo", "FROMLONLAT", "0", "0", "BYRADIUS", "-1", "km"), c("GEOSEARCHSTORE", "dest", "geo", "FROMLONLAT", "0", "0", "BYBOX", "1", "-1", "km"), c("GET", "dest")),
		k("geo", "invalid-options-preserve", seed, c("SET", "dest", "retained"), c("GEOSEARCHSTORE", "dest", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "COUNT", "0"), c("GEOSEARCHSTORE", "dest", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "UNKNOWN"), c("GEOSEARCHSTORE", "dest", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "ANY"), c("GET", "dest")),
		k("geo", "unit-error", c("GEODIST", "absent", "a", "b", "wrong"), c("GEORADIUS", "absent", "0", "0", "1", "wrong")),
		k("geo", "zset-interoperability", seed, c("TYPE", "geo"), c("ZRANGE", "geo", "0", "-1", "WITHSCORES"), c("ZSCORE", "geo", "Palermo"), c("ZREM", "geo", "Palermo"), c("GEOPOS", "geo", "Palermo", "Catania")),
		k("geo", "read-coordinates-hash-distance", seed, c("GEOPOS", "geo", "Palermo", "Catania", "absent"), c("GEOHASH", "geo", "Palermo", "Catania", "absent"), c("GEODIST", "geo", "Palermo", "Catania"), c("GEODIST", "geo", "Palermo", "Catania", "km"), c("GEODIST", "geo", "Palermo", "Catania", "ft"), c("GEODIST", "geo", "Palermo", "Catania", "mi")),
		k("geo", "raw-score-import", c("ZADD", "geo", "3479099956230698", "Palermo", "3479447370796909", "Catania"), c("GEOPOS", "geo", "Palermo", "Catania"), c("GEOHASH", "geo", "Palermo"), c("GEOADD", "geo", "CH", "13.361389", "38.115556", "Palermo")),
		k("geo", "quantized-watch", seed, on(1, "WATCH", "geo"), c("GEOADD", "geo", "CH", "13.361389000001", "38.115556000001", "Palermo"), on(1, "MULTI"), on(1, "EXISTS", "geo"), on(1, "EXEC")),
		k("geo", "boundary-coordinates", c("GEOADD", "geo", "-180", "-85.05112878", "min", "180", "85.05112878", "max", "0", "0", "origin"), c("ZRANGE", "geo", "0", "-1", "WITHSCORES"), c("GEOPOS", "geo", "min", "max", "origin"), c("GEOHASH", "geo", "min", "max", "origin")),
		k("geo", "wrongtype-readers", c("SET", "geo", "wrong"), c("GEOPOS", "geo", "a"), c("GEOHASH", "geo", "a"), c("GEODIST", "geo", "a", "b"), c("GEOSEARCH", "geo", "FROMLONLAT", "0", "0", "BYRADIUS", "1", "km"), c("GEORADIUS", "geo", "0", "0", "1", "km"), c("GEORADIUSBYMEMBER", "geo", "a", "1", "km")),
		k("geo", "wrongtype-store-preserves", c("SET", "src", "wrong"), c("SET", "dest", "retained"), c("GEOSEARCHSTORE", "dest", "src", "FROMLONLAT", "0", "0", "BYRADIUS", "1", "km"), c("GET", "dest"), c("GEORADIUS", "src", "0", "0", "1", "km", "STORE", "dest"), c("GET", "dest")),
		k("geo", "radius-with-options", seed, c("GEORADIUS", "geo", "15", "37", "200", "km", "WITHDIST", "WITHHASH", "WITHCOORD", "ASC"), c("GEORADIUSBYMEMBER", "geo", "Catania", "200", "km", "WITHDIST", "WITHHASH", "WITHCOORD", "ASC")),
		k("geo", "search-with-options", seed, c("GEOSEARCH", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "WITHDIST", "WITHHASH", "WITHCOORD", "DESC"), c("GEOSEARCH", "geo", "FROMLONLAT", "15", "37", "BYRADIUS", "200", "km", "WITHDIST", "ASC")),
		k("geo", "box-two-dimensions", c("GEOADD", "geo", "0", "0", "origin", "0", "1", "north", "1", "0", "east"), c("GEOSEARCH", "geo", "FROMLONLAT", "0", "0", "BYBOX", "10", "500", "km", "ASC"), c("GEOSEARCHSTORE", "box", "geo", "FROMLONLAT", "0", "0", "BYBOX", "500", "10", "km", "ASC"), c("ZRANGE", "box", "0", "-1", "WITHSCORES")),
		k("geo", "store-scores", seed, c("GEORADIUS", "geo", "15", "37", "200", "km", "STORE", "dest"), c("ZRANGE", "dest", "0", "-1", "WITHSCORES"), c("GEOSEARCHSTORE", "other", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km"), c("ZRANGE", "other", "0", "-1", "WITHSCORES")),
		k("geo", "storedist-scores", seed, c("GEOSEARCHSTORE", "dest", "geo", "FROMMEMBER", "Catania", "BYRADIUS", "200", "km", "STOREDIST"), c("ZRANGE", "dest", "0", "-1", "WITHSCORES")),
	}
}

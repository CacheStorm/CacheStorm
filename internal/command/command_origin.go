package command

import "strings"

// Command origins: a command Redis 7.4 has, one of the Redis Stack modules
// (RedisJSON, RediSearch, RedisTimeSeries, RedisBloom) has, or one only
// CacheStorm has.
const (
	OriginRedis     = "redis"
	OriginModule    = "module"
	OriginExtension = "extension"
)

// CommandOrigin says where a registered command name comes from.
func CommandOrigin(name string) string {
	if redisCommandMeta(name) != nil || CanonicalName(name) == "SENTINEL" {
		return OriginRedis
	}
	if moduleCommand(name) != nil {
		return OriginModule
	}
	return OriginExtension
}

// utilityNamespaces are the extension namespaces whose commands keep no
// state: functions of their arguments (string, math, encoding, hashing,
// formatting, validation of a value, id generation) that an application
// computes as easily itself. They widen what a client can make the server
// compute without storing anything, so extension-commands "stateful", the
// default, leaves them out.
var utilityNamespaces = map[string]bool{
	"CBOR": true, "COMPRESS": true, "DECOMPRESS": true, "CRYPTO": true,
	"CSV": true, "DIFF": true, "DIGEST": true, "EVAL": true, "MASK": true,
	"MATH": true, "MSGPACK": true, "PARALLEL": true, "SANITIZE": true,
	"SNOWFLAKE": true, "STR": true, "TIMESTAMP": true, "ULID": true,
	"URL": true, "UUID": true, "XML": true, "YAML": true,
}

// Values of the extension-commands setting.
const (
	ExtensionsAll      = "all"
	ExtensionsStateful = "stateful"
	ExtensionsNone     = "none"
)

// ExtensionCommandEnabled says whether a command stays registered under the
// extension-commands setting: Redis and Redis Stack commands always do.
func ExtensionCommandEnabled(name, setting string) bool {
	if CommandOrigin(name) != OriginExtension {
		return true
	}
	switch setting {
	case ExtensionsNone:
		return false
	case ExtensionsStateful:
		ns, _, dotted := strings.Cut(CanonicalName(name), ".")
		return !dotted || !utilityNamespaces[ns]
	}
	return true
}

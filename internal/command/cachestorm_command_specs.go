package command

// cachestormCommandSpecs describes CacheStorm's own commands the way Redis'
// command table describes Redis': arity, flags, key positions and ACL
// categories. Without an entry a command is unknown to everything that reads
// the table, so a tag write ran inside EVAL_RO, did not count towards the
// save rules (rdb_changes_since_last_save), was not refused over maxmemory,
// and COMMAND INFO reported it as taking any number of arguments and no key.
var cachestormCommandSpecs = map[string]commandSpec{
	"SETTAG":      {Arity: -4, Flags: []string{"write", "denyoom"}, FirstKey: 1, LastKey: 1, KeyStep: 1, Categories: []string{"write", "string", "slow"}},
	"ADDTAG":      {Arity: -3, Flags: []string{"write", "denyoom", "fast"}, FirstKey: 1, LastKey: 1, KeyStep: 1, Categories: []string{"write", "keyspace", "fast"}},
	"REMTAG":      {Arity: -3, Flags: []string{"write", "fast"}, FirstKey: 1, LastKey: 1, KeyStep: 1, Categories: []string{"write", "keyspace", "fast"}},
	"TAGS":        {Arity: 2, Flags: []string{"readonly", "fast"}, FirstKey: 1, LastKey: 1, KeyStep: 1, Categories: []string{"read", "keyspace", "fast"}},
	"TAGKEYS":     {Arity: -2, Flags: []string{"readonly"}, Categories: []string{"read", "keyspace", "slow"}},
	"TAGCOUNT":    {Arity: -2, Flags: []string{"readonly", "fast"}, Categories: []string{"read", "keyspace", "fast"}},
	"TAGCHILDREN": {Arity: 2, Flags: []string{"readonly"}, Categories: []string{"read", "keyspace", "slow"}},
	"INVALIDATE":  {Arity: -2, Flags: []string{"write"}, Categories: []string{"write", "keyspace", "slow"}},
	"TAGLINK":     {Arity: -3, Flags: []string{"write", "denyoom"}, Categories: []string{"write", "keyspace", "slow"}},
	"TAGSYNC":     {Arity: 1, Flags: []string{"admin", "noscript"}, Categories: []string{"admin", "slow", "dangerous"}},
	"TAGUNLINK":   {Arity: -3, Flags: []string{"write"}, Categories: []string{"write", "keyspace", "slow"}},
}

// cachestormCommandSummaries are the COMMAND DOCS summaries of the commands
// in cachestormCommandSpecs.
var cachestormCommandSummaries = map[string]string{
	"SETTAG":      "Sets the string value of a key and replaces its tags.",
	"ADDTAG":      "Adds tags to a key. Returns how many it did not have.",
	"REMTAG":      "Removes tags from a key. Returns how many it had.",
	"TAGS":        "Returns the tags of a key.",
	"TAGKEYS":     "Returns the keys that carry a tag.",
	"TAGCOUNT":    "Returns the number of keys that carry a tag.",
	"TAGCHILDREN": "Returns the tags linked below a tag.",
	"INVALIDATE":  "Deletes every key that carries one of the tags, and with CASCADE the tags linked below them, atomically.",
	"TAGLINK":     "Links a child tag below a parent tag, for INVALIDATE ... CASCADE.",
	"TAGUNLINK":   "Removes the link between a parent and a child tag.",
	"TAGSYNC":     "Returns the tag hierarchy with its versions, for the other nodes of a cluster.",
}

// redisCommandSpecs is the command table everything reads: Redis' own
// commands and CacheStorm's.
var redisCommandSpecs = func() map[string]commandSpec {
	m := make(map[string]commandSpec, len(redisNativeCommandSpecs)+len(cachestormCommandSpecs))
	for name, spec := range redisNativeCommandSpecs {
		m[name] = spec
	}
	for name, spec := range cachestormCommandSpecs {
		m[name] = spec
	}
	return m
}()

//go:build compat

package compat

import "strings"

const (
	SevCritical = 1
	SevCommon   = 2
	SevEdge     = 3
)

var sevTitle = map[int]string{
	SevCritical: "Severity 1: data loss, isolation/atomicity break, hang, or feature completely non-functional",
	SevCommon:   "Severity 2: wrong reply for common usage",
	SevEdge:     "Severity 3: edge cases and error-text differences",
	0:           "Unclassified (new failure, needs triage)",
}

type Verdict struct {
	Sev   int
	Class string
	Note  string
}

const (
	bug    = "bug"
	intent = "intentional difference"
	noise  = "harness noise"
)

var verdicts = map[string]Verdict{
	"strings/set-exat-pxat-past":       {SevEdge, bug, "SET EXAT/PXAT with a past (or far-future) timestamp is rejected; Redis accepts and expires immediately"},
	"keys/expiretime":                  {SevEdge, bug, "EXPIREAT/PEXPIREAT with a year-3021 timestamp rejected as 'invalid expire time'"},
	"streams/xadd-explicit":            {SevCommon, bug, "XADD with '<ms>-*' or bare '<ms>' IDs rejected"},
	"streams/xadd-zero-id":             {SevEdge, bug, "failed XADD (arity error) leaves an empty stream key behind"},
	"streams/xrange-count-rev":         {SevEdge, bug, "XRANGE exclusive start '(1-0' rejected"},
	"streams/xread":                    {SevEdge, bug, "XREAD with no data replies $-1 (Redis *-1); arity text"},
	"streams/xgroup-create":            {SevEdge, bug, "XGROUP CREATE on missing key: '-key not found' instead of Redis' ERR message"},
	"streams/xreadgroup-ack-pending":   {SevCommon, bug, "XREADGROUP entries are flattened (id, fields, id, fields) instead of [[id, fields], ...]; XPENDING consumer counts are integers not bulk strings and fully-acked consumers still listed; history read (ID 0) returns nil"},
	"streams/xreadgroup-errors":        {SevEdge, bug, "NOGROUP message text shorter"},
	"streams/xpending-extended-shape":  {SevCommon, bug, "same XREADGROUP flattened-entry shape bug"},
	"wrongtype/XRANGE":                 {SevEdge, bug, "XRANGE on a string key returns empty array instead of WRONGTYPE"},
	"wrongtype/XLEN":                   {SevEdge, bug, "XLEN on a string key returns 0 instead of WRONGTYPE"},
	"misc/object-encoding-strings":     {SevEdge, bug, "OBJECT ENCODING replies simple string instead of bulk; int/raw encodings not reported (encoding names partly intentional)"},
	"misc/object-encoding-collections": {SevEdge, intent, "internal encodings differ (quicklist/hashtable/skiplist vs listpack/intset); reply is simple string instead of bulk"},
	"misc/object-misc":                 {SevEdge, intent, "OBJECT FREQ allowed under CacheStorm's default eviction policy; unknown-subcommand text differs"},
	"misc/client-setname":              {SevEdge, bug, "CLIENT SETNAME accepts names with spaces"},
	"misc/hello-2":                     {SevCommon, bug, "HELLO not implemented (clients that negotiate protocol on connect fall back or fail)"},
	"lua/redis-call-errors":            {SevCritical, bug, "KEYS nil; redis.call errors are returned as bulk strings instead of raising, so scripts continue after a failed write; error texts differ"},
}

var arityAccepts = map[string]string{
	"arity/PING-2": "PING a b accepted (returns 'a')",
}

func verdictFor(name string) Verdict {
	if v, ok := verdicts[name]; ok {
		return v
	}
	if strings.HasPrefix(name, "arity/") {
		if n, ok := arityAccepts[name]; ok {
			return Verdict{SevEdge, bug, n}
		}
		return Verdict{SevEdge, bug, "arity error is 'ERR wrong number of arguments' without \"for '<cmd>' command\""}
	}
	return Verdict{}
}

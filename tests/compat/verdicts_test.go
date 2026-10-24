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
	"misc/object-encoding-strings":     {SevEdge, bug, "OBJECT ENCODING replies simple string instead of bulk; int/raw encodings not reported (encoding names partly intentional)"},
	"misc/object-encoding-collections": {SevEdge, intent, "internal encodings differ (quicklist/hashtable/skiplist vs listpack/intset); reply is simple string instead of bulk"},
	"misc/object-misc":                 {SevEdge, intent, "OBJECT FREQ allowed under CacheStorm's default eviction policy; unknown-subcommand text differs"},
	"lua/redis-call-errors":            {SevCritical, bug, "KEYS nil; redis.call errors are returned as bulk strings instead of raising, so scripts continue after a failed write; error texts differ"},
}

var arityAccepts = map[string]string{}

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

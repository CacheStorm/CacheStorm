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
	"restore/overflow-relative-ttl":  {SevEdge, intent, "relative TTL overflow is rejected before mutation; Redis 7.4.11 acknowledges it, logs a negative ABSTTL and loses the key on AOF restart (reproduced in the restore/dump production-readiness round)"},
	"lua/redis-call-errors":          {SevEdge, intent, "compile errors carry gopher-lua's parser message (\"near 'error': parse error\") instead of Lua 5.1's"},
	"acl/dryrun-cat-whoami":          {SevEdge, intent, "ACL CAT also lists the module categories (@bloom ... @json) of Redis 8.2, whose bundled modules CacheStorm implements; Redis 7.4 has no modules"},
	"command/count-and-list-filters": {SevEdge, intent, "COMMAND LIST also lists XACKDEL (and COMMAND COUNT counts it, with XDELEX, DELEX, DIGEST and MSETEX), commands of Redis 8.2 and 8.4 CacheStorm implements; Redis 7.4 does not have them"},
	"command/getkeys-forms":          {SevEdge, intent, "COMMAND DOCS SET lists the IFEQ, IFNE, IFDEQ and IFDNE conditions and the 8.4.0 history entry Redis 8.4 added, which CacheStorm implements; Redis 7.4 does not have them"},
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

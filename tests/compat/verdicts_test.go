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
	"strings/set-exat-pxat-past": {SevEdge, bug, "expiry instants after the year 2262 are capped (expiry is stored as int64 nanoseconds), so TTL reports the capped instant"},
	"keys/expiretime":            {SevEdge, bug, "expiry instants after the year 2262 are capped (expiry is stored as int64 nanoseconds), so EXPIRETIME/PEXPIRETIME/TTL report the capped instant"},
	"lua/redis-call-errors":      {SevEdge, intent, "compile errors carry gopher-lua's parser message (\"near 'error': parse error\") instead of Lua 5.1's"},
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

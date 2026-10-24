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
	"strings/set-nx-get":                       {SevCommon, bug, "SET NX GET on an existing key returns error '-key already exists' instead of the old value"},
	"strings/set-ex-invalid":                   {SevEdge, bug, "invalid-expire error text differs ('ERR invalid argument')"},
	"strings/set-syntax-errors":                {SevCommon, bug, "SET accepts conflicting EX+PX and KEEPTTL+EX instead of syntax error, and writes the key"},
	"strings/set-exat-pxat-past":               {SevEdge, bug, "SET EXAT/PXAT with a past (or far-future) timestamp is rejected; Redis accepts and expires immediately"},
	"strings/set-ex-overflow":                  {SevEdge, bug, "error text lacks \"in 'set' command\""},
	"strings/getex-errors":                     {SevEdge, bug, "GETEX accepts EX+PX together; error text differs"},
	"strings/incr-not-int":                     {SevEdge, bug, "INCR accepts \"01\" (leading zero) as an integer"},
	"strings/incr-overflow":                    {SevEdge, bug, "DECRBY -9223372036854775808 overflow not detected"},
	"strings/incrbyfloat":                      {SevCommon, bug, "INCRBYFLOAT formats with float64 shortest repr: 0.1+0.2 gives 0.30000000000000004 (Redis: 0.3, long double %.17Lg)"},
	"strings/incrbyfloat-errors":               {SevEdge, bug, "error texts differ; 1.7e308+1.7e308 errors (Redis long double succeeds)"},
	"strings/incrbyfloat-format":               {SevEdge, bug, "-0, 1e-20 and 18-digit inputs formatted differently"},
	"strings/setrange":                         {SevEdge, bug, "SETRANGE with empty value on a missing key creates the key (Redis: returns 0, no key); error texts differ"},
	"strings/mset-mget":                        {SevEdge, bug, "arity error text lacks command name"},
	"strings/binary-values":                    {SevCommon, bug, "keys containing NUL/CR/LF are rejected with 'ERR invalid key name' (Redis keys are binary safe)"},
	"strings/setnx-setex-psetex-getset":        {SevEdge, bug, "SETEX 0 error text differs"},
	"keys/expire-basic":                        {SevCommon, bug, "PERSIST on a key without TTL returns 1 (Redis 0)"},
	"keys/expire-option-conflicts":             {SevEdge, bug, "EXPIRE NX/XX/GT/LT conflict error texts are a generic 'syntax error'"},
	"keys/expire-negative-deletes":             {SevCommon, bug, "PEXPIRE key 0 returns an error and leaves the key alive (Redis deletes it)"},
	"keys/expiretime":                          {SevEdge, bug, "EXPIREAT/PEXPIREAT with a year-3021 timestamp rejected as 'invalid expire time'"},
	"keys/rename":                              {SevEdge, bug, "missing-key error is '-key not found' (no ERR prefix) instead of '-ERR no such key'"},
	"keys/renamenx":                            {SevEdge, bug, "missing-key error is '-key not found' instead of '-ERR no such key'"},
	"keys/scan-type":                           {SevCommon, bug, "SCAN ... TYPE <type> not supported (syntax error)"},
	"keys/scan-errors":                         {SevEdge, bug, "SCAN invalid cursor / COUNT 0 error texts differ"},
	"keys/keys-pattern":                        {SevCommon, bug, "glob character classes like [1-3] never match (KEYS returns empty)"},
	"hashes/hincrby":                           {SevEdge, bug, "HINCRBY on non-integer field: text differs ('hash value is not an integer')"},
	"hashes/hincrbyfloat":                      {SevEdge, bug, "error texts differ"},
	"hashes/hmset":                             {SevEdge, bug, "arity error text lacks command name"},
	"hashes/hset-odd-args":                     {SevEdge, bug, "arity error text lacks command name"},
	"hashes/hscan":                             {SevCritical, bug, "full HSCAN iteration returns duplicate fields and skips others (SCAN guarantee broken: clients silently miss data)"},
	"lists/lindex-lset":                        {SevEdge, bug, "LSET on missing key: '-key not found' instead of '-ERR no such key'"},
	"lists/pop-count":                          {SevCommon, bug, "LPOP/RPOP <count> ignores count: pops a single element and replies with a bulk string; count 0 / negative not handled"},
	"lists/lpos":                               {SevEdge, bug, "LPOS COUNT 0 with no match returns nil (Redis empty array); negative COUNT/MAXLEN accepted; RANK error texts differ"},
	"lists/lmpop-errors":                       {SevEdge, bug, "LMPOP numkeys/count error texts differ"},
	"blocking/blpop-timeout-errors":            {SevEdge, bug, "timeout error text differs"},
	"blocking/blpop-wrongtype":                 {SevEdge, bug, "BLPOP on a string key intermittently waits and returns nil instead of WRONGTYPE (failed in 2 of 3 runs)"},
	"blocking/blmove-absent":                   {SevEdge, bug, "BLMOVE/BRPOPLPUSH timeout replies $-1 (Redis *-1 null array)"},
	"sets/set-algebra":                         {SevEdge, bug, "SINTERCARD 0 error text differs"},
	"sets/sscan":                               {SevCritical, bug, "full SSCAN iteration skips members (27 of 40 returned): clients silently miss data"},
	"zsets/zadd-incr":                          {SevEdge, bug, "ZADD NX/XX INCR that does not apply replies :0 instead of nil; GT INCR rejected"},
	"zsets/zadd-errors":                        {SevEdge, bug, "ZADD NX GT accepted (Redis rejects); option-conflict texts differ"},
	"zsets/zadd-inf-scores":                    {SevCommon, bug, "ZADD with -inf/+inf scores rejected ('not a valid float')"},
	"zsets/score-format":                       {SevEdge, bug, "score 1e20 printed as 100000000000000000000 (Redis 1e+20); -0 printed as -0"},
	"zsets/zrange-byscore":                     {SevCommon, bug, "ZRANGE ... BYSCORE REV returns empty; LIMIT without BYSCORE/BYLEX accepted"},
	"zsets/zrange-bylex":                       {SevCommon, bug, "ZRANGE ... BYLEX REV returns empty"},
	"zsets/zrangebyscore-legacy":               {SevCommon, bug, "exclusive score bounds '(3' rejected by ZREVRANGEBYSCORE and ZCOUNT"},
	"zsets/zrank":                              {SevEdge, bug, "ZRANK/ZREVRANK WITHSCORE (7.2) not supported"},
	"zsets/zrangestore":                        {SevCommon, bug, "ZRANGESTORE BYSCORE REV stores wrong range; exclusive bounds rejected"},
	"zsets/zunion-zinter":                      {SevEdge, bug, "ZINTERCARD not implemented"},
	"zsets/zunion-with-set":                    {SevEdge, bug, "ZUNION/ZINTERSTORE reject plain sets as inputs (WRONGTYPE)"},
	"zsets/zunionstore":                        {SevEdge, bug, "numkeys error texts differ"},
	"zsets/zmpop":                              {SevCommon, bug, "ZMPOP reply is flat [m,s,m,s] instead of [[m,s],[m,s]]; empty reply is $-1 instead of *-1"},
	"zsets/zpopmin-max":                        {SevCommon, bug, "ZPOPMAX with count returns ascending order (Redis highest first)"},
	"zsets/zrem-ranges":                        {SevCommon, bug, "ZREMRANGEBYSCORE rejects exclusive bound '(2'"},
	"zsets/zincrby":                            {SevEdge, bug, "ZINCRBY non-float increment error text differs"},
	"zsets/zrandmember":                        {SevCommon, bug, "ZRANDMEMBER <count> ignores count (returns single bulk)"},
	"zsets/zscan":                              {SevCommon, bug, "ZSCAN MATCH with character class never matches (same glob bug as KEYS)"},
	"streams/xadd-explicit":                    {SevCommon, bug, "XADD with '<ms>-*' or bare '<ms>' IDs rejected"},
	"streams/xadd-zero-id":                     {SevEdge, bug, "failed XADD (arity error) leaves an empty stream key behind"},
	"streams/xrange-count-rev":                 {SevEdge, bug, "XRANGE exclusive start '(1-0' rejected"},
	"streams/xread":                            {SevEdge, bug, "XREAD with no data replies $-1 (Redis *-1); arity text"},
	"streams/xgroup-create":                    {SevEdge, bug, "XGROUP CREATE on missing key: '-key not found' instead of Redis' ERR message"},
	"streams/xreadgroup-ack-pending":           {SevCommon, bug, "XREADGROUP entries are flattened (id, fields, id, fields) instead of [[id, fields], ...]; XPENDING consumer counts are integers not bulk strings and fully-acked consumers still listed; history read (ID 0) returns nil"},
	"streams/xreadgroup-errors":                {SevEdge, bug, "NOGROUP message text shorter"},
	"streams/xpending-extended-shape":          {SevCommon, bug, "same XREADGROUP flattened-entry shape bug"},
	"transactions/execabort-unknown-command":   {SevEdge, bug, "unknown-command error text lacks \"with args beginning with\""},
	"transactions/execabort-wrong-arity":       {SevCritical, bug, "wrong-arity command inside MULTI is QUEUED and EXEC runs the rest (partial transaction applied; Redis aborts with EXECABORT)"},
	"transactions/watch-touched-aborts":        {SevCommon, bug, "aborted EXEC replies $-1 instead of *-1 (null array); clients that type-check the reply mis-handle WATCH failures"},
	"transactions/watch-missing-key-created":   {SevCommon, bug, "aborted EXEC replies $-1 instead of *-1"},
	"transactions/watch-deleted-key":           {SevCommon, bug, "aborted EXEC replies $-1 instead of *-1"},
	"transactions/watch-expire-touch":          {SevCommon, bug, "EXPIRE on a WATCHed key does not abort the transaction"},
	"transactions/watch-flushall":              {SevCommon, bug, "aborted EXEC replies $-1 instead of *-1"},
	"transactions/watch-other-db-no-abort":     {SevCritical, bug, "consequence of SELECT being a no-op: write in db1 aborts a WATCH in db0"},
	"transactions/exec-discard-without-multi":  {SevEdge, bug, "WATCH with no keys accepted"},
	"transactions/multi-select":                {SevCritical, bug, "consequence of SELECT being a no-op"},
	"transactions/exec-after-execabort-resets": {SevEdge, bug, "unknown-command error text"},
	"wrongtype/XRANGE":                         {SevEdge, bug, "XRANGE on a string key returns empty array instead of WRONGTYPE"},
	"wrongtype/XLEN":                           {SevEdge, bug, "XLEN on a string key returns 0 instead of WRONGTYPE"},
	"wrongtype/BITCOUNT":                       {SevEdge, bug, "BITCOUNT on a list key returns 0 instead of WRONGTYPE"},
	"pubsub/subscribe-publish-deliver":         {SevCritical, bug, "PUBLISH reports 1 receiver but the message frame never reaches the subscriber socket"},
	"pubsub/psubscribe-deliver":                {SevCritical, bug, "pmessage never delivered"},
	"pubsub/subscribe-multi-channel":           {SevCritical, bug, "messages never delivered; first subscribe ack carries count 2 (should be 1); later PUBLISH counts 0"},
	"pubsub/two-subscribers-count":             {SevCritical, bug, "neither subscriber receives the message"},
	"pubsub/pattern-and-channel-both":          {SevCritical, bug, "no delivery; PSUBSCRIBE ack count ignores existing channel subscription"},
	"pubsub/unsubscribe":                       {SevCritical, bug, "no delivery; UNSUBSCRIBE-all replies with nil channel instead of the channel name"},
	"pubsub/unsubscribe-not-subscribed":        {SevCommon, bug, "(un)subscribe acks use simple string '+unsubscribe' instead of bulk string"},
	"pubsub/ping-in-subscribe-mode":            {SevEdge, bug, "PING in subscribed mode not returned as [pong, msg]; GET allowed in subscribed mode"},
	"pubsub/pubsub-introspection":              {SevCommon, bug, "subscribe acks use '+subscribe' simple string instead of bulk; multi-channel ack counts wrong"},
	"pubsub/binary-message":                    {SevCritical, bug, "message never delivered"},
	"pubsub/publish-from-multi":                {SevCritical, bug, "message never delivered"},
	"pubsub/publish-from-lua":                  {SevCritical, bug, "PUBLISH unavailable inside Lua; message never delivered"},
	"pubsub/ssubscribe-spublish":               {SevCritical, bug, "smessage never delivered"},
	"db/select-dbsize":                         {SevCritical, bug, "SELECT is a no-op: all databases share one keyspace"},
	"db/select-errors":                         {SevEdge, bug, "SELECT 16 accepted; out-of-range text differs"},
	"db/move":                                  {SevCritical, bug, "MOVE reports 1 but the key stays (SELECT no-op); MOVE to same db not rejected"},
	"db/flushdb-only-current":                  {SevCritical, bug, "FLUSHDB in db 1 wipes db 0 (data loss)"},
	"db/swapdb":                                {SevCritical, bug, "SWAPDB has no effect (single keyspace)"},
	"db/select-per-connection":                 {SevCritical, bug, "a key written in db 1 is visible to and overwritten by another connection in db 0"},
	"db/copy-db":                               {SevCommon, bug, "COPY ... DB 1 returns 0"},
	"misc/object-encoding-strings":             {SevEdge, bug, "OBJECT ENCODING replies simple string instead of bulk; int/raw encodings not reported (encoding names partly intentional)"},
	"misc/object-encoding-collections":         {SevEdge, intent, "internal encodings differ (quicklist/hashtable/skiplist vs listpack/intset); reply is simple string instead of bulk"},
	"misc/object-misc":                         {SevEdge, intent, "OBJECT FREQ allowed under CacheStorm's default eviction policy; unknown-subcommand text differs"},
	"misc/unknown-command":                     {SevEdge, bug, "unknown-command error text lacks \"with args beginning with\""},
	"misc/case-insensitive":                    {SevCritical, bug, "lowercase/mixed-case command names are rejected as unknown (router lookup is case-sensitive); breaks clients that send lowercase (e.g. go-redis, ioredis)"},
	"misc/inline-commands":                     {SevCommon, bug, "inline protocol (e.g. `echo PING | nc`) closes the connection ('invalid RESP type')"},
	"misc/client-setname":                      {SevEdge, bug, "CLIENT SETNAME accepts names with spaces"},
	"misc/hello-2":                             {SevCommon, bug, "HELLO not implemented (clients that negotiate protocol on connect fall back or fail)"},
	"lua/return-types":                         {SevCommon, bug, "Lua->RESP conversion: {ok=..}/{err=..}/status_reply/error_reply become arrays, false becomes :0, floats not truncated, nil holes not truncated"},
	"lua/redis-call-types":                     {SevCritical, bug, "KEYS/ARGV not populated (KEYS[1] -> 'attempt to index nil'); status replies to Lua are strings not tables; missing key is nil not false; PING unavailable"},
	"lua/redis-call-errors":                    {SevCritical, bug, "KEYS nil; redis.call errors are returned as bulk strings instead of raising, so scripts continue after a failed write; error texts differ"},
	"lua/keys-argv":                            {SevCritical, bug, "KEYS/ARGV tables not populated"},
	"lua/evalsha":                              {SevCritical, bug, "EVALSHA script cannot read ARGV; NOSCRIPT text lacks trailing period"},
	"lua/lua-writes-state":                     {SevCritical, bug, "script using KEYS fails, so no writes happen"},
	"lua/lua-float-int-conv":                   {SevCommon, bug, "ARGV nil; INCRBYFLOAT and many commands unavailable to redis.call; cjson missing"},
	"lua/lua-select-isolated":                  {SevCritical, bug, "consequence of SELECT being a no-op"},
}

var arityAccepts = map[string]string{
	"arity/PING-2":   "PING a b accepted (returns 'a')",
	"arity/MULTI-1":  "MULTI x accepted",
	"arity/WATCH":    "WATCH with no keys accepted",
	"arity/DBSIZE-1": "DBSIZE x accepted",
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

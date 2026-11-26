Redis version: 7.4.12

Cases: 561, pass: 555, fail: 6

| Severity | Failing cases |
|---|---|
| Severity 1: data loss, isolation/atomicity break, hang, or feature completely non-functional | 0 |
| Severity 2: wrong reply for common usage | 0 |
| Severity 3: edge cases and error-text differences | 6 |
| Unclassified (new failure, needs triage) | 0 |

By classification: bug 0, intentional difference 6, harness noise 0, unclassified 0

## Severity 3: edge cases and error-text differences

### restore/overflow-relative-ttl

**intentional difference**: relative TTL overflow is rejected before mutation; Redis 7.4.11 acknowledges it, logs a negative ABSTTL and loses the key on AOF restart (reproduced in the restore/dump production-readiness round)

```
!! c0> RESTORE k 9223372036854775807 "\x00\ba\x00b\r\n,=&\f\x00y\xfc[ex\x88pi"
     redis:  "+OK\r\n"
     cstorm: "-ERR invalid expire time in 'restore' command\r\n"
!! c0> EXISTS k
     redis:  ":1\r\n"
     cstorm: ":0\r\n"
!! STATE DIFF (keyspace after the case)
--- redis
db0 "k" type=string value="$8\r\na\x00b\r\n,=&\r\n" ttl=yes
--- cachestorm
```

### acl/dryrun-cat-whoami

**intentional difference**: ACL CAT also lists the module categories (@bloom ... @json) of Redis 8.2, whose bundled modules CacheStorm implements; Redis 7.4 has no modules

```
   c0> ACL SETUSER d on nopass ~a* +get
     both:   "+OK\r\n"
   c0> ACL DRYRUN d GET a1
     both:   "+OK\r\n"
   c0> ACL DRYRUN d GET b1
     both:   "$48\r\nUser d has no permissions to access the 'b1' key\r\n"
   c0> ACL DRYRUN d SET a1 v
     both:   "$50\r\nUser d has no permissions to run the 'set' command\r\n"
   c0> ACL DRYRUN nouser GET a
     both:   "-ERR User 'nouser' not found\r\n"
   c0> ACL WHOAMI
     both:   "$7\r\ndefault\r\n"
!! c0> ACL CAT  [sorted]
     redis:  "*21\r\n$8\r\nkeyspace\r\n$4\r\nread\r\n$5\r\nwrite\r\n$3\r\nset\r\n$9\r\nsortedset\r\n$4\r\nlist\r\n$4\r\nhash\r\n$6\r\nstring\r\n$6\r\nbitmap\r\n$11\r\nhyperloglog\r\n$3\r\ngeo\r\n$6\r\nstream\r\n$6\r\npubsub\r\n$5\r\nadmin\r\n$4\r\nfast\r\n$4\r\nslow\r\n$8\r\nblocking\r\n$9\r\ndangerous\r\n$10\r\nconnection\r\n$11\r\ntransaction\r\n$9\r\nscripting\r\n"
     cstorm: "*29\r\n$8\r\nkeyspace\r\n$4\r\nread\r\n$5\r\nwrite\r\n$3\r\nset\r\n$9\r\nsortedset\r\n$4\r\nlist\r\n$4\r\nhash\r\n$6\r\nstring\r\n$6\r\nbitmap\r\n$11\r\nhyperloglog\r\n$3\r\ngeo\r\n$6\r\nstream\r\n$6\r\npubsub\r\n$5\r\nadmin\r\n$4\r\nfast\r\n$4\r\nslow\r\n$8\r\nblocking\r\n$9\r\ndangerous\r\n$10\r\nconnection\r\n$11\r\ntransaction\r\n$9\r\nscripting\r\n$5\r\nbloom\r\n$6\r\ncuckoo\r\n$3\r\ncms\r\n$4\r\ntopk\r\n$7\r\ntdigest\r\n$6\r\nsearch\r\n$10\r\ntimeseries\r\n$4\r\njson\r\n"
   c0> ACL CAT hyperloglog  [sorted]
     both:   "*5\r\n$7\r\npfdebug\r\n$7\r\npfmerge\r\n$5\r\npfadd\r\n$7\r\npfcount\r\n$10\r\npfselftest\r\n"
   c0> ACL CAT nosuchcat
     both:   "-ERR Unknown category 'nosuchcat'\r\n"
   c0> ACL DELUSER d
     both:   ":1\r\n"
```

### command/count-and-list-filters

**intentional difference**: COMMAND LIST also lists XACKDEL (and COMMAND COUNT counts it, with XDELEX, DELEX, DIGEST, MSETEX, INCREX, LMOVEM, BLMOVEM, SUNIONCARD, SDIFFCARD, XNACK, XCFGSET, XIDMPRECORD, HIMPORT, BACKUP, TRIMSLOTS and the 18 AR* array commands), commands of Redis 8.2 to 8.10 CacheStorm implements; Redis 7.4 does not have them

```
   c0> COMMAND COUNT  [type only]
     both:   ":250\r\n"
!! c0> COMMAND LIST FILTERBY PATTERN xa*  [sorted]
     redis:  "*3\r\n$10\r\nxautoclaim\r\n$4\r\nxack\r\n$4\r\nxadd\r\n"
     cstorm: "*4\r\n$4\r\nxack\r\n$7\r\nxackdel\r\n$4\r\nxadd\r\n$10\r\nxautoclaim\r\n"
   c0> COMMAND LIST FILTERBY ACLCAT hyperloglog  [sorted]
     both:   "*5\r\n$7\r\npfdebug\r\n$7\r\npfmerge\r\n$5\r\npfadd\r\n$7\r\npfcount\r\n$10\r\npfselftest\r\n"
   c0> COMMAND DOCS get
     both:   "*2\r\n$3\r\nget\r\n*10\r\n$7\r\nsummary\r\n$34\r\nReturns the string value of a key.\r\n$5\r\nsince\r\n$5\r\n1.0.0\r\n$5\r\ngroup\r\n$6\r\nstring\r\n$10\r\ncomplexity\r\n$4\r\nO(1)\r\n$9\r\narguments\r\n*1\r\n*8\r\n$4\r\nname\r\n$3\r\nkey\r\n$4\r\ntype\r\n$3\r\nkey\r\n$12\r\ndisplay_text\r\n$3\r\nkey\r\n$14\r\nkey_spec_index\r\n:0\r\n"
```

### command/getkeys-forms

**intentional difference**: COMMAND DOCS SET lists the IFEQ, IFNE, IFDEQ and IFDNE conditions and the 8.4.0 history entry Redis 8.4 added, which CacheStorm implements; Redis 7.4 does not have them

```
   c0> COMMAND GETKEYS NOSUCH a
     both:   "-ERR Invalid command specified\r\n"
   c0> COMMAND GETKEYS GET
     both:   "-ERR Invalid number of arguments specified for command\r\n"
   c0> COMMAND GETKEYS
     both:   "-ERR wrong number of arguments for 'command|getkeys' command\r\n"
   c0> COMMAND GETKEYS CONFIG GET x
     both:   "-ERR The command has no key arguments\r\n"
   c0> COMMAND GETKEYS ZUNION 0
     both:   "-ERR Invalid number of arguments specified for command\r\n"
   c0> COMMAND GETKEYS EVAL x 0
     both:   "*0\r\n"
   c0> COMMAND GETKEYSANDFLAGS SET a b GET
     both:   "*1\r\n*2\r\n$1\r\na\r\n*3\r\n+RW\r\n+access\r\n+update\r\n"
   c0> COMMAND GETKEYSANDFLAGS SET a b
     both:   "*1\r\n*2\r\n$1\r\na\r\n*2\r\n+OW\r\n+update\r\n"
   c0> COMMAND GETKEYSANDFLAGS BITFIELD a GET u8 0
     both:   "*1\r\n*2\r\n$1\r\na\r\n*2\r\n+RO\r\n+access\r\n"
   c0> COMMAND GETKEYSANDFLAGS SORT a BY w_* STORE b
     both:   "*2\r\n*2\r\n$1\r\na\r\n*2\r\n+RO\r\n+access\r\n*2\r\n$1\r\nb\r\n*2\r\n+OW\r\n+update\r\n"
   c0> COMMAND GETKEYSANDFLAGS XREAD STREAMS a b 0 0
     both:   "*2\r\n*2\r\n$1\r\na\r\n*2\r\n+RO\r\n+access\r\n*2\r\n$1\r\nb\r\n*2\r\n+RO\r\n+access\r\n"
   c0> COMMAND GETKEYSANDFLAGS ZUNIONSTORE d 2 a b
     both:   "*3\r\n*2\r\n$1\r\nd\r\n*2\r\n+OW\r\n+update\r\n*2\r\n$1\r\na\r\n*2\r\n+RO\r\n+access\r\n*2\r\n$1\r\nb\r\n*2\r\n+RO\r\n+access\r\n"
   c0> COMMAND GETKEYSANDFLAGS MIGRATE h 1 "" 0 0 KEYS a b
     both:   "*2\r\n*2\r\n$1\r\na\r\n*3\r\n+RW\r\n+access\r\n+delete\r\n*2\r\n$1\r\nb\r\n*3\r\n+RW\r\n+access\r\n+delete\r\n"
   c0> COMMAND GETKEYSANDFLAGS OBJECT ENCODING k
     both:   "*1\r\n*2\r\n$1\r\nk\r\n*1\r\n+RO\r\n"
   c0> COMMAND GETKEYSANDFLAGS GEORADIUS a 0 0 1 km STORE b
     both:   "*2\r\n*2\r\n$1\r\na\r\n*2\r\n+RO\r\n+access\r\n*2\r\n$1\r\nb\r\n*2\r\n+OW\r\n+update\r\n"
   c0> COMMAND GETKEYS LMPOP 2 a b LEFT
     both:   "*2\r\n$1\r\na\r\n$1\r\nb\r\n"
   c0> COMMAND GETKEYS SPUBLISH ch msg
     both:   "-ERR The command has no key arguments\r\n"
   c0> COMMAND INFO config|get config|nosuch nosuch|x
     both:   "*3\r\n*10\r\n$10\r\nconfig|get\r\n:-3\r\n*4\r\n+admin\r\n+noscript\r\n+loading\r\n+stale\r\n:0\r\n:0\r\n:0\r\n*3\r\n+@admin\r\n+@slow\r\n+@dangerous\r\n*0\r\n*0\r\n*0\r\n$-1\r\n$-1\r\n"
!! c0> COMMAND DOCS set client|kill nosuch
     redis:  "*4\r\n$3\r\nset\r\n*12\r\n$7\r\nsummary\r\n$90\r\nSets the string value of a key, ignoring its type. The key is created if it doesn't exist.\r\n$5\r\nsince\r\n$5\r\n1.0.0\r\n$5\r\ngroup\r\n$6\r\nstring\r\n$10\r\ncomplexity\r\n$4\r\nO(1)\r\n$7\r\nhistory\r\n*4\r\n*2\r\n$6\r\n2.6.12\r\n$44\r\nAdded the `EX`, `PX`, `NX` and `XX` options.\r\n*2\r\n$5\r\n6.0.0\r\n$27\r\nAdded the `KEEPTTL` option.\r\n*2\r\n$5\r\n6.2.0\r\n$42\r\nAdded the `GET`, `EXAT` and `PXAT` option.\r\n*2\r\n$5\r\n7.0.0\r\n$55\r\nAllowed the `NX` and `GET` options to be used together.\r\n$9\r\narguments\r\n*5\r\n*8\r\n$4\r\nna...(truncated)
     cstorm: "*4\r\n$3\r\nset\r\n*12\r\n$7\r\nsummary\r\n$90\r\nSets the string value of a key, ignoring its type. The key is created if it doesn't exist.\r\n$5\r\nsince\r\n$5\r\n1.0.0\r\n$5\r\ngroup\r\n$6\r\nstring\r\n$10\r\ncomplexity\r\n$4\r\nO(1)\r\n$7\r\nhistory\r\n*5\r\n*2\r\n$6\r\n2.6.12\r\n$44\r\nAdded the `EX`, `PX`, `NX` and `XX` options.\r\n*2\r\n$5\r\n6.0.0\r\n$27\r\nAdded the `KEEPTTL` option.\r\n*2\r\n$5\r\n6.2.0\r\n$42\r\nAdded the `GET`, `EXAT` and `PXAT` option.\r\n*2\r\n$5\r\n7.0.0\r\n$55\r\nAllowed the `NX` and `GET` options to be used together.\r\n*2\r\n$5\r\n8.4.0\r\n$47\r\nAdded 'IFEQ...(truncated)
   c0> COMMAND COUNT x
     both:   "-ERR wrong number of arguments for 'command|count' command\r\n"
   c0> COMMAND BOGUS
     both:   "-ERR unknown subcommand 'BOGUS'. Try COMMAND HELP.\r\n"
   c0> COMMAND HELP
     both:   "*21\r\n+COMMAND <subcommand> [<arg> [value] [opt] ...]. Subcommands are:\r\n+(no subcommand)\r\n+    Return details about all Redis commands.\r\n+COUNT\r\n+    Return the total number of commands in this Redis server.\r\n+LIST\r\n+    Return a list of all commands in this Redis server.\r\n+INFO [<command-name> ...]\r\n+    Return details about multiple Redis commands.\r\n+    If no command names are given, documentation details for all\r\n+    commands are returned.\r\n+DOCS [<command-name> ...]\r\n+    Return documentation details about multiple Redis commands.\r\n+    If no command names ar...(truncated)
```

### lua/redis-call-errors

**intentional difference**: compile errors carry gopher-lua's parser message ("near 'error': parse error") instead of Lua 5.1's

```
   c0> SET s v
     both:   "+OK\r\n"
   c0> EVAL "return redis.call('INCR',KEYS[1])" 1 s
     both:   "-ERR value is not an integer or out of range script: da8455f0535fd532821b3713a4eccd80fc4b8457, on @user_script:1.\r\n"
   c0> EVAL "return redis.pcall('INCR',KEYS[1])" 1 s
     both:   "-ERR value is not an integer or out of range\r\n"
   c0> EVAL "local r = redis.pcall('INCR',KEYS[1]) return type(r)" 1 s
     both:   "$5\r\ntable\r\n"
   c0> EVAL "return redis.call('NOSUCHCMD')" 0
     both:   "-ERR Unknown Redis command called from script script: ee4002f488cb0514b4fbe5979e61c3bf43204edb, on @user_script:1.\r\n"
!! c0> EVAL "syntax error here" 0
     redis:  "-ERR Error compiling script (new function): user_script:1: '=' expected near 'error'\r\n"
     cstorm: "-ERR Error compiling script (new function): user_script:1: near 'error': parse error\r\n"
   c0> EVAL error('custom') 0
     both:   "-ERR user_script:1: custom script: 999b995fd1d2b0eab82a608b66e4c8b1c62f3265, on @user_script:1.\r\n"
```

### keyspace-events/config-flags

**intentional difference**: notify-keyspace-events has Redis 8.8's array class 'a', so "g$lshzxetd" no longer covers every class and CONFIG GET lists the letters instead of A, as Redis 8.8 does

```
   c0> CONFIG SET notify-keyspace-events KEA
     both:   "+OK\r\n"
   c0> CONFIG GET notify-keyspace-events
     both:   "*2\r\n$22\r\nnotify-keyspace-events\r\n$3\r\nAKE\r\n"
   c0> CONFIG SET notify-keyspace-events KEAnm
     both:   "+OK\r\n"
   c0> CONFIG GET notify-keyspace-events
     both:   "*2\r\n$22\r\nnotify-keyspace-events\r\n$4\r\nAKEm\r\n"
   c0> CONFIG SET notify-keyspace-events g$lshzxetd
     both:   "+OK\r\n"
!! c0> CONFIG GET notify-keyspace-events
     redis:  "*2\r\n$22\r\nnotify-keyspace-events\r\n$1\r\nA\r\n"
     cstorm: "*2\r\n$22\r\nnotify-keyspace-events\r\n$10\r\ng$lshzxetd\r\n"
   c0> CONFIG SET notify-keyspace-events Elg$K
     both:   "+OK\r\n"
   c0> CONFIG GET notify-keyspace-events
     both:   "*2\r\n$22\r\nnotify-keyspace-events\r\n$5\r\ng$lKE\r\n"
   c0> CONFIG SET notify-keyspace-events Ex
     both:   "+OK\r\n"
   c0> CONFIG GET notify-keyspace-events
     both:   "*2\r\n$22\r\nnotify-keyspace-events\r\n$2\r\nxE\r\n"
   c0> CONFIG SET notify-keyspace-events KEQ
     both:   "-ERR CONFIG SET failed (possibly related to argument 'notify-keyspace-events') - Invalid event class character. Use 'Ag$lshzxeKEtmdn'.\r\n"
   c0> CONFIG GET notify-keyspace-events
     both:   "*2\r\n$22\r\nnotify-keyspace-events\r\n$2\r\nxE\r\n"
   c0> CONFIG SET notify-keyspace-events nmK
     both:   "+OK\r\n"
   c0> CONFIG GET notify-keyspace-events
     both:   "*2\r\n$22\r\nnotify-keyspace-events\r\n$3\r\nnKm\r\n"
   c0> CONFIG SET notify-keyspace-events ""
     both:   "+OK\r\n"
   c0> CONFIG GET notify-keyspace-events
     both:   "*2\r\n$22\r\nnotify-keyspace-events\r\n$0\r\n\r\n"
```

## Passing cases

`strings/set-get`, `strings/set-nx-xx`, `strings/set-get-option`, `strings/set-nx-get`, `strings/set-ex`, `strings/set-px`, `strings/set-keepttl`, `strings/set-ex-invalid`, `strings/set-syntax-errors`, `strings/set-pxat-max`, `strings/getex-pxat-max`, `strings/set-exat-pxat-past`, `strings/set-ex-overflow`, `strings/getex`, `strings/getex-errors`, `strings/getex-exat-past`, `strings/getdel`, `strings/incr-basic`, `strings/incr-not-int`, `strings/incr-overflow`, `strings/incrbyfloat`, `strings/incrbyfloat-errors`, `strings/incrbyfloat-format`, `strings/append-strlen`, `strings/setrange`, `strings/getrange`, `strings/mset-mget`, `strings/msetnx`, `strings/binary-values`, `strings/utf8-values`, `strings/large-value`, `strings/setnx-setex-psetex-getset`, `strings/lcs`, `strings/mset-final-duplicate`, `strings/msetnx-final-duplicate`, `strings/mset-clears-ttl`, `strings/msetnx-conflict-watch`, `strings/mset-destination-watch`, `strings/mset-replaces-collection`, `keys/del-multi`, `keys/exists-multi`, `keys/expire-basic`, `keys/expire-nx-xx`, `keys/expire-gt-lt`, `keys/expire-option-conflicts`, `keys/expire-negative-deletes`, `keys/expire-actually-expires`, `keys/ttl-missing-nottl`, `keys/expiretime-rounding`, `keys/expiretime-max`, `keys/expire-gt-lt-far-future`, `keys/expire-relative-far-future`, `keys/expiretime`, `keys/rename`, `keys/rename-keeps-ttl`, `keys/renamenx`, `keys/type`, `keys/scan-match-count`, `keys/scan-type`, `keys/scan-errors`, `keys/scan-empty`, `keys/keys-pattern`, `keys/randomkey`, `keys/copy`, `keys/touch`, `keys/del-then-type`, `restore/bitmap-setbit-keeps-expiry`, `restore/bitmap-bitfield-keeps-expiry`, `restore/relative-ttl`, `restore/permanent`, `restore/absolute-max`, `restore/absolute-past`, `restore/replace`, `restore/replace-past`, `restore/busy`, `restore/checksum-preserves`, `restore/negative-ttl`, `restore/invalid-integer`, `restore/invalid-options`, `restore/invalid-idletime`, `restore/invalid-freq`, `restore/idletime`, `restore/freq-ignored-without-lfu`, `restore/duplicate-options`, `bitmap/signed-set-extreme-saturation`, `bitmap/setbit-validation`, `bitmap/read-missing`, `bitmap/invalid-suffix-atomic`, `bitmap/encoding-validation`, `bitmap/scaled-offset`, `bitmap/offset-validation`, `bitmap/read-last-offset`, `bitmap/overflow-set`, `bitmap/overflow-controls`, `bitmap/signed64-overflow`, `bitmap/readonly-write-rejected`, `bitmap/empty-and-wrongtype`, `bitmap/mixed-overlap`, `bitmap/no-op-watch`, `bitmap/read-only-watch`, `hll/empty`, `hll/add-duplicates`, `hll/binary-elements`, `hll/union`, `hll/merge-destination`, `hll/merge-empty`, `hll/ttl`, `hll/wrongtype`, `hll/invalid-source-atomic`, `hll/cached-header`, `hll/count-cache-watch`, `hll/duplicate-watch`, `geoadd/invalid-batch-atomic`, `geoadd/invalid-first-missing`, `geoadd/invalid-numeric-atomic`, `geoadd/nan-infinity`, `geoadd/conflicting-options`, `geoadd/xx-still-validates`, `geoadd/no-op-watch`, `geoadd/invalid-watch`, `geo/readonly-radius`, `geo/radius-read-watch`, `geo/missing-center-preserves`, `geo/invalid-geometry-preserves`, `geo/invalid-options-preserve`, `geo/unit-error`, `geo/zset-interoperability`, `geo/read-coordinates-hash-distance`, `geo/raw-score-import`, `geo/quantized-watch`, `geo/boundary-coordinates`, `geo/wrongtype-readers`, `geo/wrongtype-store-preserves`, `geo/radius-with-options`, `geo/search-with-options`, `geo/box-two-dimensions`, `geo/store-scores`, `geo/storedist-scores`, `transfer/copy-source-watch`, `transfer/copy-destination-watch`, `transfer/copy-noop-watch`, `transfer/rename-self-watch`, `transfer/renamenx-noop-watch`, `transfer/rename-overwrite-watch`, `transfer/empty-destination`, `transfer/copy-replace-ttl`, `transfer/copy-hash-independent`, `transfer/copy-stream-pending-independent`, `db/move-noop-source-watch`, `db/copy-transfer-cross-db`, `hashes/hset-hget`, `hashes/hgetall-keys-vals`, `hashes/hdel-removes-key`, `hashes/hexists-hstrlen`, `hashes/hsetnx`, `hashes/hincrby`, `hashes/hincrbyfloat`, `hashes/hmset`, `hashes/hset-odd-args`, `hashes/hrandfield`, `hashes/hscan`, `hashes/hsetnx-conflict-watch`, `hashes/hincrby-invalid-field-watch`, `hashes/hincrbyfloat-invalid-field-watch`, `hashes/hincrby-overflow-watch`, `hashes/hash-invalid-input-missing-watch`, `hashes/hset-duplicate-final-fields`, `hashes/hash-growth-preserves-ttl`, `hash-ttl/hexpire-basic`, `hash-ttl/units`, `hash-ttl/conditions`, `hash-ttl/equal-and-duplicate`, `hash-ttl/past-deletes`, `hash-ttl/past-condition-first`, `hash-ttl/missing-key`, `hash-ttl/wrongtype`, `hash-ttl/numfields-errors`, `hash-ttl/fields-keyword-errors`, `hash-ttl/time-errors`, `hash-ttl/arity`, `hash-ttl/hpersist`, `hash-ttl/writes-and-ttl`, `hash-ttl/lazy-expiry-reads`, `hash-ttl/last-field-expiry-deletes-key`, `hash-ttl/expired-field-writes`, `hash-ttl/object-encoding`, `hash-ttl/restore-redis-payloads`, `hash-ttl/dump-restore-roundtrip`, `hash-ttl/rename-copy-keep-field-ttls`, `hash-ttl/key-ttl-independent`, `hash-ttl/multi-exec`, `hash-ttl/watch-hexpire-aborts`, `hash-ttl/watch-noop-keeps-transaction`, `hash-ttl/watch-hpersist-aborts`, `hash-ttl/lua-hexpire`, `hash-ttl/hrandfield-after-partial-expiry`, `lists/push-range`, `lists/lindex-lset`, `lists/linsert`, `lists/lrem`, `lists/ltrim`, `lists/pop-count`, `lists/pushx`, `lists/lmove-rpoplpush`, `lists/lpos`, `lists/lmpop`, `lists/lmpop-errors`, `lists/big-list-order`, `blocking/blpop-present`, `blocking/blpop-absent-fractional`, `blocking/blpop-timeout0-wakeup`, `blocking/brpop-timeout0-wakeup`, `blocking/blpop-fractional-wakeup`, `blocking/blpop-timeout-errors`, `blocking/blpop-wrongtype`, `blocking/blmove-present`, `blocking/blmove-absent`, `blocking/blmove-timeout0-wakeup`, `blocking/brpoplpush-wakeup`, `blocking/blmpop-present`, `blocking/blmpop-absent`, `blocking/blmpop-timeout0-wakeup`, `blocking/bzpopmin-present`, `blocking/bzpopmin-absent`, `blocking/bzpopmin-timeout0-wakeup`, `blocking/bzmpop-wakeup`, `blocking/blpop-inside-multi`, `blocking/blpop-fifo-two-waiters`, `blocking/blpop-wakeup-after-multi-push`, `blocking/blpop-lmove-target`, `sets/sadd-smembers`, `sets/srem-removes-key`, `sets/spop-srandmember`, `sets/spop-all`, `sets/set-algebra`, `sets/set-store`, `sets/smove`, `sets/sscan`, `sets/intset-members`, `sets/sadd-noop-watch`, `sets/smove-absent-watch`, `sets/smove-self-watch`, `sets/smove-duplicate-dest-watch`, `sets/smove-source-watch`, `sets/smove-existing-dest-ttl`, `sets/smove-new-dest-ttl`, `sets/sadd-preserves-ttl`, `sets/set-existing-empty-name`, `sets/smove-wrongtype-watch`, `sets/SUNIONSTORE-read-source-watch`, `sets/SUNIONSTORE-empty-noop-watch`, `sets/SUNIONSTORE-error-watch`, `sets/SINTERSTORE-read-source-watch`, `sets/SINTERSTORE-empty-noop-watch`, `sets/SINTERSTORE-error-watch`, `sets/SDIFFSTORE-read-source-watch`, `sets/SDIFFSTORE-empty-noop-watch`, `sets/SDIFFSTORE-error-watch`, `strings/mget-duplicate-key`, `sets/plain-numeric-key-list`, `zsets/zadd-basic`, `zsets/zadd-nx-xx`, `zsets/zadd-gt-lt-ch`, `zsets/zadd-incr`, `zsets/zadd-errors`, `zsets/zadd-inf-scores`, `zsets/score-format`, `zsets/zrange-byscore`, `zsets/zrange-bylex`, `zsets/zrange-rev-index`, `zsets/zrangebyscore-legacy`, `zsets/zrank`, `zsets/zrangestore`, `zsets/zunion-zinter`, `zsets/zunion-with-set`, `zsets/zunionstore`, `zsets/zmpop`, `zsets/zpopmin-max`, `zsets/zrem-ranges`, `zsets/zincrby`, `zsets/zrandmember`, `zsets/zscan`, `zsets/same-score-lex-order`, `streams/xadd-explicit`, `streams/xadd-zero-id`, `streams/xadd-auto`, `streams/xrange-count-rev`, `streams/xread`, `streams/xread-block-wakeup`, `streams/xgroup-create`, `streams/xreadgroup-ack-pending`, `streams/xreadgroup-errors`, `streams/xpending-extended-shape`, `streams/xdel-xtrim`, `streams/xadd-smaller-after-delete`, `streams/xadd-trim-options`, `streams/xadd-partial-ids`, `streams/xadd-field-order`, `streams/xtrim-approx-nodes`, `streams/xtrim-approx-node-bytes`, `streams/xadd-approx-after-xdel`, `streams/xtrim-errors`, `streams/xrange-exclusive`, `streams/xread-errors`, `streams/xread-plus-empty`, `streams/xread-block-in-multi`, `streams/xreadgroup-block-wakeup`, `streams/xreadgroup-block-group-destroyed`, `streams/xreadgroup-block-key-deleted`, `streams/xread-block-new-key`, `streams/xreadgroup-block-count`, `streams/xreadgroup-block-timeout`, `streams/xreadgroup-id-errors`, `streams/xreadgroup-history`, `streams/xreadgroup-multi-stream`, `streams/xreadgroup-setid-redeliver`, `streams/xpending-forms`, `streams/xclaim-forms`, `streams/xautoclaim-forms`, `streams/xgroup-subcommands`, `streams/xgroup-delconsumer-pending`, `streams/xinfo-shapes`, `streams/xinfo-after-deletes`, `streams/xinfo-lag`, `streams/xinfo-lag-tombstone`, `streams/xsetid-forms`, `streams/xack-xdel-forms`, `streams/wrongtype-family`, `transactions/multi-exec`, `transactions/multi-discard`, `transactions/execabort-unknown-command`, `transactions/execabort-wrong-arity`, `transactions/runtime-error-inside-exec`, `transactions/empty-exec`, `transactions/nested-multi`, `transactions/watch-inside-multi`, `transactions/watch-touched-aborts`, `transactions/watch-untouched-commits`, `transactions/watch-missing-key-created`, `transactions/watch-deleted-key`, `transactions/watch-expire-touch`, `transactions/unwatch`, `transactions/watch-flushall`, `transactions/watch-other-db-no-abort`, `transactions/exec-discard-without-multi`, `transactions/multi-queued-replies`, `transactions/multi-select`, `transactions/exec-after-execabort-resets`, `transactions/multi-incr-wrongtype-then-ok`, `wrongtype/GET`, `wrongtype/INCR`, `wrongtype/APPEND`, `wrongtype/STRLEN`, `wrongtype/GETRANGE`, `wrongtype/SETRANGE`, `wrongtype/INCRBYFLOAT`, `wrongtype/GETDEL`, `wrongtype/DECRBY`, `wrongtype/GETSET`, `wrongtype/LPUSH`, `wrongtype/RPUSH`, `wrongtype/LRANGE`, `wrongtype/LLEN`, `wrongtype/LPOP`, `wrongtype/LINDEX`, `wrongtype/LSET`, `wrongtype/LPOS`, `wrongtype/LMOVE`, `wrongtype/LMPOP`, `wrongtype/HSET`, `wrongtype/HGET`, `wrongtype/HGETALL`, `wrongtype/HDEL`, `wrongtype/HLEN`, `wrongtype/HINCRBY`, `wrongtype/HKEYS`, `wrongtype/SADD`, `wrongtype/SMEMBERS`, `wrongtype/SISMEMBER`, `wrongtype/SCARD`, `wrongtype/SINTER`, `wrongtype/SREM`, `wrongtype/ZADD`, `wrongtype/ZRANGE`, `wrongtype/ZSCORE`, `wrongtype/ZCARD`, `wrongtype/ZINCRBY`, `wrongtype/ZUNION`, `wrongtype/ZRANK`, `wrongtype/XADD`, `wrongtype/XRANGE`, `wrongtype/XLEN`, `wrongtype/SUNIONSTORE`, `wrongtype/PFADD`, `wrongtype/SETBIT`, `wrongtype/BITCOUNT`, `arity/GET`, `arity/GET-2`, `arity/SET-1`, `arity/DEL`, `arity/EXISTS`, `arity/INCR`, `arity/INCRBY-1`, `arity/APPEND-1`, `arity/MGET`, `arity/MSET`, `arity/STRLEN`, `arity/GETRANGE-2`, `arity/SETEX-2`, `arity/EXPIRE-1`, `arity/TTL`, `arity/RENAME-1`, `arity/TYPE`, `arity/KEYS`, `arity/SCAN`, `arity/HSET-1`, `arity/HGET-1`, `arity/HGETALL`, `arity/HDEL-1`, `arity/HMGET-1`, `arity/HINCRBY-2`, `arity/LPUSH-1`, `arity/RPUSH`, `arity/LRANGE-2`, `arity/LPOP`, `arity/LINDEX-1`, `arity/LREM-2`, `arity/BLPOP-1`, `arity/SADD-1`, `arity/SMEMBERS`, `arity/SISMEMBER-1`, `arity/SINTER`, `arity/ZADD-1`, `arity/ZRANGE-2`, `arity/ZSCORE-1`, `arity/ZINCRBY-2`, `arity/XADD-1`, `arity/XRANGE-2`, `arity/XLEN`, `arity/PUBLISH-1`, `arity/SUBSCRIBE`, `arity/SELECT`, `arity/ECHO`, `arity/ECHO-2`, `arity/PING-2`, `arity/EVAL`, `arity/EVAL-1`, `arity/MULTI-1`, `arity/WATCH`, `arity/OBJECT`, `arity/DBSIZE-1`, `pubsub/subscribe-publish-deliver`, `pubsub/psubscribe-deliver`, `pubsub/subscribe-multi-channel`, `pubsub/publish-no-subscribers`, `pubsub/two-subscribers-count`, `pubsub/pattern-and-channel-both`, `pubsub/unsubscribe`, `pubsub/unsubscribe-not-subscribed`, `pubsub/ping-in-subscribe-mode`, `pubsub/pubsub-introspection`, `pubsub/binary-message`, `pubsub/publish-from-multi`, `pubsub/publish-from-lua`, `pubsub/ssubscribe-spublish`, `db/select-dbsize`, `db/select-errors`, `db/move`, `db/flushdb-only-current`, `db/swapdb`, `db/select-per-connection`, `db/copy-db`, `misc/ping-echo`, `misc/object-encoding-strings`, `misc/object-encoding-collections`, `misc/object-misc`, `misc/unknown-command`, `misc/case-insensitive`, `misc/dbsize-flushall`, `misc/inline-commands`, `misc/pipelined-burst`, `misc/client-setname`, `misc/hello-2`, `sort/numeric-alpha-limit`, `sort/by-get-store`, `sort/hash-by-and-sets`, `sort/errors`, `bits/bitcount-bitpos`, `bits/bitop`, `strings/lcs-forms`, `strings/setrange-getrange-edges`, `strings/incr-decr-edges`, `function/load-call-list`, `function/load-errors`, `scripting/script-commands`, `scripting/eval-ro-and-flags`, `keys/expire-collections-and-persist`, `keys/object-freq-and-encoding-growth`, `keys/dump-restore-collections`, `keys/type-and-unlink`, `encoding/thresholds`, `encoding/long-element-conversion`, `lists/lpos-and-linsert-edges`, `zsets/range-and-store-edges`, `sets/intercard-and-misc-errors`, `keys/scan-type-and-patterns`, `strings/getex-persist-and-set-get-wrongtype`, `acl/setuser-getuser-list`, `command/info-and-getkeys`, `client/info-reply-no-evict`, `config/get-set-errors`, `config/set-error-texts`, `acl/rule-rendering`, `acl/dryrun-channels-subcommands`, `admin/container-subcommand-errors`, `acl/selectors-and-key-permissions`, `lua/return-types`, `lua/redis-call-types`, `lua/keys-argv`, `lua/evalsha`, `lua/evalsha-loaded`, `lua/lua-writes-state`, `lua/lua-float-int-conv`, `lua/lua-select-isolated`, `keyspace-events/off`, `keyspace-events/classes-filter`, `keyspace-events/keyevent-only`, `keyspace-events/strings`, `keyspace-events/set-expired-instant`, `keyspace-events/bitmaps-hll`, `keyspace-events/generic`, `keyspace-events/rename`, `keyspace-events/copy-move`, `keyspace-events/swapdb-flush`, `keyspace-events/dump-restore`, `keyspace-events/sort-store`, `keyspace-events/lists`, `keyspace-events/list-moves`, `keyspace-events/blocked-pop-served`, `keyspace-events/hashes`, `keyspace-events/hash-field-ttl`, `keyspace-events/sets`, `keyspace-events/sorted-sets`, `keyspace-events/sorted-set-stores`, `keyspace-events/geo`, `keyspace-events/streams`, `keyspace-events/expiry-lazy`, `keyspace-events/expiry-active`, `keyspace-events/keymiss`, `keyspace-events/new-keys`, `keyspace-events/multi-exec`, `keyspace-events/lua`, `keyspace-events/other-db`, `keyspace-events/wrongtype-no-event`, `keyspace-events/eviction`

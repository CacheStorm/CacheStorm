# CacheStorm Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned
- Cloud-native commands (object storage, queues, topics)
- GraphQL API
- WebSocket support
- GraphQL subscriptions

## [0.1.25] - 2026-02-21

### Added - Redis Compatibility Improvements (8 new commands - Total: 1,606)

**Redis 7 Sharded Pub/Sub**
- `SSUBSCRIBE` - Subscribe to sharded channel
- `SUNSUBSCRIBE` - Unsubscribe from sharded channel
- `SPUBLISH` - Publish to sharded channel

**Key Commands**
- `EXPIRETIME` - Get expiration Unix timestamp
- `PEXPIRETIME` - Get expiration Unix millisecond timestamp
- `MOVE` - Move key between databases

**Server Commands**
- `WAITAOF` - Wait for AOF sync
- `BLMPOP` - Blocking multiple list pop

### Redis Compatibility
- **~99% Redis Compatible** - All core Redis commands implemented
- Full support for: Strings, Hashes, Lists, Sets, Sorted Sets, Bitmaps, HyperLogLog, Geo, Streams
- Complete Pub/Sub including sharded (Redis 7)
- Full transaction support (MULTI/EXEC/DISCARD/WATCH)
- Lua scripting (EVAL/EVALSHA/SCRIPT)
- Cluster and replication commands
- ACL support

## [0.1.24] - 2026-02-21

### Added - Machine Learning Commands (80 new commands - Total: 1,598)

**Model Commands**
- `MODEL.CREATE` - Create ML model
- `MODEL.TRAIN` - Train model
- `MODEL.PREDICT` - Make predictions
- `MODEL.DELETE` - Delete model
- `MODEL.LIST` - List models
- `MODEL.STATUS` - Get model status

**Feature Commands**
- `FEATURE.SET` - Set feature value
- `FEATURE.GET` - Get feature value
- `FEATURE.DEL` - Delete feature
- `FEATURE.INCR` - Increment feature
- `FEATURE.NORMALIZE` - Normalize features
- `FEATURE.VECTOR` - Get feature vector

**Embedding Commands**
- `EMBEDDING.CREATE` - Create embedding
- `EMBEDDING.GET` - Get embedding
- `EMBEDDING.SEARCH` - Search embeddings
- `EMBEDDING.SIMILAR` - Find similar embeddings
- `EMBEDDING.DELETE` - Delete embedding

**Tensor Commands**
- `TENSOR.CREATE` - Create tensor
- `TENSOR.GET` - Get tensor
- `TENSOR.ADD` - Add tensors
- `TENSOR.MATMUL` - Matrix multiplication
- `TENSOR.RESHAPE` - Reshape tensor
- `TENSOR.DELETE` - Delete tensor

**Classifier Commands**
- `CLASSIFIER.CREATE` - Create classifier
- `CLASSIFIER.TRAIN` - Train classifier
- `CLASSIFIER.PREDICT` - Predict class
- `CLASSIFIER.DELETE` - Delete classifier

**Regressor Commands**
- `REGRESSOR.CREATE` - Create regressor
- `REGRESSOR.TRAIN` - Train regressor
- `REGRESSOR.PREDICT` - Predict value
- `REGRESSOR.DELETE` - Delete regressor

**Clustering Commands**
- `CLUSTER.CREATE` - Create cluster model
- `CLUSTER.FIT` - Fit clusters
- `CLUSTER.PREDICT` - Predict cluster
- `CLUSTER.CENTROIDS` - Get centroids
- `CLUSTER.DELETE` - Delete cluster model

**Anomaly Detection Commands**
- `ANOMALY.CREATE` - Create anomaly detector
- `ANOMALY.DETECT` - Detect anomalies
- `ANOMALY.LEARN` - Learn normal behavior
- `ANOMALY.DELETE` - Delete detector

**Sentiment Analysis Commands**
- `SENTIMENT.ANALYZE` - Analyze sentiment
- `SENTIMENT.BATCH` - Batch sentiment analysis

**NLP Commands**
- `NLP.TOKENIZE` - Tokenize text
- `NLP.ENTITIES` - Extract entities
- `NLP.KEYWORDS` - Extract keywords
- `NLP.SUMMARIZE` - Summarize text

**Similarity Commands**
- `SIMILARITY.COSINE` - Cosine similarity
- `SIMILARITY.EUCLIDEAN` - Euclidean distance
- `SIMILARITY.JACCARD` - Jaccard similarity
- `SIMILARITY.DOTPRODUCT` - Dot product

**Dataset Commands**
- `DATASET.CREATE` - Create dataset
- `DATASET.ADD` - Add data
- `DATASET.GET` - Get dataset
- `DATASET.SPLIT` - Split dataset
- `DATASET.DELETE` - Delete dataset

**ML Experiment Commands**
- `MLXPERIMENT.CREATE` - Create experiment
- `MLXPERIMENT.LOG` - Log metrics
- `MLXPERIMENT.METRICS` - Get metrics
- `MLXPERIMENT.COMPARE` - Compare experiments
- `MLXPERIMENT.DELETE` - Delete experiment

**ML Pipeline Commands**
- `PIPELINEML.CREATE` - Create ML pipeline
- `PIPELINEML.ADD` - Add step
- `PIPELINEML.RUN` - Run pipeline
- `PIPELINEML.DELETE` - Delete pipeline

**Hyperparameter Commands**
- `HYPERPARAM.SET` - Set hyperparameter
- `HYPERPARAM.GET` - Get hyperparameter
- `HYPERPARAM.SEARCH` - Search hyperparameters
- `HYPERPARAM.DELETE` - Delete hyperparameters

**Evaluator Commands**
- `EVALUATOR.CREATE` - Create evaluator
- `EVALUATOR.RUN` - Run evaluation
- `EVALUATOR.METRICS` - Get metrics
- `EVALUATOR.DELETE` - Delete evaluator

**Recommendation Commands**
- `RECOMMEND.CREATE` - Create recommender
- `RECOMMEND.TRAIN` - Train recommender
- `RECOMMEND.GET` - Get recommendations
- `RECOMMEND.DELETE` - Delete recommender

**Time Series Forecast Commands**
- `TIMEFORECAST.CREATE` - Create forecaster
- `TIMEFORECAST.TRAIN` - Train forecaster
- `TIMEFORECAST.PREDICT` - Predict values
- `TIMEFORECAST.DELETE` - Delete forecaster

## [0.1.23] - 2026-02-21

### Added - Resilience Commands (138 new commands - Total: 1,458)

**Circuit Breaker Extended Commands**
- `CIRCUITX.CREATE` - Create a circuit breaker
- `CIRCUITX.OPEN` - Open circuit breaker
- `CIRCUITX.CLOSE` - Close circuit breaker  
- `CIRCUITX.HALFOPEN` - Set to half-open state
- `CIRCUITX.STATUS` - Get circuit breaker status
- `CIRCUITX.METRICS` - Get circuit breaker metrics
- `CIRCUITX.RESET` - Reset circuit breaker
- `CIRCUITX.DELETE` - Delete circuit breaker

**Rate Limiter Commands**
- `RATELIMITER.CREATE` - Create rate limiter
- `RATELIMITER.TRY` - Try to acquire permit
- `RATELIMITER.WAIT` - Wait for permit
- `RATELIMITER.RESET` - Reset rate limiter
- `RATELIMITER.STATUS` - Get rate limiter status
- `RATELIMITER.DELETE` - Delete rate limiter

**Retry Commands**
- `RETRY.CREATE` - Create retry policy
- `RETRY.EXECUTE` - Execute with retry
- `RETRY.STATUS` - Get retry status
- `RETRY.DELETE` - Delete retry policy

**Timeout Commands**
- `TIMEOUT.CREATE` - Create timeout handler
- `TIMEOUT.EXECUTE` - Execute with timeout
- `TIMEOUT.DELETE` - Delete timeout handler

**Bulkhead Commands**
- `BULKHEAD.CREATE` - Create bulkhead
- `BULKHEAD.ACQUIRE` - Acquire permit
- `BULKHEAD.RELEASE` - Release permit
- `BULKHEAD.STATUS` - Get bulkhead status
- `BULKHEAD.DELETE` - Delete bulkhead

**Fallback Commands**
- `FALLBACK.CREATE` - Create fallback handler
- `FALLBACK.EXECUTE` - Execute fallback
- `FALLBACK.DELETE` - Delete fallback

**Observability Commands**
- `OBSERVABILITY.TRACE` - Record trace
- `OBSERVABILITY.METRIC` - Record metric
- `OBSERVABILITY.LOG` - Log message
- `OBSERVABILITY.SPAN` - Create span

**Telemetry Commands**
- `TELEMETRY.RECORD` - Record telemetry point
- `TELEMETRY.QUERY` - Query telemetry data
- `TELEMETRY.EXPORT` - Export telemetry

**Diagnostic Commands**
- `DIAGNOSTIC.RUN` - Run diagnostic
- `DIAGNOSTIC.RESULT` - Get diagnostic result
- `DIAGNOSTIC.LIST` - List diagnostics

**Profile Extended Commands**
- `PROFILE.START` - Start profiling
- `PROFILE.STOP` - Stop profiling
- `PROFILE.RESULT` - Get profile result
- `PROFILEX.LIST` - List profiles

**Heap Commands**
- `HEAP.STATS` - Get heap statistics
- `HEAP.DUMP` - Dump heap
- `HEAP.GC` - Run garbage collection

**Memory Extended Commands**
- `MEMORYX.ALLOC` - Allocate memory
- `MEMORYX.FREE` - Free memory
- `MEMORYX.STATS` - Memory statistics
- `MEMORYX.TRACK` - Track memory allocation

**Connection Pool Commands**
- `CONPOOL.CREATE` - Create connection pool
- `CONPOOL.GET` - Get connection from pool
- `CONPOOL.RETURN` - Return connection to pool
- `CONPOOL.STATUS` - Get pool status
- `CONPOOL.DELETE` - Delete connection pool

**Batch Extended Commands**
- `BATCHX.CREATE` - Create batch
- `BATCHX.ADD` - Add item to batch
- `BATCHX.EXECUTE` - Execute batch
- `BATCHX.STATUS` - Get batch status
- `BATCHX.DELETE` - Delete batch

**Pipeline Extended Commands**
- `PIPELINEX.START` - Start pipeline
- `PIPELINEX.ADD` - Add command to pipeline
- `PIPELINEX.EXECUTE` - Execute pipeline
- `PIPELINEX.CANCEL` - Cancel pipeline

**Transaction Extended Commands**
- `TRANSX.BEGIN` - Begin transaction
- `TRANSX.COMMIT` - Commit transaction
- `TRANSX.ROLLBACK` - Rollback transaction
- `TRANSX.STATUS` - Get transaction status

**Lock Extended Commands**
- `LOCKX.ACQUIRE` - Acquire distributed lock
- `LOCKX.RELEASE` - Release lock
- `LOCKX.EXTEND` - Extend lock TTL
- `LOCKX.STATUS` - Get lock status

**Semaphore Extended Commands**
- `SEMAPHOREX.CREATE` - Create semaphore
- `SEMAPHOREX.ACQUIRE` - Acquire permits
- `SEMAPHOREX.RELEASE` - Release permits
- `SEMAPHOREX.STATUS` - Get semaphore status

**Async Commands**
- `ASYNC.SUBMIT` - Submit async job
- `ASYNC.STATUS` - Get job status
- `ASYNC.RESULT` - Get job result
- `ASYNC.CANCEL` - Cancel job

**Promise Commands**
- `PROMISE.CREATE` - Create promise
- `PROMISE.RESOLVE` - Resolve promise
- `PROMISE.REJECT` - Reject promise
- `PROMISE.STATUS` - Get promise status
- `PROMISE.AWAIT` - Await promise

**Future Commands**
- `FUTURE.CREATE` - Create future
- `FUTURE.COMPLETE` - Complete future
- `FUTURE.GET` - Get future value
- `FUTURE.CANCEL` - Cancel future

**Observable Commands**
- `OBSERVABLE.CREATE` - Create observable
- `OBSERVABLE.NEXT` - Emit next value
- `OBSERVABLE.COMPLETE` - Complete observable
- `OBSERVABLE.ERROR` - Emit error
- `OBSERVABLE.SUBSCRIBE` - Subscribe to observable

**Stream Processing Commands**
- `STREAMPROC.CREATE` - Create stream processor
- `STREAMPROC.PUSH` - Push to stream
- `STREAMPROC.POP` - Pop from stream
- `STREAMPROC.PEEK` - Peek stream
- `STREAMPROC.DELETE` - Delete stream processor

**Event Sourcing Commands**
- `EVENTSOURCING.APPEND` - Append event
- `EVENTSOURCING.REPLAY` - Replay events
- `EVENTSOURCING.SNAPSHOT` - Create snapshot
- `EVENTSOURCING.GET` - Get event

**Compact Commands**
- `COMPACT.MERGE` - Merge compaction
- `COMPACT.STATUS` - Get compaction status

**Backpressure Commands**
- `BACKPRESSURE.CREATE` - Create backpressure handler
- `BACKPRESSURE.CHECK` - Check backpressure
- `BACKPRESSURE.STATUS` - Get backpressure status

**Throttle Extended Commands**
- `THROTTLEX.CREATE` - Create throttle
- `THROTTLEX.CHECK` - Check throttle
- `THROTTLEX.STATUS` - Get throttle status

**Debounce Extended Commands**
- `DEBOUNCEX.CREATE` - Create debounce
- `DEBOUNCEX.CALL` - Call debounced function
- `DEBOUNCEX.CANCEL` - Cancel debounce
- `DEBOUNCEX.FLUSH` - Flush debounce

**Coalesce Commands**
- `COALESCE.CREATE` - Create coalescer
- `COALESCE.ADD` - Add value
- `COALESCE.GET` - Get coalesced value
- `COALESCE.CLEAR` - Clear coalescer

**Aggregator Commands**
- `AGGREGATOR.CREATE` - Create aggregator
- `AGGREGATOR.ADD` - Add value
- `AGGREGATOR.GET` - Get aggregated value
- `AGGREGATOR.RESET` - Reset aggregator

**Window Extended Commands**
- `WINDOWX.CREATE` - Create window
- `WINDOWX.ADD` - Add value to window
- `WINDOWX.GET` - Get window values
- `WINDOWX.AGGREGATE` - Aggregate window

**Join Extended Commands**
- `JOINX.CREATE` - Create join
- `JOINX.ADD` - Add to join
- `JOINX.GET` - Get joined results
- `JOINX.DELETE` - Delete join

**Shuffle Commands**
- `SHUFFLE.CREATE` - Create shuffle
- `SHUFFLE.ADD` - Add value
- `SHUFFLE.GET` - Get shuffled value

**Partition Extended Commands**
- `PARTITIONX.CREATE` - Create partition
- `PARTITIONX.ADD` - Add to partition
- `PARTITIONX.GET` - Get partition values
- `PARTITIONX.REBALANCE` - Rebalance partitions

## [0.1.22] - 2026-02-21

### Added - Advanced Commands 2 (175 new commands - Total: 1,393)

- Filter Commands: FILTER.CREATE, DELETE, APPLY, LIST
- Transform Commands: TRANSFORM.CREATE, DELETE, APPLY, LIST
- Enrichment Commands: ENRICH.CREATE, DELETE, APPLY, LIST
- Validator Commands: VALIDATE.CREATE, DELETE, CHECK, LIST
- Job Extended Commands: JOBX.CREATE, DELETE, RUN, STATUS, LIST
- Stage Commands: STAGE.CREATE, DELETE, NEXT, PREV, LIST
- Context Commands: CONTEXT.CREATE, DELETE, SET, GET, LIST
- Rule Commands: RULE.CREATE, DELETE, EVAL, LIST
- Policy Commands: POLICY.CREATE, DELETE, CHECK, LIST
- Permit Commands: PERMIT.GRANT, REVOKE, CHECK, LIST
- Grant Commands: GRANT.CREATE, DELETE, CHECK, LIST
- Chain Extended Commands: CHAINX.CREATE, DELETE, EXECUTE, LIST
- Task Extended Commands: TASKX.CREATE, DELETE, RUN, LIST
- Timer Commands: TIMER.CREATE, DELETE, STATUS, LIST

## [0.1.21] - 2026-02-21

### Added - Extra Commands (138 new commands - Total: 1,218)

- Counter Extended Commands
- Pool Extended Commands
- Registry Commands
- Heartbeat Commands
- Gossip Commands
- Anti-Entropy Commands
- Quorum Commands
- Consensus Commands
- Leader Election Commands
- Membership Commands
- Failure Detector Commands
- Replication Commands
- Sharding Commands
- Vector Clock Commands

## [0.1.20] - 2026-02-21

### Added - More Commands (129 new commands - Total: 1,080)

- Ring Buffer Commands
- Trie Commands
- Interval Tree Commands
- Spatial Index Commands
- Bloom Filter Extended Commands
- Count-Min Sketch Commands
- Top-K Commands
- Similarity Commands
- Embedding Commands
- Feature Store Commands

## [0.1.19] - 2026-02-21

### Added - Extended Commands (97 new commands - Total: 951)

- Rate Limiting Commands
- Token Bucket Commands
- Sliding Window Commands
- Adaptive Rate Limit Commands
- Circuit Breaker Commands
- Health Check Commands
- Dependency Commands
- Orchestration Commands

## [0.1.18] - 2026-02-21

### Added - Integration Commands (71 new commands - Total: 854)

- HTTP Client Commands
- gRPC Client Commands
- WebSocket Commands
- Message Queue Commands
- Event Bridge Commands

## [0.1.0] - 2026-02-19

### Added - Initial Release (289 commands)

**Core Redis Commands**
- String Commands: SET, GET, INCR, DECR, APPEND, MSET, MGET, etc.
- Hash Commands: HSET, HGET, HINCRBY, HMSET, HGETALL, etc.
- List Commands: LPUSH, RPUSH, LPOP, RPOP, LRANGE, etc.
- Set Commands: SADD, SREM, SINTER, SUNION, etc.
- Sorted Set Commands: ZADD, ZRANGE, ZINCRBY, etc.
- Bitmap Commands: SETBIT, GETBIT, BITCOUNT, BITOP, etc.
- HyperLogLog Commands: PFADD, PFCOUNT, PFMERGE
- Geo Commands: GEOADD, GEODIST, GEORADIUS, etc.
- Stream Commands: XADD, XREAD, XGROUP, etc.

**Extended Features**
- JSON Commands: JSON.GET, JSON.SET, JSON.DEL, etc.
- Time Series Commands: TS.CREATE, TS.ADD, TS.RANGE, etc.
- Search Commands: FT.CREATE, FT.SEARCH, FT.AGGREGATE, etc.
- Probabilistic Commands: BF.ADD, BF.EXISTS, etc.
- Graph Commands: GRAPH.QUERY, etc.

**Infrastructure**
- Tag Commands: TAG.ADD, TAG.GET, TAG.DEL, etc.
- Namespace Commands: NAMESPACE.CREATE, etc.
- Cluster Commands: CLUSTER.ADDNODE, etc.
- Transaction Commands: MULTI, EXEC, DISCARD, etc.
- Pub/Sub Commands: SUBSCRIBE, PUBLISH, etc.

---

## Version History Summary

| Version | Date | Commands | Description |
|---------|------|----------|-------------|
| 0.1.25 | 2026-02-21 | 1,606 | Redis compatibility improvements |
| 0.1.24 | 2026-02-21 | 1,598 | Machine learning commands |
| 0.1.23 | 2026-02-21 | 1,518 | Resilience patterns |
| 0.1.22 | 2026-02-21 | 1,393 | Advanced commands 2 |
| 0.1.21 | 2026-02-21 | 1,218 | Extra commands |
| 0.1.20 | 2026-02-21 | 1,080 | More commands |
| 0.1.19 | 2026-02-21 | 951 | Extended commands |
| 0.1.18 | 2026-02-21 | 854 | Integration commands |
| 0.1.0 | 2026-02-19 | 289 | Initial release |

---

[Unreleased]: https://github.com/cachestorm/cachestorm/compare/v0.1.25...HEAD
[0.1.25]: https://github.com/cachestorm/cachestorm/compare/v0.1.24...v0.1.25
[0.1.23]: https://github.com/cachestorm/cachestorm/compare/v0.1.22...v0.1.23
[0.1.22]: https://github.com/cachestorm/cachestorm/compare/v0.1.21...v0.1.22
[0.1.21]: https://github.com/cachestorm/cachestorm/compare/v0.1.20...v0.1.21
[0.1.20]: https://github.com/cachestorm/cachestorm/compare/v0.1.19...v0.1.20
[0.1.19]: https://github.com/cachestorm/cachestorm/compare/v0.1.18...v0.1.19
[0.1.18]: https://github.com/cachestorm/cachestorm/compare/v0.1.0...v0.1.18
[0.1.0]: https://github.com/cachestorm/cachestorm/releases/tag/v0.1.0

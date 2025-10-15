# ADR-005: Asynchronous Log Persistence System

## Status

Accepted

## Context

When we first built Joblet, log handling seemed simple. Job outputs to stdout/stderr, we capture it, save it, stream it
to clients. Easy, right?

Then we met reality. A machine learning job that dumps gigabytes of debug output. A build system that generates
thousands of lines per second. A data processing pipeline with hundreds of concurrent jobs, each producing verbose logs.
Our naive synchronous approach where jobs waited for log writes hit a wall - hard.

The breaking point came during a stress test with an HPC workload. A user ran 1000 parallel jobs, each outputting
aggressively. The jobs slowed to a crawl, not because of CPU or memory constraints, but because they were waiting for
log I/O. We were literally bottlenecking on disk writes.

We needed to decouple log generation from log persistence. Jobs needed to fire off their logs and keep running, not wait
for some slow disk to catch up.

## Decision

We built a rate-decoupled asynchronous log system. Jobs write to an in-memory channel with microsecond latency. A
background worker pulls from this channel and handles disk persistence. The two sides run at their own pace.

The architecture is elegant in its simplicity:

```go
Job Process → Channel (instant) → Background Worker → Disk (batched)
           ↓
        Subscribers (real-time streaming)
```

Jobs get a simple channel to write to. They dump their logs and continue immediately. The channel is large (100k entries
by default) to handle bursts. The background worker batches writes for efficiency and implements multiple overflow
strategies for when things get really crazy.

## Consequences

### The Good

The performance improvement was dramatic. Jobs now write logs in microseconds instead of milliseconds. That HPC workload
that was crawling? It runs at full speed now, generating 5 million log writes per second without breaking a sweat.

The system gracefully handles bursts. A job can dump a massive amount of output quickly, and the system absorbs it
without blocking. The background worker catches up at its own pace.

Batching improved disk efficiency. Instead of thousands of small writes, we do fewer, larger writes. This is gentler on
SSDs and more efficient for traditional drives.

### The Trade-offs

There's a memory cost. Those logs have to live somewhere between generation and persistence. We cap it at 1GB by
default, but that's still memory that could be used elsewhere.

In extreme cases, we might lose logs. If the system is overwhelmed and memory limits are hit, we have to make choices.
We implemented multiple strategies (compress, spill to temp files, sample), but there's no magic - at some point, you
have to drop data or block.

There's also a slight delay between log generation and persistence. Usually microseconds to milliseconds, but it means
you can't rely on logs being immediately on disk. For crash recovery, this could theoretically mean losing the last few
moments of logs.

### The Overflow Strategies

We didn't just accept potential log loss. We built multiple strategies for handling overflow:

**Compress**: When memory gets tight, compress older log chunks. Trading CPU for memory.

**Spill**: Write excess logs to temporary files. Trading memory pressure for disk I/O.

**Sample**: In extreme cases, keep every Nth log line. Better to have some data than none.

**Alert**: Notify operators that the system is overwhelmed so they can intervene.

The default compression strategy works well for most workloads. Logs compress beautifully (often 10:1 or better), so we
can handle huge bursts without losing data.

### The Unexpected Benefits

The async system enabled features we didn't initially plan. Real-time log streaming became trivial - subscribers just
tap into the channel. Log filtering and transformation can happen in the background worker without affecting job
performance.

It also improved system resilience. A slow disk doesn't slow down jobs. A burst of logs doesn't crash the system. The
decoupling isolated failures and made the system more predictable.

The pattern was so successful that we're considering applying it to other areas - metrics collection, event processing,
even job state updates.

### Real-World Impact

Here's a before and after from that HPC workload:

**Before** (synchronous logs):

- 1000 concurrent jobs
- Each outputting 5000 lines/second
- Jobs blocked 70% of the time on I/O
- Total throughput: 500K lines/second
- Job completion: 45 minutes

**After** (async logs):

- Same 1000 concurrent jobs
- Same 5000 lines/second per job
- Jobs blocked <1% on I/O
- Total throughput: 5M lines/second
- Job completion: 12 minutes

The jobs ran nearly 4x faster, just by fixing log I/O. That's the power of decoupling.

### Streaming Completeness - The Drain Mode Fix (v5.0.0)

Early in production, we discovered a subtle race condition in real-time log streaming. When a job completed, final log
lines were sometimes missing from the live stream - appearing only when querying historical logs. The issue affected the
last 3-4 lines of output, which often contained critical information like final results or error messages.

The root cause was a timing issue between job completion and log delivery:

1. Job process completes and pipes close
2. Final log chunks are still being published to pubsub asynchronously
3. Job status updates to COMPLETED
4. Stream subscribers receive the COMPLETED event and terminate immediately
5. **Final log chunks arrive too late** - the stream has already closed

We implemented a **drain mode** solution that mirrors TCP connection teardown:

```go
When COMPLETED event received:
1. Enter drain mode (don't terminate immediately)
2. Continue processing LOG_CHUNK events for 500ms
3. Terminate only after drain deadline expires
```

This ensures that:

- All final logs are delivered before stream termination
- No logs are lost due to async pub-sub delivery timing
- Clients receive complete output even for short-lived jobs

The fix also addressed related issues:

- **Pubsub blocking**: Changed from non-blocking send (which silently dropped messages) to blocking send with 100ms
  timeout
- **Immediate cleanup**: Removed force-cancellation of subscriptions on job completion
- **UUID filtering**: Fixed subscription filtering to use full UUIDs instead of prefixes

The same drain mode pattern was applied to metrics streaming, ensuring final metrics samples are captured even after the
collector stops.

## Learn More

See [DESIGN.md](/docs/DESIGN.md#51-async-log-persistence-system) for implementation details
and [persistence-log.md](/docs/design/persistence-log.md) for performance characteristics.
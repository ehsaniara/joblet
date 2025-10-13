# ADR 0001: CQRS Architecture with joblet-persist Service

## Status

Accepted

## Date

2025-10-12

## Context

Joblet's original architecture combined all responsibilities (job execution, log storage, metrics collection, historical
queries) in a single monolithic service. This led to several challenges:

1. **Performance bottlenecks**: Historical log/metrics queries competed for resources with active job execution
2. **Scalability limitations**: Storage and compute concerns were tightly coupled
3. **Data management complexity**: Retention policies, cleanup, and archival were intertwined with runtime logic
4. **Single point of failure**: Service restart meant losing access to both live operations and historical data
5. **Resource contention**: Large historical queries could impact job execution performance

## Decision

We implement a CQRS (Command Query Responsibility Segregation) architecture by:

1. **Splitting into two services:**
    - **joblet-core** (Port 50051): Handles commands (job execution, management) and live data streaming
    - **joblet-persist** (Port 50052): Handles queries for historical logs and metrics

2. **Using Unix socket IPC for data replication:**
    - Non-blocking async writes from joblet-core to joblet-persist
    - Protocol buffers for efficient serialization
    - Automatic reconnection with backoff on failures
    - Queue-based buffering to prevent job execution blocking

3. **Maintaining backward compatibility:**
    - joblet-persist is optional
    - joblet-core functions independently if persist is not available
    - Existing deployments continue working without changes

4. **Client-side hybrid fetching:**
    - RNX CLI and SDKs fetch historical data from persist service first
    - Seamlessly transition to live streaming from core service
    - Fallback to live-only mode if persist unavailable

## Consequences

### Positive

1. **Improved performance**: Job execution unaffected by historical queries
2. **Better scalability**: Can scale persist service independently based on query load
3. **Simpler data management**: Dedicated service for retention, cleanup, and archival
4. **Enhanced reliability**: Core service restart doesn't affect historical data access
5. **Cleaner separation of concerns**: Command and query logic decoupled
6. **Resource optimization**: Historical queries use separate process resources
7. **Flexible deployment**: Can deploy persist on different host if needed

### Negative

1. **Increased operational complexity**: Two services to deploy and monitor instead of one
2. **Network dependency**: IPC socket communication introduces failure mode
3. **Data lag**: Small delay between real-time and historical data availability
4. **Storage duplication**: Logs/metrics stored in both services temporarily
5. **Additional configuration**: Users must configure persist service and connection

### Neutral

1. **IPC overhead**: Minimal impact due to async non-blocking design (~1-2% CPU)
2. **Storage requirements**: Persist service needs dedicated disk space for historical data
3. **Learning curve**: Developers must understand dual-service architecture

## Implementation Details

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Client (rnx)                         │
│                                                               │
│  1. Query historical data (Port 50052)                       │
│  2. Stream live data (Port 50051)                            │
└────────────────┬───────────────────────────┬─────────────────┘
                 │                           │
                 │                           │
         ┌───────▼────────┐         ┌───────▼────────┐
         │ joblet-persist │         │  joblet-core   │
         │  (Port 50052)  │         │  (Port 50051)  │
         │                │         │                │
         │ - Query logs   │◄────────│ - Job execute  │
         │ - Query metrics│  Unix   │ - Live stream  │
         │ - Storage      │  Socket │ - Job mgmt     │
         │ - Retention    │  IPC    │ - Scheduling   │
         └────────────────┘         └────────────────┘
```

### Data Flow

1. Job executes and generates logs/metrics in joblet-core
2. joblet-core writes to local files AND sends via IPC to joblet-persist
3. joblet-persist stores data in indexed local storage
4. Client queries historical data from persist service
5. Client streams live data from core service

### Protocol

**IPC Protocol (internal/proto/ipc.proto):**

```protobuf
message IPCMessage {
  uint32 version = 1;
  MessageType type = 2;        // LOG or METRIC
  string job_id = 3;
  int64 timestamp = 4;
  uint64 sequence = 5;
  bytes data = 6;              // Serialized LogLine or Metric
}
```

**Persist API (internal/proto/persist.proto):**

```protobuf
service PersistService {
  rpc QueryLogs(QueryLogsRequest) returns (stream LogLine);
  rpc QueryMetrics(QueryMetricsRequest) returns (stream Metric);
}
```

### Configuration

**joblet-config.yml:**

```yaml
persist:
  enabled: true
  socket: "/tmp/joblet-persist.sock"
  buffer_size: 10000
  reconnect_delay: "5s"
```

**rnx-config.yml:**

```yaml
nodes:
  default:
    address: "server:50051"        # joblet-core
    persistAddress: "server:50052"  # joblet-persist (optional)
```

## Alternatives Considered

### 1. Keep Monolithic Architecture

**Rejected**: Doesn't solve performance and scalability issues. Resource contention would continue.

### 2. Use Message Queue (RabbitMQ/Kafka)

**Rejected**: Adds external dependency and operational complexity. Unix socket IPC is simpler and faster for single-host
deployment.

### 3. Use Shared Database

**Rejected**: Introduces ACID transaction overhead. File-based storage is simpler and faster for append-only
logs/metrics.

### 4. Event Sourcing with Full Replay

**Rejected**: Overkill for our use case. We only need CQRS for read scalability, not full event sourcing.

### 5. HTTP/REST for IPC

**Rejected**: gRPC with Unix sockets is faster and more efficient than HTTP. Protocol buffers provide better
serialization.

## Migration Strategy

### Phase 1: Deploy (Current)

- Deploy joblet-persist alongside joblet-core
- Configure Unix socket IPC
- Historical data starts accumulating

### Phase 2: Client Updates

- Update rnx CLI to use hybrid fetching
- Update Go/Python SDKs with persist client support
- Backward compatible: Works without persist service

### Phase 3: Gradual Rollout

- Deploy persist service to staging first
- Monitor performance and stability
- Roll out to production gradually

### Phase 4: Optimization (Future)

- Implement data archival to cold storage
- Add data compression for older data
- Implement advanced indexing for faster queries

## Monitoring and Observability

### Key Metrics to Monitor

1. **IPC Health:**
    - Messages sent/dropped
    - Reconnection attempts
    - Queue depth

2. **Persist Service:**
    - Query latency
    - Storage usage
    - Index size

3. **Core Service:**
    - IPC write latency
    - Job execution impact
    - Memory usage

### Alerts

- IPC disconnection lasting > 5 minutes
- Message drop rate > 1%
- Persist service query latency > 5s
- Storage usage > 80%

## References

- [CQRS Pattern - Martin Fowler](https://martinfowler.com/bliki/CQRS.html)
- [Event-Driven Architecture](https://martinfowler.com/articles/201701-event-driven.html)
- joblet-proto v2.0.3: https://github.com/ehsaniara/joblet-proto
- persist/README.md: Service documentation

## Notes

- This ADR supersedes any previous decisions about monolithic log/metrics storage
- Future ADRs may address data archival, cold storage, and advanced querying
- IPC implementation is internal and may be replaced without affecting public APIs

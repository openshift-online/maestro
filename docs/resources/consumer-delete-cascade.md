# Explicit consumer delete cascade

Related: [HYPERFLEET-1644](https://redhat.atlassian.net/browse/HYPERFLEET-1644).

## What and why

A permanently decommissioned consumer cannot acknowledge its remaining resource
deletions. Ordinary delete recovery deliberately retains those tombstones: age
alone cannot prove that an agent or its workloads are gone. This opt-in feature
allows an explicit consumer DELETE to finalize old tombstones and notify source
watchers, without direct database repair.

`--consumer-delete-cascade` is **false by default**. With the flag off, consumer
deletion retains its existing foreign-key protection, including soft-deleted
resources. There is no per-request bypass or new API field.

The flag changes the meaning of consumer DELETE for every caller of that server.
Enable it only where existing API callers are trusted to assert that a consumer
is decommissioned. Maestro does not probe the cluster or establish that the agent
is permanently absent. The caller must establish that before invoking deletion;
the age guard is an additional safety check, not proof of decommissioning.

## How it works

```mermaid
sequenceDiagram
    participant Caller
    participant Maestro
    participant PostgreSQL
    participant Watcher as Source gRPC watcher
    Caller->>Maestro: DELETE consumer (cascade flag enabled)
    Maestro->>PostgreSQL: Acquire consumer advisory lock
    Maestro->>PostgreSQL: Begin transaction and lock consumer and resources
    Maestro->>PostgreSQL: Validate every resource against original deleted_at
    alt Any live or recent resource
        Maestro->>PostgreSQL: Roll back
        Maestro-->>Caller: 403 Forbidden
    else All remaining resources are eligible
        Maestro->>PostgreSQL: Hard-delete resources and reconcile pending Delete events
        Maestro->>PostgreSQL: Insert terminal StatusDelete events and NOTIFY
        Maestro->>PostgreSQL: Delete consumer and commit
        PostgreSQL-->>Maestro: Deliver committed status-event notifications
        Maestro-->>Watcher: ResourceDeleted / Deleted
        Maestro-->>Caller: 204 No Content
    end
    Maestro->>PostgreSQL: Release advisory lock
```

Each remaining resource must have `deleted_at` strictly older than the existing
`--stale-delete-event-threshold`. A live resource, a fresh deletion, or a
future-dated deletion refuses the entire operation without changes. The threshold
must be positive and fit a Go duration when cascade is enabled; startup rejects
an enabled cascade with a disabled (`0`), negative, or overflowing threshold.
This does not couple the threshold to recovery backoff settings.

The implementation uses a dedicated database transaction rather than assuming
that independent service calls participate in the HTTP middleware transaction.
Resources, pending Delete-event reconciliation, terminal status events and
consumer removal commit together. PostgreSQL holds the notifications until
commit. A failure while constructing a status, inserting an event, notifying,
deleting the consumer, or committing rolls everything back. Retrying after such a
failure is safe; retrying an already successful DELETE creates no new events.
There is no deliberately tolerated "resource gone, notification missing" gap.

Synthetic events carry `ResourceDeleted=True` with reason
`ConsumerDeletionCascade`. They preserve source, resource ID and generation,
consumer, work metadata and spec payload so the existing SDK decoder and gRPC
watch path can emit `Deleted` after the resource row has gone. A terminal event
includes a valid sequence string (`0`); source stores handle deleted conditions
before sequence ordering. It is explicitly not presented as an agent
acknowledgement.

## Concurrency and recovery state

Consumer cascades and terminal agent acknowledgements share a per-consumer
advisory lock. An acknowledgement re-reads the resource under that lock before
creating a terminal event, so an acknowledgement racing a cascade cannot emit a
second terminal event for a resource already finalized by the cascade. All
servers must use this coordination code before enabling the flag; it cannot
coordinate older binaries during a mixed-version rollout.

Within the transaction, the consumer is locked `FOR NO KEY UPDATE` and resource
rows are locked in ID order. The weaker consumer row lock permits the recovery
scheduler's foreign-key checks while it holds resource locks, avoiding a
consumer/resource lock-order inversion. If a concurrent caller creates a new
resource, the final consumer DELETE remains protected by its foreign key and
rolls back the entire cascade rather than abandoning that new resource.

Pending resource Delete events are marked reconciled. Deleting the consumer also
removes its durable recovery queue entry via the existing cascading foreign key.
Deleted resource rows no longer carry retry state. Resources and recovery work
for other consumers are unchanged.

## Observability

`consumer_delete_cascaded_resources_total` counts resources finalized in
successfully committed cascades. It has no resource or consumer labels. Failed
transactions and retries of an already deleted consumer do not increment it.
Each committed resource finalization also produces an audit log with its resource
ID and consumer ID. The counter is process-local, not an exactly-once audit
ledger: a crash after commit but before recording the metric can undercount.

## Trade-offs

The all-or-nothing transaction holds locks for the complete consumer cleanup.
Large consumers may require a long request and delay acknowledgements for that
consumer. This is an explicit decommissioning operation, not a periodic sweep or
a way to increase normal deletion throughput. Other consumers do not take its
advisory lock.

## Alternatives

| Alternative | Why it is not used |
| --- | --- |
| Hard-delete after an elapsed timer | Age cannot distinguish a dead agent from an unreachable live agent |
| Delete all resources on consumer deletion | Could silently abandon live workloads; live and fresh rows must refuse the operation |
| Delete resources and notify in separate commits | Leaves watchers unaware if event creation fails after resource deletion |
| Keep requiring manual SQL cleanup | Does not exercise the normal status notification path and is harder to audit safely |

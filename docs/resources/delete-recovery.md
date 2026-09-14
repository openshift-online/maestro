# Delete recovery

Maestro keeps soft-deleted resources until the agent reports `ResourceDeleted`.
Deletion age does not expire recovery or authorize hard deletion. Tombstones stay
visible to source clients and reconnect/spec resync. A continuously connected
agent that loses deletion state also needs recovery publications, without waiting
for another delete request or reconnect.

The first delete request writes its event immediately. Retried delete requests are
idempotent and do not query or append events. A server controller schedules recovery
independently of request traffic, while the message broker is enabled.

## Bounds and fairness

`--delete-event-republish-batch-size` defaults to **100** (allowed range 1-1000).
Each fleet-wide round:

* Initializes at most that many unscheduled tombstones, oldest deletion first.
* Visits at most that many due consumers, oldest queued deadline first.
* Visits only the oldest due resource of each selected consumer.
* Appends at most one Delete event per selected consumer, and none if that resource
  already has an unreconciled Delete event.

A durable database deadline permits one round, then waits at least **one second
from the end of its work** before admitting another. Replicas cannot take sequential
immediate bursts, and downtime does not accumulate credits. At the defaults this
allows at most 100 recovery publications per round and one per consumer per round,
not a claim about total create/update/delete throughput. Initialization and recovery
share a transaction but each has its own batch bound.

Consumers with more due resources rotate behind consumers already waiting. Within
a consumer, resources are ordered by their persistent due time and ID. A large
consumer cannot take the entire batch. This is bounded FIFO rotation among
**initialized** due consumers, not equal per-resource throughput or a latency SLA.
A finite bootstrap backlog drains oldest-first; consumers not yet initialized wait
for that pass. Locked resource candidates are skipped without expanding the scan
to replace them. Persistent lock contention, publication failures, a backlog beyond
the configured capacity, or a stopped scheduler can delay recovery.

Only the scheduler takes the fleet deadline lock. Normal resource requests do not
take it. Candidate queries use partial indexes and bounded limits; point lookups
and per-consumer minimum-deadline lookups use indexes. Each controller round has a
30-second deadline. A failed round rolls back events, notifications, retry state
and the fleet deadline together.

## Persistent retry state

`--delete-event-republish-interval` defaults to **60 seconds**, the initial backoff.
The first scheduled attempt is due no earlier than the original deletion time plus
that interval. A tombstone older than this can recover as soon as its bounded turn
arrives. After each recovery publication, the nominal backoff doubles, capped by
`--delete-event-republish-max-interval` (default **3600 seconds**). The persisted next
attempt uses equal jitter between half and the full nominal backoff. Due times are
eligibility times, not guaranteed delivery times.

An outstanding publication does not advance the backoff or append another event.
The scheduler checks it again after the initial interval, subject to queue turns.
Publishing marks an event reconciled, not the resource acknowledged. If a later
attempt is due, another Delete event may therefore be needed. The stale-event
detector can also retire an outstanding event; the next due turn can replace it.
New recovery events get the detector's full age threshold even for old tombstones.

The next attempt and nominal backoff live on the resource, independent of event
purging, server restarts and caller retries. The final acknowledgement removes
the resource and this state together. An empty consumer queue entry is removed
on its next bounded turn. Late event-controller writes update existing events
only, and late status writes cannot recreate resources.

Setting `--delete-event-republish-interval=0` disables the recovery controller,
including bootstrap. It does not disable initial deletion, acknowledgement,
event-controller retries, stale-event retirement, or reconnect/spec resync.
Existing scheduling state is retained; re-enabling resumes it without catch-up
credits. Configure all replicas consistently.

## Migration and rollout

Run the schema migration before starting servers that use the scheduler. It adds
nullable resource retry columns without eagerly updating tombstones, two small
scheduler tables, and concurrent partial indexes for bootstrap, per-consumer due
work and pending Delete lookups. Interrupted index builds are rebuilt on migration
retry. Existing duplicate Delete events are accepted, not bulk-rewritten by the
migration; new publications coalesce against any pending one.

The schema is compatible with servers that do not know about recovery columns.
Fleet pacing and caller coalescing apply once **all** servers use this code.
During a mixed-version rollout, servers using request-driven republishing can
still enqueue their own recovery events outside the new fleet budget. Stop the
new controllers before rolling back this migration. Rollback removes scheduling
metadata, not resources, their deletion timestamps, or events. Reapplying the
migration bootstraps surviving tombstones with the configured initial backoff.
Schema changes use a five-second lock timeout so a blocked migration can be retried
instead of waiting indefinitely behind application transactions.

This migration is separate from the resource-label JSON containment index.
Source cleanup-loop rate limiting is outside Maestro's scope. Bounding recovery
publications does not by itself establish that all incident database CPU is fixed.

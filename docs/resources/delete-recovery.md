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

`--delete-event-republish-batch-size` defaults to **25** (allowed range 1-1000).
Each fleet-wide round:

* Initializes at most that many unscheduled tombstones, oldest deletion first.
* Visits at most that many due consumers, oldest queued deadline first.
* Visits only the oldest due resource of each selected consumer.
* Appends at most one Delete event per selected consumer, and none if that resource
  already has an unreconciled Delete event.

A durable database deadline permits one round, then waits at least **one second
from the end of its work** before admitting another. Replicas cannot take sequential
immediate bursts, and downtime does not accumulate credits. At the defaults this
allows at most 25 recovery publications per round and one per consumer per round,
not a claim about total create/update/delete throughput. Initialization and recovery
share a transaction but each has its own batch bound.

The default bounds each transaction to 25 initializations and 25 consumer visits.
This is one quarter of the work budget of an explicit batch of 100, so large
fleets wait longer for their turn. With 200,000 continuously due consumers, one
visit each requires at least 8,000 rounds (2,000 with batch 100), including at
least 7,999 one-second inter-round gaps plus transaction time. This is an ideal
capacity lower bound, not a recovery deadline; bootstrap, backoff, polling and
contention can add delay.

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

## Recovery telemetry and lock contention

The batch of 25 is a conservative starting point informed by local measurements,
not a production-safe latency guarantee. An isolated PostgreSQL 17.10 comparison
on a 16-logical-CPU AMD EPYC WSL host with 47 GiB RAM used 200,000 synthetic
tombstones across 10,000 consumers, durable writes, 100 unscheduled candidates,
and 40 measured rounds per batch/scenario (three warmups excluded):

| Scenario | Batch | Round p50 / p95 / max (ms) | Colliding acknowledgement lock wait p50 / p95 / max (ms) |
| --- | ---: | ---: | ---: |
| Publication | 100 | 395 / 444 / 457 | 392 / 441 / 452 |
| Publication | 25 | 112 / 122 / 139 | 108 / 118 / 134 |
| Pending events | 100 | 350 / 369 / 409 | 347 / 366 / 407 |
| Pending events | 25 | 93 / 107 / 115 | 90 / 104 / 112 |
| Full status acknowledgement handler | 100 | 403 / 451 / 513 | 397 / 443 / 507 |
| Full status acknowledgement handler | 25 | 104 / 118 / 123 | 98 / 112 / 117 |

These 240 samples use the actual recovery DAO and deliberately start the
acknowledgement after the first initialized resource is locked. PostgreSQL lock
logs and blocker PIDs confirm the waits, not a subtraction from application
latency. Direct-delete scenarios exercise the resource-service hard delete; the
full-handler scenario includes status-event processing, but no broker or agent.
Each round initializes and visits its configured batch, publishing that many
events unless they are pending. Measurements are warm-cache, sequential runs
with fixed reset targets, synthetic approximately 1 KiB payloads and a polling
observer. They do not measure collision frequency, full-fleet drain time or
predict latency on a production 2-vCPU database. Use rollout telemetry and
acknowledgement evidence before tuning the batch upward.

The [measurement archive](https://redhat.atlassian.net/secure/attachment/1186970/pr597-delete-recovery-measurements.tar.gz)
contains the harnesses, raw samples, PostgreSQL lock logs, per-round statement
statistics, settings and analysis scripts for this comparison and the broader
batch-100 contention study. The comparison is in `pr597-batch-comparison`;
`analyze.py` joins acknowledgements to server lock logs by backend PID and time,
checks the committed work and blocker PID, and regenerates `summary.json`.
The p50 is the median; p95 uses nearest rank. Reproduction scripts record the
local fixture layout and must be adapted to a fresh, isolated local PostgreSQL
cluster, never pointed at an existing shared database.

`delete_recovery_round_duration_seconds{outcome}` measures elapsed time around the
recovery runner, including connection acquisition, the transaction and its commit
or error/rollback return. It is not SQL execution time or PostgreSQL row lock wait
time. An error observation ends when the runner returns, not proof that a server
backend released every lock at that instant.

The bounded outcomes are:

* `committed`: the fleet round committed with initialized resources or visited
  consumers. A visited consumer can have pending events, locked resources or an
  empty queue; this does not imply a publication.
* `empty`: the fleet round committed without initializations or consumer visits.
* `noop`: no fleet round was claimed (deadline not due, another replica holds it,
  or recovery is disabled when the runner is called directly).
* `error`: the runner returned an error, including transaction/commit failures.

`delete_recovery_work_total{kind}` counts only committed `initialized` resources,
visited `consumers`, and `published` events. Failed rounds contribute no work.
Publications are durable event requests, not broker delivery or acknowledgements.
Neither metric labels resources, consumers, SQL text or error messages.

Filter the histogram when examining active rounds, so idle polls and empty rounds
do not hide transaction latency:

```promql
histogram_quantile(0.95,
  sum by (le) (rate(delete_recovery_round_duration_seconds_bucket{outcome="committed"}[5m]))
)
```

Inspect `empty`, `noop` and `error` counts separately, together with work-counter
rates. Do not pool their duration distributions with committed work. The histogram
includes buckets through 30 and 60 seconds to expose slow/error rounds around the
controller's cancellation deadline.

Recovery holds resource locks until the round transaction completes. A colliding
acknowledgement hard delete can wait behind those locks. `SKIP LOCKED` makes the
scheduler skip a held candidate; timing that SELECT would not measure the
acknowledgement's row lock wait. For an authorized, read-only investigation, sample
actual waiting backends separately rather than adding database polling to every
server:

```sql
SELECT clock_timestamp() AS sampled_at,
       pid AS waiting_pid, application_name, wait_event_type, wait_event,
       pg_blocking_pids(pid) AS blocking_pids,
       clock_timestamp() - query_start AS statement_age
FROM pg_stat_activity
WHERE datname = current_database() AND wait_event_type = 'Lock';
```

Match the blocker PID to a recovery transaction and the waiter to the
acknowledgement. `statement_age` includes execution before the wait and is not
pure lock-wait duration; intermittent sampling can miss short waits. Where
permitted, PostgreSQL `log_lock_waits` provides server lock-acquisition timings
above its `deadlock_timeout` threshold. That is a censored distribution, not
all-request acknowledgement latency. Do not enable verbose logging or lower that
threshold in production without assessing its overhead.

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
Schema changes, including concurrent index drops and builds, use a five-second
timeout per lock wait so blocked operations can be retried. This is not a
five-second limit on the migration or total index build time. The concurrent-index
phase restores the session's original timeout even on failure, or discards the
connection if restoration fails. Migrators hold a session advisory lock across
schema changes and migration history writes, so replicas cannot race the index DDL.

This migration is separate from the resource-label JSON containment index.
Source cleanup-loop rate limiting is outside Maestro's scope. Bounding recovery
publications does not by itself establish that all incident database CPU is fixed.

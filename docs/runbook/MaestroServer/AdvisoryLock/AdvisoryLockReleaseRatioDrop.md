# Maestro Advisory Lock Release Rate Drop (Alpha)

## Symptom

A drop in the advisory lock release rate suggests that some acquired locks are not being properly released.

You can check this condition using the following PromQL query:

```promql
(
  sum(rate(advisory_unlock_count[5m]))
  /
  sum(rate(advisory_lock_count[5m]))
) < 0.99
```

## Meaning

Fewer than 99% of advisory locks acquired are released within a 5-minute window, indicating delays in lock handling.

## Impact

Unreleased locks can cause resource contention, increased latency, blocked operations, and degraded performance in maestro.

## Diagnosis

Identify long-running or idle transactions opened by maestro:

```psql
SELECT pid, usename, state, now() - xact_start AS age, query
  FROM pg_stat_activity
  WHERE xact_start IS NOT NULL
    AND usename = 'maestro'
  ORDER BY age DESC;
```

Example output:

```psql
 pid  | usename |         state         |   age    |           query
------+---------+------------------------+----------+--------------------------
 1234 | maestro | idle in transaction    | 00:45:12 | SELECT FROM resources ...
 5678 | maestro | active                 | 00:02:14 | INSERT INTO events ...
```

- `xact_start`: When the transaction started.
- `age`: Duration the transaction has been open.
- `state`: Session state (e.g., active, idle in transaction).
- `query`: SQL query running in the session.

Run this query multiple times to check for persistent long-running idle transactions.

If no long-running transactions are found, review maestro server logs for potential issues:

```shell
kubectl -n maestro logs -l app=maestro -c maestro --tail=100
```

Look for signs of:

- Connection pool exhaustion (too many open connections)
- Database transaction errors
- Panic or runtime errors
- Deadlocks or timeouts

# Purpose
pgfailure is a tool that can be deployed as a pod to inject behaviors into maestro's postgres database to
trigger targeted failure modes.  It is currently used in test/e2e/pkg/pg_failure_modes_test.go and
test/e2e/pkg/stress_test.go to for this purpose.

# Usage
The tool takes an arbitrary number of commands that will be executed on pod startup in sequence.

```
pgfailure -c "command [args]" -c "command [args]" -c "command [args]" ...
```

## Available commands:
| Command | Description | Example |
| --- | --- | --- |
| lock | locks the specified table with an exclusive lock (i.e. blocks writes) | pgfailure -c "lock events" |
| unlock | removes an exclusive lock | pgfailure -c "unlock events" |
| block | adds a trigger that raises an exception, effectively blocking write opreations on the target table | pgfailure -c "block events" |
| unblock | remove the trigger added by block from the target table | pgfailure -c "unblock events" |
| terminate backends | terminates active client backends that are waiting on a lock with INSERT or UPDATE statements | pgfailure -c "lock events" -c "terminate backends" |
| terminate listeners| terminates backends actively listening on the target queue | pgfailure -c "terminate listeners" |
| notify-queue fill | fills the NOTIFY queue until `too many notifications in the NOTIFY queue` errors occurr | pgfailure -c "notify-queue fill" |
| notify-queue drain | attempts to drain the NOTIFY queue until `pg_notification_queue_usage()` reads 0.0 | pgfailure -c "notify-queue drain" |
| notify-queue set-max-size | set max_notify_queue_pages = 64, which reduces the time required to fill the queue (requires a full postgres restart) | pgfailure -c "notify-queue set-max-size" |
| notify-queue reset-max-size | resets max_notify_queue_pagesw to the configured default | pgfailure -c "notify-queue reset-max-size" |

## Example Usage

### Lock a table, then terminate backends waiting on that lock 
```bash
# lock the events table for writes
pgfailure -c "lock events"

# asynchronously perform commands that INSERT/UPDATE/DELETE records in the events table...

# terminate any backends that are waiting on the lock, and then immediate remove the lock
pgfailure -c "terminate backends" -c "unlock events"
```

### Lock a table, then terminate listeners and immediately unlock the table
This combination is used to reproduce listener drops when inserting `events` or `spec_events` records.
In that situation we have an INSERT followed immediately by a pg_notify, and this combination will
**usually** result in pg_notify() executing before the listener can reconnect, simulating a dropped
NOTIFY.

```bash
# lock the events table for writes
pgfailure -c "lock events"

# asynchronously perform commands that INSERT records to the events table followed by pg_notify e.g. 
# INSERT INTO events VALUES(...)
# pg_notify(...)

# terminate any listeners on the events queue and immediately unlock the table
pgfailure -c "terminate listeners" -c "unlock events"

# listeners will usually not reconnect before pg_notify is executed
```

### Block any writes to a table, then unblock to allow writes again
```bash
# block INSERT/UPDATE/DELETE statements against the events table
pgfailure -c "block events"

# any INSERT/UPDATE/DELETE statements will fail immediately...

# unblock INSERT/UPDATE/DELETE statements against the events table
pgfailure -c "unblock events"

# any INSERT/UPDATE/DELETE statements will succeed...
```

### Fill the NOTIFY queue, then drain the NOTIFY queue
```bash
# fill the NOTIFY queue
pgfailure -c "notify-queue fill"

# any pg_notify() statements will fail with `too many notifications in the NOTIFY queue`...

# drain the NOTIFY queue
pgfailure -c "notify-queue drain"

# pg_notify() statements will succeed...
```

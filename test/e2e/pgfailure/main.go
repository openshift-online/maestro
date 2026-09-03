package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/lib/pq"
)

type handler struct {
	fn func(connStr string, args []string)
}

type handlerInstance struct {
	h    handler
	args []string
}

var handlers = map[string]handler{
	"lock":                        {fn: lockTable},
	"unlock":                      {fn: unlockTable},
	"block":                       {fn: blockTable},
	"unblock":                     {fn: unblockTable},
	"terminate backends":          {fn: terminateBackends},
	"terminate listeners":         {fn: terminateListeners},
	"notify-queue fill":           {fn: fillNotifyQueue},
	"notify-queue drain":          {fn: drainNotifyQueue},
	"notify-queue set-max-size":   {fn: setMaxNotifyQueueSize},
	"notify-queue reset-max-size": {fn: resetMaxNotifyQueueSize},
}

func main() {
	commands := parseFlags(os.Args[1:])
	if len(commands) == 0 {
		log.Fatalf("usage: pgfailure -c \"command [args]\" [-c \"command [args]\" ...]")
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable application_name=pgfailure",
		readFile("/secrets/rds/db.host"),
		readFile("/secrets/rds/db.port"),
		readFile("/secrets/rds/db.user"),
		readFile("/secrets/rds/db.password"),
		readFile("/secrets/rds/db.name"),
	)

	for _, instance := range commands {
		instance.h.fn(connStr, instance.args)
	}

	signalReady()
	select {}
}

func parseFlags(args []string) []handlerInstance {
	var commands []handlerInstance
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-c":
			if i+1 >= len(args) {
				log.Fatalf("-c requires an argument")
			}
			i++
			commands = append(commands, matchCommand(args[i]))
		default:
			log.Fatalf("unknown flag: %s", args[i])
		}
	}
	return commands
}

func matchCommand(raw string) handlerInstance {
	var name string
	for cmd := range handlers {
		if raw == cmd || strings.HasPrefix(raw, cmd+" ") {
			if len(name) < len(cmd) {
				name = cmd
			}
		}
	}
	if name == "" {
		log.Fatalf("unknown command: %s", raw)
	}
	var instance handlerInstance
	instance.h = handlers[name]
	rest := strings.TrimSpace(strings.TrimPrefix(raw, name))
	if rest != "" {
		instance.args = strings.Fields(rest)
	}
	return instance
}

func lockTable(connStr string, args []string) {
	if len(args) != 1 {
		log.Fatalf("lock requires a table argument")
	}
	table := args[0]

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open connection: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("failed to begin transaction: %v", err)
	}

	stmt := fmt.Sprintf("LOCK TABLE %s IN EXCLUSIVE MODE", pq.QuoteIdentifier(table))
	if _, err := tx.Exec(stmt); err != nil {
		tx.Rollback()
		log.Fatalf("failed to lock table %s: %v", table, err)
	}
	log.Printf("pgfailure: table %s locked", table)
}

func unlockTable(connStr string, args []string) {
	if len(args) != 1 {
		log.Fatalf("unlock requires a table argument")
	}
	table := args[0]

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open connection: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	rows, err := db.Query(`
		SELECT pid FROM pg_locks l
		JOIN pg_class c ON l.relation = c.oid
		WHERE c.relname = $1
		AND l.mode = 'ExclusiveLock'
		AND l.granted = true
		AND pid != pg_backend_pid()
		`, table)
	if err != nil {
		log.Fatalf("failed to query lock holders for %s: %v", table, err)
	}

	var pids []int64
	for rows.Next() {
		var pid int64
		if err := rows.Scan(&pid); err != nil {
			log.Fatalf("failed to scan pid: %v", err)
		}
		pids = append(pids, pid)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		log.Fatalf("failed to iterate lock holders for %s: %v", table, err)
	}

	if len(pids) == 0 {
		log.Printf("pgfailure: no lock holders found for table %s", table)
		return
	}

	for _, pid := range pids {
		var terminated bool
		err := db.QueryRow("SELECT pg_terminate_backend($1)", pid).Scan(&terminated)
		if err != nil {
			log.Printf("pgfailure: error terminating lock holder pid=%d: %v", pid, err)
			continue
		}
		log.Printf("pgfailure: terminated lock holder pid=%d for table %s result=%v", pid, table, terminated)
	}
}

func blockTable(connStr string, args []string) {
	if len(args) != 1 {
		log.Fatalf("block requires a table argument")
	}
	table := args[0]

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open connection: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	triggerName := fmt.Sprintf("pgfailure_block_%s", table)
	funcName := fmt.Sprintf("pgfailure_block_%s_fn", table)

	createFunc := fmt.Sprintf(
		`CREATE OR REPLACE FUNCTION %s() RETURNS trigger AS $$ BEGIN RAISE EXCEPTION 'pgfailure: writes blocked on %s'; END; $$ LANGUAGE plpgsql`,
		pq.QuoteIdentifier(funcName), table)
	if _, err := db.Exec(createFunc); err != nil {
		log.Fatalf("failed to create block function for %s: %v", table, err)
	}

	createTrigger := fmt.Sprintf(
		`CREATE OR REPLACE TRIGGER %s BEFORE INSERT OR UPDATE OR DELETE ON %s FOR EACH ROW EXECUTE FUNCTION %s()`,
		pq.QuoteIdentifier(triggerName), pq.QuoteIdentifier(table), pq.QuoteIdentifier(funcName))
	if _, err := db.Exec(createTrigger); err != nil {
		log.Fatalf("failed to create block trigger on %s: %v", table, err)
	}
	log.Printf("pgfailure: writes blocked on %s", table)
}

func unblockTable(connStr string, args []string) {
	if len(args) != 1 {
		log.Fatalf("unblock requires a table argument")
	}
	table := args[0]

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open connection: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	triggerName := fmt.Sprintf("pgfailure_block_%s", table)
	funcName := fmt.Sprintf("pgfailure_block_%s_fn", table)

	dropTrigger := fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s",
		pq.QuoteIdentifier(triggerName), pq.QuoteIdentifier(table))
	if _, err := db.Exec(dropTrigger); err != nil {
		log.Fatalf("failed to drop block trigger on %s: %v", table, err)
	}

	dropFunc := fmt.Sprintf("DROP FUNCTION IF EXISTS %s()", pq.QuoteIdentifier(funcName))
	if _, err := db.Exec(dropFunc); err != nil {
		log.Fatalf("failed to drop block function for %s: %v", table, err)
	}
	log.Printf("pgfailure: writes unblocked on %s", table)
}

func terminateBackends(connStr string, _ []string) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open connection: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	terminateRows, err := db.Query(`
		SELECT pg_terminate_backend(pid), pid, state, wait_event_type, left(query, 80)
		FROM pg_stat_activity
		WHERE pid != pg_backend_pid()
		AND datname = current_database()
		AND backend_type = 'client backend'
		AND state = 'active'
		AND wait_event_type = 'Lock'
		AND (query LIKE 'INSERT %' OR query LIKE 'UPDATE %')
		`)
	if err != nil {
		log.Fatalf("failed to terminate backends: %v", err)
	}
	defer terminateRows.Close()

	count := 0
	for terminateRows.Next() {
		var terminated bool
		var pid int
		var state, waitEvent, query sql.NullString
		if err := terminateRows.Scan(&terminated, &pid, &state, &waitEvent, &query); err != nil {
			log.Printf("pgfailure: scan error: %v", err)
			continue
		}
		log.Printf("pgfailure: pid=%d terminated=%v state=%s wait=%s query=%s",
			pid, terminated, state.String, waitEvent.String, query.String)
		count++
	}
	log.Printf("pgfailure: terminated %d backends", count)
}

func terminateListeners(connStr string, args []string) {
	if len(args) != 1 {
		log.Fatalf("terminate listeners requires a queue name argument")
	}
	queue := args[0]

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open connection: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	rows, err := db.Query(`
		SELECT pg_terminate_backend(pid), pid
		FROM pg_stat_activity
		WHERE application_name = $1
		AND pid != pg_backend_pid()
		`, fmt.Sprintf("%s_listener", queue))
	if err != nil {
		log.Fatalf("failed to terminate listeners on %s: %v", queue, err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var terminated bool
		var pid int
		if err := rows.Scan(&terminated, &pid); err != nil {
			log.Printf("pgfailure: scan error: %v", err)
			continue
		}
		log.Printf("pgfailure: terminated listener pid=%d on queue %s result=%v", pid, queue, terminated)
		count++
	}
	log.Printf("pgfailure: terminated %d listener(s) on queue %s", count, queue)
}

const notifyChannel = "test_queue_exhaustion"

func fillNotifyQueue(connStr string, _ []string) {
	listener := pq.NewListener(connStr, 10*time.Second, time.Minute, func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Printf("listener event %d: %v", ev, err)
		}
	})
	if err := listener.Listen(notifyChannel); err != nil {
		log.Fatalf("failed to LISTEN on %s: %v", notifyChannel, err)
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open sender connection: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	payload := strings.Repeat("x", 7000)
	count := 0
	for {
		_, err := db.Exec("SELECT pg_notify($1, $2)", notifyChannel, payload)
		if err != nil {
			if strings.Contains(err.Error(), "too many notifications in the NOTIFY queue") {
				break
			}
			log.Printf("NOTIFY error after %d sends: %v", count, err)
			time.Sleep(time.Second)
			continue
		}
		count++
		if count%1000 == 0 {
			log.Printf("sent %d notifications", count)
		}
	}

	log.Printf("pgfailure: notification queue full after %d sends", count)
}

func drainNotifyQueue(connStr string, _ []string) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open connection: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	rows, err := db.Query(`
		SELECT pg_terminate_backend(pid), pid
		FROM pg_stat_activity
		WHERE query = $1
		AND pid != pg_backend_pid()
		`, fmt.Sprintf("LISTEN \"%s\"", notifyChannel))
	if err != nil {
		log.Fatalf("failed to terminate filler listeners: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var terminated bool
		var pid int
		if err := rows.Scan(&terminated, &pid); err != nil {
			log.Printf("pgfailure: scan error: %v", err)
			continue
		}
		log.Printf("pgfailure: terminated listener pid=%d result=%v", pid, terminated)
		count++
	}
	log.Printf("pgfailure: terminated %d filler listener(s)", count)

	notified := false
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		var usage float64
		if err := db.QueryRow("SELECT pg_notification_queue_usage()").Scan(&usage); err != nil {
			log.Fatalf("failed to query notification queue usage: %v", err)
		}
		if usage == 0 {
			log.Printf("pgfailure: notification queue drained")
			return
		}
		if !notified {
			if _, err := db.Exec("SELECT pg_notify('events', 'drain')"); err != nil {
				log.Printf("pgfailure: drain notify failed (will retry): %v", err)
			} else {
				log.Printf("pgfailure: sent drain notify on events channel")
				notified = true
			}
		}
		log.Printf("pgfailure: notification queue usage: %.4f, waiting...", usage)
		time.Sleep(time.Second)
	}
	log.Fatalf("pgfailure: notification queue did not drain within 1 minute")
}

func setMaxNotifyQueueSize(connStr string, _ []string) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open connection: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	if _, err := db.Exec("ALTER SYSTEM SET max_notify_queue_pages = 64"); err != nil {
		log.Fatalf("failed to set max_notify_queue_pages: %v", err)
	}
	log.Printf("pgfailure: max_notify_queue_pages set to 64 (restart required)")
}

func resetMaxNotifyQueueSize(connStr string, _ []string) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("failed to open connection: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	if _, err := db.Exec("ALTER SYSTEM RESET max_notify_queue_pages"); err != nil {
		log.Fatalf("failed to reset max_notify_queue_pages: %v", err)
	}
	log.Printf("pgfailure: max_notify_queue_pages reset to default (restart required)")
}

func signalReady() {
	ln, err := net.Listen("tcp", ":8081")
	if err != nil {
		log.Fatalf("failed to open readiness port: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read %s: %v", path, err)
	}
	return strings.TrimSpace(string(data))
}

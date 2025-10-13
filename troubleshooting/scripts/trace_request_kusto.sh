#!/bin/bash

svr_log=${svr_log:-"$HOME/logs/export_svr.csv"}
agent_log=${agent_log:-"$HOME/logs/export_agent.csv"}

resource_id=${resource_id:-""}
work_name=${work_name:-""}

resource_request=${resource_request:-"create_request"}

# log file
timestamp=$(date '+%Y-%m-%d_%H-%M-%S')
trace_log_file="trace_request.${timestamp}.log"

case "$resource_request" in
    create_request)
        db_action="insert"
        status_action="update"
        evt_type="ADDED"
        ;;
    update_request)
        db_action="update"
        status_action="update"
        evt_type="MODIFIED"
        ;;
    delete_request)
        db_action="delete"
        status_action="delete"
        evt_type="DELETED"
        ;;
    *)
        echo "Unknown resource_request: $resource_request"
        exit 1
        ;;
esac

echo "----------------------"
echo "Trace resource request"
echo "----------------------"
echo "Maestro server log: $svr_log"
echo "Maestro agent log: $agent_log"
echo "Maestro resource ID: $resource_id"
echo "Clusters service work name: $work_name"
echo "Resource request: $resource_request"
echo "Trace log: $trace_log_file"
echo "----------------------"

if [ ! -f "$svr_log" ]; then
    echo "Error: Maestro server log not found: $svr_log"
    exit 1
fi

if [ ! -f "$agent_log" ]; then
    echo "Error: Maestro agent log not found: $agent_log"
    exit 1
fi

# Clusters Service (CS) publishes spec request to Maestro server
echo "Trace resource spec request on maestro server ..."
result=$(cat $svr_log | \
    grep -A 2 "INFO" | \
    grep -A 2 "receive the event from client" | \
    grep -B 1 -A 1 $resource_request | \
    grep -B 2 $resource_id)
if [ -z "$result" ]; then
    echo "ERROR: Maestro server does not receive the spec request"
    exit 1
fi

# Maestro server publishes the CS spec request to mqtt broker (Azure Event Grid)
result=$(cat $svr_log | grep "Publishing resource" | grep $resource_id | grep $db_action)
if [ -z "$result" ]; then
    echo "ERROR: Maestro server does not publish the spec request"
    exit 1
fi

result=$(cat $svr_log | grep -A 2 "INFO" | grep -B 1 -A 1 $resource_request | grep -B 2 $resource_id)
echo "$result" >> $trace_log_file

# Maestro agent subscribes to mqtt broker receiving the CS spec request
echo "Trace resource request on maestro agent ..."
result=$(cat $agent_log | grep -B 1 -A 1 $resource_request | grep -B 2 $resource_id)
if [ -z "$result" ]; then
    echo "ERROR: Maestro agent does not receive the spec request"
    exit 1
fi
echo "$result" >> $trace_log_file

result=$(cat $agent_log | grep "Receive the event" | grep $work_name)
if [ -z "$result" ]; then
    echo "ERROR: Maestro agent does not handle the spec request"
    exit 1
fi
echo "$result" >> $trace_log_file

# Maestro agent publishes status update request to mqtt broker
result=$(cat $agent_log | grep -A 2 "Sending event" | grep -B 1 $resource_id | grep -v '^--$')
if [ -z "$result" ]; then
    echo "ERROR: Maestro agent does not publish the status update request"
    exit 1
fi
echo "$result" >> $trace_log_file

# Maestro server subscribes to mqtt broker receiving the status update request
echo "Trace resource status update request on maestro server ..."
result=$(cat $svr_log | grep "received action STATUSMODIFIED" | grep $resource_id | grep -v '^--$')
if [ -z "$result" ]; then
    echo "ERROR: Maestro server does not receive the status update request"
    exit 1
fi
echo "$result" >> $trace_log_file

result=$(cat $svr_log | grep "skipping resource status update" | grep $resource_id | grep -v '^--$')
echo "$result" >> $trace_log_file

result=$(cat $svr_log | grep "status $status_action event was sent" | grep $resource_id | grep -v '^--$')
if [ -z "$result" ]; then
    echo "ERROR: Maestro server does not handle the status update request"
    exit 1
fi
echo "$result" >> $trace_log_file

result=$(cat $svr_log | grep "Broadcast the resource status" | grep $resource_id | grep -v '^--$')
if [ -z "$result" ]; then
    echo "ERROR: Maestro server does not broadcast the status update request"
    exit 1
fi
echo "$result" >> $trace_log_file

# Maestro server send the status update to CS
result=$(cat $svr_log | \
    grep -A 2 "INFO" | \
    grep -A 2 "send the event to status subscribers" | \
    grep -B 2 $resource_id)
if [ -z "$result" ]; then
    echo "ERROR: Maestro server does not publish the status update request"
    exit 1
fi
echo "$result" >> $trace_log_file

result=$(sort -t',' -k2 $trace_log_file | grep -v '^--$' | \
    awk '/receive the event from client/ && !flag { flag=1 } flag')
echo "$result" > $trace_log_file

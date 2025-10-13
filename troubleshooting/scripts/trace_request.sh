#!/bin/bash

logs_dir=${logs_dir:-"$HOME/maestro-logs"}

resource_id=${resource_id:-"c49f39cc-561e-5293-944e-4ba5a75bcf52"}
work_name=${work_name:-"work-kmhm7"}

resource_request=${resource_request:-"delete_request"}

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
echo "Logs directory: $logs_dir"
echo "Maestro resource ID: $resource_id"
echo "Clusters service work name: $work_name"
echo "Resource request: $resource_request"
echo "Trace log: $logs_dir/$trace_log_file"
echo "----------------------"

cd "$logs_dir" || exit 1

# Clusters Service (CS) publishes spec request to Maestro server
echo "Trace resource spec request on maestro server ..."
has_result=false
for file in maestro-*.log; do
    if [[ ! "$file" == maestro-agent-* ]]; then
        result=$(cat $file | \
            grep -A 10 "INFO" | \
            grep -A 10 "receive the event from client" | \
            grep -B 2 -A 8 "$resource_request" | \
            grep -B 9 "$resource_id")
        if [ -n "$result" ]; then
            has_result=true
            echo "In maestro server instance ${file%.log}" > $trace_log_file
            echo "$result" >> $trace_log_file
        fi
    fi
done
if ! $has_result; then
    echo "ERROR: Maestro server does not receive the spec request"
    exit 1
fi

# Maestro server publishes the CS spec request to mqtt broker
has_result=false
for file in maestro-*.log; do
    if [[ ! "$file" == maestro-agent-* ]]; then
        result=$(cat $file | grep -A 13 "Publishing resource $resource_id")
        if [ -n "$result" ]; then
            has_result=true
            echo "In maestro server instance ${file%.log}"  >> $trace_log_file
            echo "$result" >> $trace_log_file
        fi
    fi
done
if ! $has_result; then
    echo "ERROR: Maestro server does not publish the spec request"
    exit 1
fi

# Maestro agent subscribes to mqtt broker receiving the CS spec request
echo "Trace resource request on maestro agent ..."
has_result=false
for file in maestro-agent-*.log; do
    result=$(cat $file | grep -B 2 -A 9 "spec.$resource_request" | grep -B 10 "$resource_id")
    if [ -n "$result" ]; then
        has_result=true
        echo "In maestro agent ${file%.log}" >> $trace_log_file
        echo "$result" >> $trace_log_file
    fi
done
if ! $has_result; then
    echo "ERROR: Maestro agent does not receive the spec request"
    exit 1
fi

# Maestro agent publishes status update request to mqtt broker
has_result=false
for file in maestro-agent-*.log; do
    result=$(cat $file | grep "Receive the event $evt_type for $work_name")
    if [ -n "$result" ]; then
        has_result=true
        echo "In maestro agent ${file%.log}" >> $trace_log_file
        echo "$result" >> $trace_log_file
    fi
done
if ! $has_result; then
    echo "ERROR: Maestro agent does not handle the spec request"
    exit 1
fi

# Maestro server subscribes to mqtt broker receiving the status update request
has_result=false
for file in maestro-agent-*.log; do
    result=$(cat $file | grep -A 10 "Sending event" | grep -B 10 "$resource_id")
    if [ -n "$result" ]; then
        has_result=true
        echo "In maestro agent ${file%.log}" >> $trace_log_file
        echo "$result" >> $trace_log_file
    fi
done
if ! $has_result; then
    echo "ERROR: Maestro agent does not publish the status update request"
    exit 1
fi


echo "Trace resource status update request on maestro server ..."
has_result=false
for file in maestro-*.log; do
    if [[ ! "$file" == maestro-agent-* ]]; then
        result=$(cat $file | grep "received action STATUSMODIFIED" | grep $resource_id)
        if [ -n "$result" ]; then
            has_result=true
            echo "In maestro server instance ${file%.log}" >> $trace_log_file
            echo "$result" >> $trace_log_file
        fi
    fi
done
if ! $has_result; then
    echo "ERROR: Maestro server does not receive the status update request"
    exit 1
fi

for file in maestro-*.log; do
    if [[ ! "$file" == maestro-agent-* ]]; then
        result=$(cat $file | grep "skipping resource status update" | grep $resource_id)
        if [ -n "$result" ]; then
            has_result=true
            echo "In maestro server instance ${file%.log}" >> $trace_log_file
            echo "$result" >> $trace_log_file
        fi
    fi
done

has_result=false
for file in maestro-*.log; do
    if [[ ! "$file" == maestro-agent-* ]]; then
        result=$(cat $file | grep "resource $resource_id status $status_action event was sent")
        if [ -n "$result" ]; then
            has_result=true
            echo "In maestro server instance ${file%.log}" >> $trace_log_file
            echo "$result" >> $trace_log_file
        fi
    fi
done
if ! $has_result; then
    echo "ERROR: Maestro server does not handle the status update request"
    exit 1
fi

has_result=false
for file in maestro-*.log; do
    if [[ ! "$file" == maestro-agent-* ]]; then
        result=$(cat $file | grep "Broadcast the resource status" | grep $resource_id)
        if [ -n "$result" ]; then
            has_result=true
            echo "In maestro server instance ${file%.log}" >> $trace_log_file
            echo "$result" >> $trace_log_file
        fi
    fi
done
if ! $has_result; then
    echo "ERROR: Maestro server does not broadcast the status update request"
    exit 1
fi

# Maestro server send the status update to CS
has_result=false
for file in maestro-*.log; do
    if [[ ! "$file" == maestro-agent-* ]]; then
        result=$(cat $file | \
            grep -A 9 "INFO" | \
            grep -A 9 "send the event to status subscribers" | \
            grep -B 9 $resource_id)
        if [ -n "$result" ]; then
            has_result=true
            echo "In maestro server instance ${file%.log}" >> $trace_log_file
            echo "$result" >> $trace_log_file
        fi
    fi
done
if ! $has_result; then
    echo "ERROR: Maestro server does not publish the status update request"
    exit 1
fi

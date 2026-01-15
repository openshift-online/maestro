#!/bin/bash

# Trace resource requests through Maestro server -> agent -> server pipeline
# This script analyzes logs to track the complete lifecycle of a resource request

set -e

# Configuration
logs_dir=${logs_dir:-"$HOME/maestro-logs"}
resource_id=${resource_id:-""}
work_name=${work_name:-""}
op_id=${op_id:-""}
manifest_name=${manifest_name:-""}

# Output configuration
timestamp=$(date '+%Y-%m-%d_%H-%M-%S')
trace_log_file="${logs_dir}/trace_request.${timestamp}.log"

# Color output helpers
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Trace resource requests through the Maestro system using various identifiers.

Options:
    --resource-id <id>     Trace by resource ID (UUID format)
    --work-name <name>     Trace by work name
    --op-id <id>           Trace by operation ID
    --manifest-name <name> Trace by manifest name
    --logs-dir <path>      Directory containing maestro logs (default: $HOME/maestro-logs)
    --help                 Show this help message

Examples:
    $0 --resource-id 9936a444-051a-5658-9b57-af855e27b01b
    $0 --work-name nginx-work
    $0 --op-id 2dd9d768-a02d-4539-a99f-8a2c801a1c4b
    $0 --logs-dir /path/to/logs --resource-id <id>

EOF
    exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --resource-id)
            [[ -z "$2" || "$2" == --* ]] && { log_error "--resource-id requires a value"; usage; }
            resource_id="$2"
            shift 2
            ;;
        --work-name)
            [[ -z "$2" || "$2" == --* ]] && { log_error "--work-name requires a value"; usage; }
            work_name="$2"
            shift 2
            ;;
        --op-id)
            [[ -z "$2" || "$2" == --* ]] && { log_error "--op-id requires a value"; usage; }
            op_id="$2"
            shift 2
            ;;
        --manifest-name)
            [[ -z "$2" || "$2" == --* ]] && { log_error "--manifest-name requires a value"; usage; }
            manifest_name="$2"
            shift 2
            ;;
        --logs-dir)
            [[ -z "$2" || "$2" == --* ]] && { log_error "--logs-dir requires a value"; usage; }
            logs_dir="$2"
            shift 2
            ;;
        --help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Validate inputs
if [[ -z "$resource_id" && -z "$work_name" && -z "$op_id" && -z "$manifest_name" ]]; then
    log_error "At least one identifier must be provided (resource-id, work-name, op-id, or manifest-name)"
    usage
fi

if [[ ! -d "$logs_dir" ]]; then
    log_error "Logs directory not found: $logs_dir"
    exit 1
fi

# Print configuration
log_info "========================================"
log_info "Tracing Resource Request"
log_info "========================================"
log_info "Logs directory: $logs_dir"
[[ -n "$resource_id" ]] && log_info "Resource ID: $resource_id"
[[ -n "$work_name" ]] && log_info "Work name: $work_name"
[[ -n "$op_id" ]] && log_info "Operation ID: $op_id"
[[ -n "$manifest_name" ]] && log_info "Manifest name: $manifest_name"
log_info "Trace log: $trace_log_file"
log_info "========================================"

cd "$logs_dir" || exit 1
echo "# Maestro Resource Request Trace" > "$trace_log_file"
echo "Generated: $(date)" >> "$trace_log_file"
echo "" >> "$trace_log_file"

# Helper function to search logs
search_server_logs() {
    local pattern="$1"
    local description="$2"
    local found=false

    for file in maestro*.log; do
        # Skip agent logs
        if [[ "$file" == maestro-agent* ]] || [[ "$file" == maestro.agent* ]]; then
            continue
        fi

        result=$(grep "$pattern" "$file" 2>/dev/null || true)
        if [ -n "$result" ]; then
            found=true
            echo "" >> "$trace_log_file"
            echo "## $description" >> "$trace_log_file"
            echo "**File:** $file" >> "$trace_log_file"
            echo '```' >> "$trace_log_file"
            echo "$result" >> "$trace_log_file"
            echo '```' >> "$trace_log_file"
        fi
    done

    if ! $found; then
        log_warn "$description - No matching entries found"
        echo "" >> "$trace_log_file"
        echo "## $description" >> "$trace_log_file"
        echo "**Status:** ⚠️ No matching entries found" >> "$trace_log_file"
    else
        log_info "$description - Found"
    fi

    echo "$found"
}

search_agent_logs() {
    local pattern="$1"
    local description="$2"
    local found=false

    for file in maestro-agent*.log maestro.agent*.log; do
        if [[ ! -f "$file" ]]; then
            continue
        fi

        result=$(grep "$pattern" "$file" 2>/dev/null || true)
        if [ -n "$result" ]; then
            found=true
            echo "" >> "$trace_log_file"
            echo "## $description" >> "$trace_log_file"
            echo "**File:** $file" >> "$trace_log_file"
            echo '```' >> "$trace_log_file"
            echo "$result" >> "$trace_log_file"
            echo '```' >> "$trace_log_file"
        fi
    done

    if ! $found; then
        log_warn "$description - No matching entries found"
        echo "" >> "$trace_log_file"
        echo "## $description" >> "$trace_log_file"
        echo "**Status:** ⚠️ No matching entries found" >> "$trace_log_file"
    else
        log_info "$description - Found"
    fi

    echo "$found"
}

# Trace the request path
log_info "Starting trace analysis..."
echo "" >> "$trace_log_file"
echo "---" >> "$trace_log_file"
echo "" >> "$trace_log_file"

# 1. Server receives spec request from client
if [[ -n "$op_id" || -n "$resource_id" ]]; then
    log_info "[1/7] Checking server spec request reception..."
    if [[ -n "$op_id" ]]; then
        search_server_logs "receive the event from client.*op-id=\"$op_id\"" "1. Server Receives Spec Request (by op-id)"
    fi
    if [[ -n "$resource_id" ]]; then
        search_server_logs "receive the event from client.*resourceid=\"$resource_id\"" "1. Server Receives Spec Request (by resource-id)"
    fi
fi

# 2. Server publishes resource to message broker
if [[ -n "$resource_id" ]]; then
    log_info "[2/7] Checking server publishing to broker..."
    search_server_logs "Publishing resource.*resourceID=\"$resource_id\"" "2. Server Publishes to Message Broker"
    search_server_logs "Sending event.*resourceID=\"$resource_id\"" "2b. Server Sends CloudEvent"
fi

# 3. Agent receives spec request
if [[ -n "$resource_id" ]]; then
    log_info "[3/7] Checking agent spec request reception..."
    search_agent_logs "resourceid=\"$resource_id\".*Received event" "3. Agent Receives Spec Request"
fi

# 4. Agent handles/applies the spec
if [[ -n "$work_name" || -n "$manifest_name" ]]; then
    log_info "[4/7] Checking agent manifest handling..."
    if [[ -n "$work_name" ]]; then
        search_agent_logs "\"$work_name\"" "4. Agent Handles ManifestWork"
    fi
    if [[ -n "$manifest_name" ]]; then
        search_agent_logs "\"$manifest_name\"" "4b. Agent Handles Manifest"
    fi
fi

# 5. Agent publishes status update
if [[ -n "$resource_id" ]]; then
    log_info "[5/7] Checking agent status update publication..."
    search_agent_logs "resourceid=\"$resource_id\".*Sending event" "5. Agent Publishes Status Update"
fi

# 6. Server receives status update
if [[ -n "$resource_id" || -n "$op_id" ]]; then
    log_info "[6/7] Checking server status update reception..."
    if [[ -n "$resource_id" ]]; then
        search_server_logs "received status update.*resourceID=\"$resource_id\"" "6. Server Receives Status Update"
        search_server_logs "Updating resource status.*resourceID=\"$resource_id\"" "6b. Server Updates Resource Status"
    fi
    if [[ -n "$op_id" ]]; then
        search_server_logs "received status update.*op-id=\"$op_id\"" "6c. Server Receives Status Update (by op-id)"
    fi
fi

# 7. Server broadcasts and sends to subscribers
if [[ -n "$resource_id" || -n "$op_id" ]]; then
    log_info "[7/7] Checking server broadcast to clients..."
    if [[ -n "$resource_id" ]]; then
        search_server_logs "Broadcast the resource status.*resourceID=\"$resource_id\"" "7. Server Broadcasts Status"
        search_server_logs "send the event to status subscribers.*resourceID=\"$resource_id\"" "7b. Server Sends to gRPC Subscribers"
    fi
    if [[ -n "$op_id" ]]; then
        search_server_logs "send the event to status subscribers.*op-id=\"$op_id\"" "7c. Server Sends to Subscribers (by op-id)"
    fi
fi

log_info "========================================"
log_info "Trace analysis complete!"
log_info "Results saved to: $trace_log_file"
log_info "========================================"

# Check for common errors
echo "" >> "$trace_log_file"
echo "---" >> "$trace_log_file"
echo "" >> "$trace_log_file"
echo "## Error Analysis" >> "$trace_log_file"
echo "" >> "$trace_log_file"

has_errors=false

# Check for publish errors
if grep -q "Failed to publish resource" maestro*.log 2>/dev/null; then
    has_errors=true
    echo "- ⚠️ **Publish Error Detected**: Check for message broker connectivity issues" >> "$trace_log_file"
fi

# Check for database errors
if grep -q "failed to create status event" maestro*.log 2>/dev/null; then
    has_errors=true
    echo "- ⚠️ **Database Error Detected**: Status event creation failed" >> "$trace_log_file"
fi

# Check for consumer mismatch
if grep -q "unmatched consumer name" maestro*.log 2>/dev/null; then
    has_errors=true
    echo "- ⚠️ **Consumer Mismatch**: Agent consumer name doesn't match" >> "$trace_log_file"
fi

# Check for decode errors
if grep -q "failed to convert resource\|failed to decode cloudevent" maestro*.log 2>/dev/null; then
    has_errors=true
    echo "- ⚠️ **Decode Error**: Failed to convert/decode resource" >> "$trace_log_file"
fi

if ! $has_errors; then
    echo "- ✅ No common errors detected" >> "$trace_log_file"
fi

echo "" >> "$trace_log_file"
echo "For detailed error troubleshooting, see references/error_analysis.md" >> "$trace_log_file"

cat "$trace_log_file"

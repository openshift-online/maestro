#!/bin/bash

# Trace resource requests through Maestro server -> agent -> server pipeline
# This script analyzes Kusto CSV exports to track the complete lifecycle of a resource request
#
# This is the Kusto-specific version that works directly with CSV exports from Azure Kusto
# For standard log files from Kubernetes, use trace_request.sh instead

set -e

# Configuration
server_csv=""
agent_csv=""
resource_id=${resource_id:-""}
work_name=${work_name:-""}
op_id=${op_id:-""}
manifest_name=${manifest_name:-""}

# Output configuration
timestamp=$(date '+%Y-%m-%d_%H-%M-%S')
output_dir=${output_dir:-"."}
trace_log_file="${output_dir}/trace_request.${timestamp}.log"

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

Trace resource requests through the Maestro system using Kusto CSV exports.
This script works directly with CSV files exported from Azure Kusto.

Options:
    --server-csv <file>    Path to server logs CSV file from Kusto
    --agent-csv <file>     Path to agent logs CSV file from Kusto
    --resource-id <id>     Trace by resource ID (UUID format)
    --work-name <name>     Trace by work name
    --op-id <id>           Trace by operation ID (recommended)
    --manifest-name <name> Trace by manifest name
    --output-dir <path>    Directory for trace output (default: current directory)
    --help                 Show this help message

CSV Format:
    Kusto exports should have 3 columns: TIMESTAMP, pod_name, log
    Use the Kusto queries documented in SKILL.md to export logs

Examples:
    # Trace by operation ID (most comprehensive)
    $0 --server-csv export.svc.csv --agent-csv export.agent.csv \\
       --op-id 2dd9d768-a02d-4539-a99f-8a2c801a1c4b

    # Trace by resource ID
    $0 --server-csv export.svc.csv --agent-csv export.agent.csv \\
       --resource-id 9936a444-051a-5658-9b57-af855e27b01b

    # Server logs only
    $0 --server-csv export.svc.csv --op-id <op-id>

    # Save output to specific directory
    $0 --server-csv export.svc.csv --agent-csv export.agent.csv \\
       --op-id <op-id> --output-dir /tmp/traces

EOF
    exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --server-csv)
            [[ -z "$2" || "$2" == --* ]] && { log_error "--server-csv requires a value"; usage; }
            server_csv="$2"
            shift 2
            ;;
        --agent-csv)
            [[ -z "$2" || "$2" == --* ]] && { log_error "--agent-csv requires a value"; usage; }
            agent_csv="$2"
            shift 2
            ;;
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
        --output-dir)
            [[ -z "$2" || "$2" == --* ]] && { log_error "--output-dir requires a value"; usage; }
            output_dir="$2"
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

if [[ -z "$server_csv" && -z "$agent_csv" ]]; then
    log_error "At least one CSV file must be provided (--server-csv or --agent-csv)"
    usage
fi

if [[ -n "$server_csv" && ! -f "$server_csv" ]]; then
    log_error "Server CSV file not found: $server_csv"
    exit 1
fi

if [[ -n "$agent_csv" && ! -f "$agent_csv" ]]; then
    log_error "Agent CSV file not found: $agent_csv"
    exit 1
fi

# Create output directory if needed
mkdir -p "$output_dir"
trace_log_file="${output_dir}/trace_request.${timestamp}.log"

# Print configuration
log_info "========================================"
log_info "Tracing Resource Request (Kusto CSV)"
log_info "========================================"
[[ -n "$server_csv" ]] && log_info "Server CSV: $server_csv"
[[ -n "$agent_csv" ]] && log_info "Agent CSV: $agent_csv"
[[ -n "$resource_id" ]] && log_info "Resource ID: $resource_id"
[[ -n "$work_name" ]] && log_info "Work name: $work_name"
[[ -n "$op_id" ]] && log_info "Operation ID: $op_id"
[[ -n "$manifest_name" ]] && log_info "Manifest name: $manifest_name"
log_info "Trace log: $trace_log_file"
log_info "========================================"

echo "# Maestro Resource Request Trace (Kusto)" > "$trace_log_file"
echo "Generated: $(date)" >> "$trace_log_file"
echo "" >> "$trace_log_file"

# Helper function to extract log column from CSV and search
# CSV format: "TIMESTAMP","pod_name","log"
# We need to extract the 3rd column (log) and search it
search_csv_logs() {
    local csv_file="$1"
    local pattern="$2"
    local description="$3"
    local found=false

    if [[ -z "$csv_file" || ! -f "$csv_file" ]]; then
        return
    fi

    # Extract log column (3rd column) from CSV, skip header, and search for pattern
    # Using awk to properly handle CSV quoted fields
    result=$(awk -F'","' 'NR>1 {
        # Remove leading quote from first field and trailing quote from last field
        gsub(/^"/, "", $1)
        gsub(/"$/, "", $3)
        # Print only the log column (3rd column)
        print $3
    }' "$csv_file" | grep "$pattern" 2>/dev/null || true)

    if [ -n "$result" ]; then
        found=true
        echo "" >> "$trace_log_file"
        echo "## $description" >> "$trace_log_file"
        echo "**File:** $(basename "$csv_file")" >> "$trace_log_file"
        echo '```' >> "$trace_log_file"
        echo "$result" >> "$trace_log_file"
        echo '```' >> "$trace_log_file"
    fi

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
        search_csv_logs "$server_csv" "receive the event from client.*op-id=\"\"$op_id\"\"" "1. Server Receives Spec Request (by op-id)"
    fi
    if [[ -n "$resource_id" ]]; then
        search_csv_logs "$server_csv" "receive the event from client.*resourceid=\"\"$resource_id\"\"" "1. Server Receives Spec Request (by resource-id)"
    fi
fi

# 2. Server publishes resource to message broker
if [[ -n "$resource_id" ]]; then
    log_info "[2/7] Checking server publishing to broker..."
    search_csv_logs "$server_csv" "Publishing resource.*resourceID=\"\"$resource_id\"\"" "2. Server Publishes to Message Broker"
    search_csv_logs "$server_csv" "Sending event.*resourceID=\"\"$resource_id\"\"" "2b. Server Sends CloudEvent"
fi

# 3. Agent receives spec request
if [[ -n "$resource_id" ]]; then
    log_info "[3/7] Checking agent spec request reception..."
    search_csv_logs "$agent_csv" "resourceid=\"\"$resource_id\"\".*Received event" "3. Agent Receives Spec Request"
fi

# 4. Agent handles/applies the spec
if [[ -n "$work_name" || -n "$manifest_name" ]]; then
    log_info "[4/7] Checking agent manifest handling..."
    if [[ -n "$work_name" ]]; then
        search_csv_logs "$agent_csv" "\"\"$work_name\"\"" "4. Agent Handles ManifestWork"
    fi
    if [[ -n "$manifest_name" ]]; then
        search_csv_logs "$agent_csv" "\"\"$manifest_name\"\"" "4b. Agent Handles Manifest"
    fi
fi

# 5. Agent publishes status update
if [[ -n "$resource_id" ]]; then
    log_info "[5/7] Checking agent status update publication..."
    search_csv_logs "$agent_csv" "resourceid=\"\"$resource_id\"\".*Sending event" "5. Agent Publishes Status Update"
fi

# 6. Server receives status update
if [[ -n "$resource_id" || -n "$op_id" ]]; then
    log_info "[6/7] Checking server status update reception..."
    if [[ -n "$resource_id" ]]; then
        search_csv_logs "$server_csv" "received status update.*resourceID=\"\"$resource_id\"\"" "6. Server Receives Status Update"
        search_csv_logs "$server_csv" "Updating resource status.*resourceID=\"\"$resource_id\"\"" "6b. Server Updates Resource Status"
    fi
    if [[ -n "$op_id" ]]; then
        search_csv_logs "$server_csv" "received status update.*op-id=\"\"$op_id\"\"" "6c. Server Receives Status Update (by op-id)"
    fi
fi

# 7. Server broadcasts and sends to subscribers
if [[ -n "$resource_id" || -n "$op_id" ]]; then
    log_info "[7/7] Checking server broadcast to clients..."
    if [[ -n "$resource_id" ]]; then
        search_csv_logs "$server_csv" "Broadcast the resource status.*resourceID=\"\"$resource_id\"\"" "7. Server Broadcasts Status"
        search_csv_logs "$server_csv" "send the event to status subscribers.*resourceID=\"\"$resource_id\"\"" "7b. Server Sends to gRPC Subscribers"
    fi
    if [[ -n "$op_id" ]]; then
        search_csv_logs "$server_csv" "send the event to status subscribers.*op-id=\"\"$op_id\"\"" "7c. Server Sends to Subscribers (by op-id)"
    fi
fi

log_info "========================================"
log_info "Trace analysis complete!"
log_info "Results saved to: $trace_log_file"
log_info "========================================"

# Check for common errors (only in files that were provided)
echo "" >> "$trace_log_file"
echo "---" >> "$trace_log_file"
echo "" >> "$trace_log_file"
echo "## Error Analysis" >> "$trace_log_file"
echo "" >> "$trace_log_file"

has_errors=false

check_error_in_csv() {
    local csv_file="$1"
    local pattern="$2"

    if [[ -z "$csv_file" || ! -f "$csv_file" ]]; then
        return 1
    fi

    awk -F'","' 'NR>1 {
        gsub(/^"/, "", $1)
        gsub(/"$/, "", $3)
        print $3
    }' "$csv_file" | grep -q "$pattern" 2>/dev/null
}

# Check for publish errors
if [[ -n "$server_csv" ]] && check_error_in_csv "$server_csv" "Failed to publish resource"; then
    has_errors=true
    echo "- ⚠️ **Publish Error Detected**: Check for message broker connectivity issues" >> "$trace_log_file"
fi

# Check for database errors
if [[ -n "$server_csv" ]] && check_error_in_csv "$server_csv" "failed to create status event"; then
    has_errors=true
    echo "- ⚠️ **Database Error Detected**: Status event creation failed" >> "$trace_log_file"
fi

# Check for consumer mismatch
if [[ -n "$server_csv" ]] && check_error_in_csv "$server_csv" "unmatched consumer name"; then
    has_errors=true
    echo "- ⚠️ **Consumer Mismatch**: Agent consumer name doesn't match" >> "$trace_log_file"
fi

# Check for decode errors
if [[ -n "$server_csv" ]] && check_error_in_csv "$server_csv" "failed to convert resource\|failed to decode cloudevent"; then
    has_errors=true
    echo "- ⚠️ **Decode Error**: Failed to convert/decode resource" >> "$trace_log_file"
fi

if ! $has_errors; then
    echo "- ✅ No common errors detected" >> "$trace_log_file"
fi

echo "" >> "$trace_log_file"
echo "For detailed error troubleshooting, see references/error_analysis.md" >> "$trace_log_file"

cat "$trace_log_file"

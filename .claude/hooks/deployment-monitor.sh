#!/bin/bash
#
# Deployment Monitor Hook
# This hook monitors long-running deployment processes and notifies when complete
#
# Usage: Can be called after triggering a deployment to monitor its progress
#
# Dependencies:
# - Required: bash, wc, tail, sed, grep, cat, tr, date, sleep
# - Optional: curl (for Slack notifications), osascript (for macOS notifications), notify-send (for Linux notifications)
#
# Configuration:
# Set SLACK_WEBHOOK_URL environment variable for Slack notifications

set -e

HOOK_NAME="deployment-monitor"

# Configuration is loaded from environment variables:
# - SLACK_WEBHOOK_URL: Optional Slack webhook URL for notifications

# Function to check if required commands are available
check_command() {
    local cmd=$1
    local required=$2

    if ! command -v "$cmd" &> /dev/null; then
        if [ "$required" = "true" ]; then
            echo "[$HOOK_NAME] ERROR: Required command '$cmd' is not installed"
            return 1
        else
            echo "[$HOOK_NAME] WARNING: Optional command '$cmd' is not installed"
            return 0
        fi
    fi
    return 0
}

# Function to monitor deployment
monitor_deployment() {
    local task_id=$1

    if [ -z "$task_id" ]; then
        echo "[$HOOK_NAME] ERROR: Task ID required"
        echo "Usage: $0 monitor <task_id>"
        exit 1
    fi

    # Build task output paths dynamically based on current working directory
    local cwd_sanitized
    cwd_sanitized=$(pwd | tr '/' '-' | sed 's/^-//')
    local task_dir="/tmp/claude/-${cwd_sanitized}/tasks"

    # Ensure task directory exists
    if [ ! -d "$task_dir" ]; then
        echo "[$HOOK_NAME] WARNING: Task directory does not exist: $task_dir"
        echo "[$HOOK_NAME] Creating directory..."
        mkdir -p "$task_dir"
    fi

    local output_file="${task_dir}/${task_id}.output"
    local exit_code_file="${task_dir}/${task_id}.exit_code"
    local start_time
    start_time=$(date +%s)

    echo "[$HOOK_NAME] Monitoring deployment task: $task_id"
    echo "[$HOOK_NAME] Started at: $(date)"
    echo ""

    # Wait for the deployment to complete
    local last_line_count=0
    local max_wait_seconds=${MONITOR_TIMEOUT:-7200}  # Default 2 hours
    while true; do
        # Check for timeout
        local elapsed=$(($(date +%s) - start_time))
        if [ "$elapsed" -ge "$max_wait_seconds" ]; then
            local minutes=$((max_wait_seconds / 60))
            echo ""
            echo "[$HOOK_NAME] ERROR: Maximum wait time (${minutes} minutes) reached"
            notify_completion "FAILED" "Deployment monitoring timed out after ${minutes} minutes without completion"
            return 2
        fi

        # Check if exit code file exists (task completed)
        if [ -f "$exit_code_file" ]; then
            local exit_code
            exit_code=$(cat "$exit_code_file")
            echo "[$HOOK_NAME] Deployment process finished with exit code: $exit_code"
            break
        fi

        # Show progress if output file exists
        if [ -f "$output_file" ]; then
            local current_lines
            current_lines=$(wc -l < "$output_file" | tr -d ' ')
            if [ "$current_lines" != "$last_line_count" ]; then
                local elapsed=$(($(date +%s) - start_time))
                local minutes=$((elapsed / 60))
                local seconds=$((elapsed % 60))
                echo "[$HOOK_NAME] Progress: $current_lines lines | Elapsed: ${minutes}m ${seconds}s | $(date +%H:%M:%S)"

                # Show latest activity
                tail -3 "$output_file" | sed 's/\x1b\[[0-9;]*m//g' | grep -v "^$" | tail -1 | sed "s/^/[$HOOK_NAME]   Latest: /"

                last_line_count=$current_lines
            fi
        fi

        # Sleep before next check
        sleep 15
    done

    # Calculate total time
    local end_time
    end_time=$(date +%s)
    local total_time=$((end_time - start_time))
    local minutes=$((total_time / 60))
    local seconds=$((total_time % 60))

    # Determine status and send notification
    if [ "$exit_code" -eq 0 ]; then
        notify_completion "COMPLETE" "Maestro cluster deployment completed successfully in ${minutes}m ${seconds}s!"
        echo ""
        echo "[$HOOK_NAME] Total deployment time: ${minutes}m ${seconds}s"
        echo "[$HOOK_NAME] Output file: $output_file"
        return 0
    else
        notify_completion "FAILED" "Deployment failed with exit code $exit_code after ${minutes}m ${seconds}s"
        echo ""
        echo "[$HOOK_NAME] Total deployment time: ${minutes}m ${seconds}s"
        echo "[$HOOK_NAME] Output file: $output_file"
        return 1
    fi
}

# Function to send Slack notification
send_slack_notification() {
    local status=$1
    local message=$2
    local webhook_url=$3

    if [ -z "$webhook_url" ]; then
        return 1
    fi

    # Check if curl is available
    if ! check_command "curl" "false"; then
        echo "[$HOOK_NAME] Skipping Slack notification - curl not available"
        return 1
    fi

    # Determine color based on status
    local color="good"
    local emoji=":white_check_mark:"
    if [[ "$status" == "FAILED" ]]; then
        color="danger"
        emoji=":x:"
    elif [[ "$status" == "COMPLETE" ]]; then
        color="good"
        emoji=":white_check_mark:"
    fi

    # Create JSON payload using jq for safe escaping
    local payload
    if command -v jq &> /dev/null; then
        # Use jq for safe JSON construction
        payload=$(jq -n \
            --arg color "$color" \
            --arg title "$emoji Maestro Deployment $status" \
            --arg text "$message" \
            --arg footer "Maestro Deployment Monitor" \
            --argjson ts "$(date +%s)" \
            '{attachments: [{color: $color, title: $title, text: $text, footer: $footer, ts: $ts}]}')
    elif command -v python3 &> /dev/null; then
        # Fallback: Use Python for proper JSON encoding
        payload=$(python3 -c "import json, sys; print(json.dumps({'attachments': [{'color': sys.argv[1], 'title': sys.argv[2] + ' Maestro Deployment ' + sys.argv[3], 'text': sys.argv[4], 'footer': 'Maestro Deployment Monitor', 'ts': int(sys.argv[5])}]}))" "$color" "$emoji" "$status" "$message" "$(date +%s)")
    else
        # Last resort: Extended manual escaping for all control characters
        local escaped_message="${message//\\/\\\\}"      # Escape backslashes
        escaped_message="${escaped_message//\"/\\\"}"    # Escape quotes
        escaped_message="${escaped_message//$'\n'/\\n}"  # Escape newlines
        escaped_message="${escaped_message//$'\r'/\\r}"  # Escape carriage returns
        escaped_message="${escaped_message//$'\t'/\\t}"  # Escape tabs

        local escaped_status="${status//\\/\\\\}"
        escaped_status="${escaped_status//\"/\\\"}"
        escaped_status="${escaped_status//$'\n'/\\n}"
        escaped_status="${escaped_status//$'\r'/\\r}"
        escaped_status="${escaped_status//$'\t'/\\t}"

        payload=$(cat <<EOF
{
  "attachments": [
    {
      "color": "$color",
      "title": "$emoji Maestro Deployment $escaped_status",
      "text": "$escaped_message",
      "footer": "Maestro Deployment Monitor",
      "ts": $(date +%s)
    }
  ]
}
EOF
)
    fi

    # Send to Slack and capture exit status
    # --fail ensures curl returns non-zero on HTTP 4xx/5xx errors
    local curl_exit_code
    if curl -X POST -H 'Content-type: application/json' \
        --data "$payload" \
        "$webhook_url" \
        --silent --show-error --fail; then
        curl_exit_code=0
    else
        curl_exit_code=$?
        echo "[$HOOK_NAME] ERROR: Failed to send Slack notification (curl exit code: $curl_exit_code)"
    fi

    return $curl_exit_code
}

# Function to send notification
notify_completion() {
    local status=$1
    local message=$2

    echo ""
    echo "=========================================="
    echo "[$HOOK_NAME] DEPLOYMENT $status"
    echo "Message: $message"
    echo "Time: $(date)"
    echo "=========================================="
    echo ""

    # Send Slack notification if webhook is configured
    if [ -n "$SLACK_WEBHOOK_URL" ]; then
        echo "[$HOOK_NAME] Sending Slack notification..."
        if send_slack_notification "$status" "$message" "$SLACK_WEBHOOK_URL"; then
            echo "[$HOOK_NAME] Slack notification sent successfully"
        else
            echo "[$HOOK_NAME] Failed to send Slack notification"
        fi
    fi

    # Also send system notification if available
    if command -v osascript &> /dev/null; then
        # macOS notification - escape message for AppleScript
        local safe_message="${message//\\/\\\\}"
        safe_message="${safe_message//\"/\\\"}"
        osascript -e "display notification \"$safe_message\" with title \"Maestro Deployment $status\""
    elif command -v notify-send &> /dev/null; then
        # Linux notification - use safe argument passing
        notify-send -- "Maestro Deployment $status" "$message"
    fi
}

# Main execution
case "${1:-notify}" in
    monitor)
        monitor_deployment "$2"
        exit $?
        ;;
    notify)
        notify_completion "${2:-COMPLETE}" "${3:-Deployment finished}"
        ;;
    *)
        echo "Usage: $0 {monitor <task_id>|notify <status> <message>}"
        exit 1
        ;;
esac

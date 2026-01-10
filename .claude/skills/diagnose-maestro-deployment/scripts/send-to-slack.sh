#!/bin/bash
set -e

REPORT_FILE="$1"
WEBHOOK_URL="${SLACK_WEBHOOK_URL}"

if [ -z "$REPORT_FILE" ] || [ ! -f "$REPORT_FILE" ]; then
    echo "ERROR: Report file not provided or does not exist"
    echo "Usage: $0 <report-file>"
    exit 1
fi

if [ -z "$WEBHOOK_URL" ]; then
    echo "ERROR: SLACK_WEBHOOK_URL not set"
    exit 1
fi

# Extract key information from report
TOTAL_ISSUES=$(grep "^Total Issues:" "$REPORT_FILE" | awk '{print $3}')
CRITICAL_ISSUES=$(grep "^Critical Issues:" "$REPORT_FILE" | awk '{print $3}')
OVERALL_STATUS=$(grep "^Overall Status:" "$REPORT_FILE" | sed 's/Overall Status: //')
ACTION_REQUIRED=$(grep "^Action Required:" "$REPORT_FILE" | sed 's/Action Required: //')

# Extract cluster info
SVC_CLUSTER=$(grep "Service:" "$REPORT_FILE" | head -1 | awk '{print $2"/"$3}')
MGMT_CLUSTER=$(grep "Management:" "$REPORT_FILE" | head -1 | awk '{print $2"/"$3}')

# Determine color based on critical issues
if [ "$CRITICAL_ISSUES" = "0" ]; then
    COLOR="warning"
    EMOJI="⚠️"
    STATUS_ICON="⚠️"
else
    COLOR="danger"
    EMOJI="🚨"
    STATUS_ICON="❌"
fi

# Extract primary failure reason - just the first line
PRIMARY_REASON=$(grep "^Primary Failure:" "$REPORT_FILE" | sed 's/Primary Failure: //' | head -1)

# Extract conflict fields - clean format
CONFLICT_FIELDS=$(sed -n '/^Conflicting Fields:/,/^$/p' "$REPORT_FILE" 2>/dev/null | grep "•" | sed 's/  • //' | tr '\n' ',' | sed 's/,$//' | sed 's/,/, /g')

# Extract cascading failure if exists
CASCADING=$(grep "^Cascading Failure:" "$REPORT_FILE" | sed 's/Cascading Failure: //' | head -1)

# Build issue fields array - each issue gets its own field with detailed info
ISSUE_FIELDS="[]"
issue_num=1
while IFS= read -r line; do
    if [[ "$line" =~ ^\[([0-9]+)\]\ (.+)$ ]]; then
        issue_title="${BASH_REMATCH[2]}"

        # Get full issue section
        issue_section=$(sed -n "/^\[$issue_num\]/,/^\[/p" "$REPORT_FILE")
        severity=$(echo "$issue_section" | grep "Severity:" | awk '{print $2}' | head -1)

        # Determine emoji based on severity
        if [ "$severity" = "CRITICAL" ]; then
            sev_emoji="🔴"
        else
            sev_emoji="🟡"
        fi

        # Build structured issue description based on issue type
        issue_value=""

        if [[ "$issue_title" =~ "Hypershift" ]]; then
            # Hypershift issue - show clear cause and effect
            issue_value="*Root Cause:*"
            issue_value+=$'\n'"• Hypershift release post-install hook attempted to create ClusterSizingConfiguration resource"
            issue_value+=$'\n'"• Resource was already created and managed by hypershift-operator-manager"

            # Get specific conflicting fields
            specific_conflicts=$(echo "$issue_section" | sed -n '/Specific Conflicting Fields:/,/^    $/p' | grep "•" | sed 's/^      • //')
            if [ -n "$specific_conflicts" ]; then
                issue_value+=$'\n'"• Leading to field conflicts:"
                while IFS= read -r field; do
                    issue_value+=$'\n'"  - $field"
                done <<< "$specific_conflicts"
            fi
            issue_value+=$'\n'"• Helm marked the release as failed due to post-install hook failure"

            # Get actual status
            issue_value+=$'\n\n'"*Actual Status:*"
            actual_status=$(echo "$issue_section" | sed -n '/Actual Service Status:/,/^    $/p' | grep "✓" | sed 's/^      //')
            if [ -n "$actual_status" ]; then
                while IFS= read -r line; do
                    issue_value+=$'\n'"$line"
                done <<< "$actual_status"
            fi

            issue_value+=$'\n\n'"*Conclusion:* Although Helm status is failed, services are actually running normally. This is a Helm hook timing issue."

        elif [[ "$issue_title" =~ "MCE" ]]; then
            # MCE issue - similar structure
            root_cause=$(echo "$issue_section" | sed -n '/Root Cause:/,/^    $/p' | grep -v "Root Cause:" | grep -v "^    $" | sed 's/^      //' | head -1)
            issue_value="*Root Cause:*"$'\n'"• $root_cause"

            # Get actual status
            issue_value+=$'\n\n'"*Actual Status:*"
            actual_status=$(echo "$issue_section" | sed -n '/Actual Service Status:/,/^    $/p' | grep "✓" | sed 's/^      //')
            if [ -n "$actual_status" ]; then
                while IFS= read -r line; do
                    issue_value+=$'\n'"$line"
                done <<< "$actual_status"
            fi

            issue_value+=$'\n\n'"*Conclusion:* MCE services are running normally, Helm failure can be ignored."

        elif [[ "$issue_title" =~ "Maestro Not Deployed" ]]; then
            # Maestro not deployed - show cascading failure
            issue_value="*Root Cause (Cascading Failure):*"
            what_happened=$(echo "$issue_section" | sed -n '/What Happened:/,/^    $/p' | grep -v "What Happened:" | grep -v "^    $" | sed 's/^      //' | sed 's/^[0-9]\. /• /')
            if [ -n "$what_happened" ]; then
                while IFS= read -r line; do
                    issue_value+=$'\n'"$line"
                done <<< "$what_happened"
            fi

            # Get impact
            issue_value+=$'\n\n'"*Impact:*"
            impact=$(echo "$issue_section" | sed -n '/Impact:/,/^    $/p' | grep "✗" | sed 's/^      //')
            if [ -n "$impact" ]; then
                while IFS= read -r line; do
                    issue_value+=$'\n'"$line"
                done <<< "$impact"
            fi

            issue_value+=$'\n\n'"*Conclusion:* Service cluster deployment incomplete, manual intervention required."
        fi

        # Add to issues array
        if command -v jq &> /dev/null; then
            issue_field=$(jq -n \
                --arg title "$sev_emoji Issue $issue_num: $issue_title" \
                --arg value "$issue_value" \
                '{
                    title: $title,
                    value: $value,
                    short: false
                }')
            ISSUE_FIELDS=$(echo "$ISSUE_FIELDS" | jq --argjson field "$issue_field" '. += [$field]')
        fi

        issue_num=$((issue_num + 1))
    fi
done < <(grep -E '^\[[0-9]+\]' "$REPORT_FILE")

# Build clean, simple message using Slack fields format
if command -v jq &> /dev/null; then
    # Build base fields first
    BASE_FIELDS=$(jq -n \
        --arg status "$OVERALL_STATUS" \
        --arg total "$TOTAL_ISSUES" \
        --arg critical "$CRITICAL_ISSUES" \
        --arg svc "$SVC_CLUSTER" \
        --arg mgmt "$MGMT_CLUSTER" \
        --arg primary "$PRIMARY_REASON" \
        --arg conflicts "$CONFLICT_FIELDS" \
        --arg cascading "$CASCADING" \
        --arg action "$ACTION_REQUIRED" \
        '[
            {
                title: "Status",
                value: $status,
                short: true
            },
            {
                title: "Issues",
                value: ("Total: " + $total + " | Critical: " + $critical),
                short: true
            },
            {
                title: "Service Cluster",
                value: $svc,
                short: true
            },
            {
                title: "Management Cluster",
                value: $mgmt,
                short: true
            },
            {
                title: "Primary Failure",
                value: $primary,
                short: false
            },
            (if $conflicts != "" then {
                title: "Conflicting Fields",
                value: $conflicts,
                short: false
            } else empty end),
            (if $cascading != "" then {
                title: "Cascading Impact",
                value: $cascading,
                short: false
            } else empty end)
        ]')

    # Combine base fields with issue fields and action
    ALL_FIELDS=$(echo "$BASE_FIELDS $ISSUE_FIELDS" | jq -s '.[0] + .[1] + [{title: "Action Required", value: $action, short: false}]' --arg action "$ACTION_REQUIRED")

    # Build final payload
    PAYLOAD=$(jq -n \
        --arg color "$COLOR" \
        --arg title "$EMOJI Maestro Deployment Diagnostic" \
        --argjson fields "$ALL_FIELDS" \
        --argjson ts "$(date +%s)" \
        '{
            attachments: [{
                color: $color,
                title: $title,
                fields: $fields,
                footer: "Maestro Diagnostic Tool",
                ts: $ts,
                mrkdwn_in: ["fields"]
            }]
        }')
elif command -v python3 &> /dev/null; then
    # Fallback to simple format if jq not available
    MESSAGE="*$EMOJI Maestro Deployment Diagnosis*\n\n"
    MESSAGE+="*Status:* $STATUS_ICON $OVERALL_STATUS\n"
    MESSAGE+="*Action Required:* $ACTION_REQUIRED\n"
    MESSAGE+="*Total Issues:* \`$TOTAL_ISSUES\` | *Critical:* \`$CRITICAL_ISSUES\`\n\n"
    MESSAGE+="*Clusters:*\n• Service: \`$SVC_CLUSTER\`\n• Management: \`$MGMT_CLUSTER\`\n\n"
    MESSAGE+="See full diagnostic report for details."

    PAYLOAD=$(python3 -c "
import json, sys
payload = {
    'attachments': [{
        'color': sys.argv[1],
        'text': sys.argv[2],
        'footer': 'Maestro Diagnostic Tool',
        'ts': int(sys.argv[3]),
        'mrkdwn_in': ['text']
    }]
}
print(json.dumps(payload))
" "$COLOR" "$MESSAGE" "$(date +%s)")
else
    echo "ERROR: jq or python3 required for JSON construction"
    exit 1
fi

# Send to Slack
echo "Sending diagnostic report to Slack..."
if curl -X POST -H 'Content-type: application/json' \
    --data "$PAYLOAD" \
    "$WEBHOOK_URL" \
    --silent --show-error --fail; then
    echo ""
    echo "✓ Diagnostic report sent to Slack successfully"
    exit 0
else
    echo ""
    echo "✗ Failed to send diagnostic report to Slack"
    exit 1
fi

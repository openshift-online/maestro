#!/bin/bash
# Log Analysis Module for Maestro Deployment Diagnostics
# This module intelligently parses deployment logs to identify issues

set -e

# Extract failed Helm releases from deployment logs
extract_failed_helm_releases() {
    local log_file=$1
    local temp_dir=$2

    # Look for Helm deployment errors in logs
    grep -i "failed to deploy helm release\|helm release.*failed\|error running Helm" "$log_file" 2>/dev/null | \
        grep -o "helm release: [a-zA-Z0-9-]*\|release [a-zA-Z0-9-]*" | \
        awk '{print $NF}' | \
        sort -u > "$temp_dir/failed_helm_releases.txt" || true

    # Also extract from error messages with release names
    grep -oP '(?<=aro-hcp-)[a-zA-Z0-9-]+(?=/templates)' "$log_file" 2>/dev/null | \
        sort -u >> "$temp_dir/failed_helm_releases.txt" || true
}

# Extract resource conflicts from logs
extract_resource_conflicts() {
    local log_file=$1
    local temp_dir=$2

    # Look for resource conflict errors
    if grep -q "Apply failed with.*conflicts\|conflict occurred while applying" "$log_file"; then
        python3 -c "
import re
import sys

try:
    with open('$log_file', 'r') as f:
        content = f.read()

    conflicts = {}

    # Find conflict patterns
    for line in content.split('\n'):
        if 'Apply failed with' in line or 'conflict occurred while applying' in line:
            # Extract resource type and name
            resource_match = re.search(r'(?:object|resource)\s+/([a-zA-Z0-9-]+)\s+([a-zA-Z0-9./]+)', line)
            if resource_match:
                resource_name = resource_match.group(1)
                resource_type = resource_match.group(2)

                # Extract conflicting fields
                fields = []

                # Pattern 1: Field paths in error message
                field_matches = re.findall(r'\.spec\.[a-zA-Z0-9.\[\]=\"]+', line)
                fields.extend(field_matches)

                # Pattern 2: conflicts with manager
                manager_match = re.search(r'conflicts with\\\\\"([^\\\\]+)\\\\\"', line)
                manager = manager_match.group(1) if manager_match else 'unknown'

                if resource_name not in conflicts:
                    conflicts[resource_name] = {
                        'type': resource_type,
                        'fields': set(),
                        'manager': manager
                    }

                conflicts[resource_name]['fields'].update(fields)

    # Output conflicts in structured format
    for resource, info in conflicts.items():
        print(f'CONFLICT:{resource}:{info[\"type\"]}:{info[\"manager\"]}:{\"|\".join(sorted(info[\"fields\"]))}')

except Exception as e:
    print(f'ERROR: Failed to parse conflicts: {e}', file=sys.stderr)
" > "$temp_dir/resource_conflicts.txt" 2>/dev/null || echo "ERROR:parse_failed" > "$temp_dir/resource_conflicts.txt"
    fi
}

# Extract deployment timeline from logs
extract_deployment_timeline() {
    local log_file=$1
    local temp_dir=$2

    # Extract timestamped events
    grep -E '^\[?[0-9]{2}:[0-9]{2}:[0-9]{2}' "$log_file" | \
        grep -i "error\|failed\|success\|complete\|deployed\|installing" | \
        tail -50 > "$temp_dir/timeline.txt" || true
}

# Identify root cause from error patterns
identify_root_cause() {
    local log_file=$1
    local temp_dir=$2

    # Common error patterns and their interpretations
    python3 -c "
import re

error_patterns = {
    'timing_conflict': r'conflict occurred while applying.*hook',
    'resource_exists': r'already exists',
    'timeout': r'context (deadline exceeded|canceled)|timed? out',
    'authentication': r'authentication|unauthorized|forbidden',
    'network': r'connection refused|network.*unreachable|dial tcp',
    'resource_limit': r'(insufficient|exceeded).*resources',
    'dependency_missing': r'not found.*required|missing.*dependency',
    'api_error': r'Internal error occurred|API.*error',
    'helm_hook_failed': r'Hook.*failed|post-install.*failed',
}

with open('$log_file', 'r') as f:
    content = f.read()

detected_patterns = []
for pattern_name, pattern_regex in error_patterns.items():
    if re.search(pattern_regex, content, re.IGNORECASE):
        detected_patterns.append(pattern_name)

        # Find specific error context
        matches = re.finditer(pattern_regex, content, re.IGNORECASE)
        for match in list(matches)[:3]:  # Limit to first 3
            start = max(0, match.start() - 200)
            end = min(len(content), match.end() + 200)
            context = content[start:end].replace('\n', ' ')
            print(f'{pattern_name}:::{context}')
" > "$temp_dir/error_patterns.txt" 2>/dev/null || true
}

# Extract component status from logs
extract_component_status() {
    local log_file=$1
    local temp_dir=$2

    # Look for explicit status messages
    grep -i "status.*complete\|deployment.*success\|installed.*successfully" "$log_file" | \
        tail -20 > "$temp_dir/success_components.txt" || true

    grep -i "status.*fail\|deployment.*fail\|installation.*fail" "$log_file" | \
        tail -20 > "$temp_dir/failed_components.txt" || true
}

# Main analysis function
analyze_deployment_logs() {
    local log_file=$1
    local output_dir=$2

    if [ ! -f "$log_file" ]; then
        echo "ERROR: Log file not found: $log_file"
        return 1
    fi

    mkdir -p "$output_dir"

    echo "Analyzing deployment logs: $log_file"
    echo "Output directory: $output_dir"
    echo ""

    # Run all analysis functions
    extract_failed_helm_releases "$log_file" "$output_dir"
    extract_resource_conflicts "$log_file" "$output_dir"
    extract_deployment_timeline "$log_file" "$output_dir"
    identify_root_cause "$log_file" "$output_dir"
    extract_component_status "$log_file" "$output_dir"

    echo "Log analysis complete. Results in: $output_dir"
}

# If script is executed directly, run analysis
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    if [ $# -lt 2 ]; then
        echo "Usage: $0 <log-file> <output-dir>"
        exit 1
    fi

    analyze_deployment_logs "$1" "$2"
fi

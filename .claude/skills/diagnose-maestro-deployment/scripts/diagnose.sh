#!/bin/bash
set -e

echo "=========================================="
echo "Maestro Deployment Diagnostic Tool"
echo "=========================================="
echo ""

# Initialize variables
DEPLOYMENT_OUTPUT=""
SVC_RESOURCE_GROUP=""
SVC_CLUSTER_NAME=""
MGMT_RESOURCE_GROUP=""
MGMT_CLUSTER_NAME=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --svc-rg)
            SVC_RESOURCE_GROUP="$2"
            shift 2
            ;;
        --svc-cluster)
            SVC_CLUSTER_NAME="$2"
            shift 2
            ;;
        --mgmt-rg)
            MGMT_RESOURCE_GROUP="$2"
            shift 2
            ;;
        --mgmt-cluster)
            MGMT_CLUSTER_NAME="$2"
            shift 2
            ;;
        *)
            if [ -z "$DEPLOYMENT_OUTPUT" ] && [ -f "$1" ]; then
                DEPLOYMENT_OUTPUT="$1"
            fi
            shift
            ;;
    esac
done

# Function to extract cluster info from deployment output
extract_cluster_info() {
    local output_file=$1

    echo "Analyzing deployment output: $output_file"
    echo ""

    # Try to find cluster names from the output
    if grep -q "pers-usw3" "$output_file"; then
        # Extract cluster name pattern
        local cluster_base=$(grep -o "pers-usw3[a-z]*" "$output_file" | head -1)

        if [ -n "$cluster_base" ]; then
            SVC_CLUSTER_NAME="${cluster_base}-svc"
            SVC_RESOURCE_GROUP="hcp-underlay-${cluster_base}-svc"
            MGMT_CLUSTER_NAME="${cluster_base}-mgmt-1"
            MGMT_RESOURCE_GROUP="hcp-underlay-${cluster_base}-mgmt-1"

            echo "Detected clusters:"
            echo "  Service: $SVC_RESOURCE_GROUP / $SVC_CLUSTER_NAME"
            echo "  Management: $MGMT_RESOURCE_GROUP / $MGMT_CLUSTER_NAME"
            echo ""
        fi
    fi
}

# Extract cluster info if deployment output provided
if [ -n "$DEPLOYMENT_OUTPUT" ]; then
    extract_cluster_info "$DEPLOYMENT_OUTPUT"
fi

# Validate we have cluster information
if [ -z "$SVC_RESOURCE_GROUP" ] || [ -z "$SVC_CLUSTER_NAME" ] || \
   [ -z "$MGMT_RESOURCE_GROUP" ] || [ -z "$MGMT_CLUSTER_NAME" ]; then
    echo "ERROR: Could not determine cluster information."
    echo ""
    echo "Usage:"
    echo "  $0 <deployment-output-file>"
    echo "  $0 --svc-rg <rg> --svc-cluster <cluster> --mgmt-rg <rg> --mgmt-cluster <cluster>"
    exit 1
fi

# Check prerequisites
echo "Step 1: Checking prerequisites..."
if ! command -v az &> /dev/null; then
    echo "ERROR: Azure CLI not installed"
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    echo "ERROR: kubectl not installed"
    exit 1
fi

if ! command -v helm &> /dev/null; then
    echo "ERROR: helm not installed"
    exit 1
fi

if ! command -v jq &> /dev/null; then
    echo "ERROR: jq not installed (required for JSON parsing)"
    echo "Install with: brew install jq (macOS) or apt-get install jq (Linux)"
    exit 1
fi

if ! az account show &> /dev/null; then
    echo "ERROR: Not logged into Azure"
    exit 1
fi

# kubelogin is optional but recommended for Azure AD authentication
if ! command -v kubelogin &> /dev/null; then
    echo "WARNING: kubelogin not installed (Azure AD authentication may fail)"
    echo "Install with: brew install Azure/kubelogin/kubelogin (macOS)"
fi

echo "✓ All prerequisites met"
echo ""

# Create temporary directory for kubeconfigs
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

SVC_KUBECONFIG="$TEMP_DIR/svc.kubeconfig"
MGMT_KUBECONFIG="$TEMP_DIR/mgmt.kubeconfig"

# Get cluster credentials
echo "Step 2: Retrieving cluster credentials..."

# Initialize issue tracking early (will be used if credentials fail)
CREDENTIAL_ISSUES=0

if az aks get-credentials \
    --resource-group "$SVC_RESOURCE_GROUP" \
    --name "$SVC_CLUSTER_NAME" \
    --overwrite-existing \
    -f "$SVC_KUBECONFIG" 2>/dev/null; then
    echo "✓ Service cluster credentials retrieved"
    # kubelogin may fail but shouldn't stop the script
    if command -v kubelogin &> /dev/null; then
        kubelogin convert-kubeconfig -l azurecli --kubeconfig "$SVC_KUBECONFIG" 2>/dev/null || true
    fi
else
    echo "✗ Failed to get service cluster credentials"
    SVC_KUBECONFIG=""
    CREDENTIAL_ISSUES=$((CREDENTIAL_ISSUES + 1))
fi

if az aks get-credentials \
    --resource-group "$MGMT_RESOURCE_GROUP" \
    --name "$MGMT_CLUSTER_NAME" \
    --overwrite-existing \
    -f "$MGMT_KUBECONFIG" 2>/dev/null; then
    echo "✓ Management cluster credentials retrieved"
    # kubelogin may fail but shouldn't stop the script
    if command -v kubelogin &> /dev/null; then
        kubelogin convert-kubeconfig -l azurecli --kubeconfig "$MGMT_KUBECONFIG" 2>/dev/null || true
    fi
else
    echo "✗ Failed to get management cluster credentials"
    MGMT_KUBECONFIG=""
    CREDENTIAL_ISSUES=$((CREDENTIAL_ISSUES + 1))
fi

echo ""

# Step 3: Analyze deployment logs if provided
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_ANALYSIS_DIR="$TEMP_DIR/log-analysis"

if [ -n "$DEPLOYMENT_OUTPUT" ] && [ -f "$DEPLOYMENT_OUTPUT" ]; then
    echo "Step 3: Analyzing deployment logs..."
    echo ""

    # Source the log analysis module
    source "$SCRIPT_DIR/analyze-logs.sh"

    # Run log analysis
    analyze_deployment_logs "$DEPLOYMENT_OUTPUT" "$LOG_ANALYSIS_DIR"

    echo ""
fi

# Initialize report
REPORT_FILE="$TEMP_DIR/diagnosis-report.txt"

cat > "$REPORT_FILE" << EOF
========================================
Maestro Deployment Diagnostic Report
========================================
Generated: $(date)

Clusters Analyzed:
  Service: $SVC_RESOURCE_GROUP / $SVC_CLUSTER_NAME
  Management: $MGMT_RESOURCE_GROUP / $MGMT_CLUSTER_NAME

EOF

# Step 4: Analyze Management Cluster (dynamically based on log analysis)
echo "Step 4: Analyzing Management Cluster..."
echo ""

if [ -n "$MGMT_KUBECONFIG" ]; then
    echo "Management Cluster Analysis" >> "$REPORT_FILE"
    echo "==========================" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"

    # Get all Helm releases for reference
    echo "Helm Releases:" >> "$REPORT_FILE"
    helm --kubeconfig "$MGMT_KUBECONFIG" list -A -o json | \
        jq -r '.[] | "\(.name) (\(.namespace)): \(.status) - Chart: \(.chart)"' >> "$REPORT_FILE" 2>/dev/null || \
        echo "Failed to retrieve Helm releases" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"

    # Determine what to check based on log analysis
    FAILED_RELEASES=""

    if [ -f "$LOG_ANALYSIS_DIR/failed_helm_releases.txt" ]; then
        # Use log analysis results to identify failed releases
        FAILED_RELEASES=$(cat "$LOG_ANALYSIS_DIR/failed_helm_releases.txt" | tr '\n' ' ')
        echo "Failed releases identified from logs: $FAILED_RELEASES"
    fi

    # If no log analysis or empty results, fallback to checking cluster state
    if [ -z "$FAILED_RELEASES" ]; then
        FAILED_RELEASES=$(helm --kubeconfig "$MGMT_KUBECONFIG" list -A -o json | \
            jq -r '.[] | select(.status == "failed") | .name + ":" + .namespace' 2>/dev/null || echo "")
    fi

    if [ -n "$FAILED_RELEASES" ]; then
        echo "Investigating Failed Components:" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"

        # Process each failed release
        for release_info in $FAILED_RELEASES; do
            if [[ "$release_info" == *":"* ]]; then
                release=$(echo "$release_info" | cut -d: -f1)
                namespace=$(echo "$release_info" | cut -d: -f2)
            else
                release="$release_info"
                # Try to find namespace from Helm
                namespace=$(helm --kubeconfig "$MGMT_KUBECONFIG" list -A -o json 2>/dev/null | \
                    jq -r ".[] | select(.name == \"$release\") | .namespace" | head -1)
                if [ -z "$namespace" ]; then
                    namespace="unknown"
                fi
            fi

            echo "Analyzing: $release in namespace $namespace"
            echo "[$release] (namespace: $namespace)" >> "$REPORT_FILE"
            echo "---" >> "$REPORT_FILE"

            if [ "$namespace" != "unknown" ]; then
                # Get pod status
                echo "Pods:" >> "$REPORT_FILE"
                kubectl --kubeconfig "$MGMT_KUBECONFIG" get pods -n "$namespace" -o wide 2>/dev/null >> "$REPORT_FILE" || \
                    echo "  No pods found or error retrieving pods" >> "$REPORT_FILE"
                echo "" >> "$REPORT_FILE"

                # Check for resource conflicts if indicated in log analysis
                if [ -f "$LOG_ANALYSIS_DIR/resource_conflicts.txt" ]; then
                    while IFS= read -r line; do
                        # Parse CONFLICT:resource_name:resource_type:manager:fields format
                        # Use awk to properly split on first 4 colons only
                        conflict_type=$(echo "$line" | awk -F: '{print $1}')
                        if [ "$conflict_type" = "CONFLICT" ]; then
                            resource_name=$(echo "$line" | awk -F: '{print $2}')
                            # Resource type may contain colons, extract everything between 2nd and 3rd-to-last colon
                            resource_type=$(echo "$line" | awk -F: '{for(i=3;i<NF-1;i++) printf "%s%s", $i, (i<NF-2?":":"")}')
                            manager=$(echo "$line" | awk -F: '{print $(NF-1)}')
                            fields=$(echo "$line" | awk -F: '{print $NF}')

                            echo "Resource Conflict Detected:" >> "$REPORT_FILE"
                            echo "  Resource: $resource_name (type: $resource_type)" >> "$REPORT_FILE"
                            echo "  Managed by: $manager" >> "$REPORT_FILE"
                            if [ -n "$fields" ]; then
                                echo "  Conflicting fields:" >> "$REPORT_FILE"
                                echo "$fields" | tr '|' '\n' | sed 's/^/    - /' >> "$REPORT_FILE"
                            fi
                            echo "" >> "$REPORT_FILE"
                        fi
                    done < "$LOG_ANALYSIS_DIR/resource_conflicts.txt"
                fi
            fi
            echo "" >> "$REPORT_FILE"
        done
    else
        echo "No failed Helm releases detected" >> "$REPORT_FILE"
        echo "✓ No failed Helm releases in management cluster"
    fi
    echo "" >> "$REPORT_FILE"
fi

echo ""

# Step 5: Analyze Service Cluster
echo "Step 5: Analyzing Service Cluster..."
echo ""

if [ -n "$SVC_KUBECONFIG" ]; then
    echo "Service Cluster Analysis" >> "$REPORT_FILE"
    echo "========================" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"

    # Get Helm releases
    echo "Helm Releases:" >> "$REPORT_FILE"
    helm --kubeconfig "$SVC_KUBECONFIG" list -A -o json | \
        jq -r '.[] | "\(.name) (\(.namespace)): \(.status) - Chart: \(.chart)"' >> "$REPORT_FILE" 2>/dev/null || \
        echo "Failed to retrieve Helm releases" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"

    # Check maestro namespace
    echo "Maestro Namespace Status:" >> "$REPORT_FILE"
    if kubectl --kubeconfig "$SVC_KUBECONFIG" get namespace maestro &>/dev/null; then
        kubectl --kubeconfig "$SVC_KUBECONFIG" get pods -n maestro -o wide 2>/dev/null >> "$REPORT_FILE" || \
            echo "No pods in maestro namespace" >> "$REPORT_FILE"
    else
        echo "Maestro namespace does not exist" >> "$REPORT_FILE"
    fi
    echo "" >> "$REPORT_FILE"
fi

echo ""

# Step 6: Include Log Analysis Results in Report
if [ -d "$LOG_ANALYSIS_DIR" ]; then
    echo "Deployment Log Analysis" >> "$REPORT_FILE"
    echo "======================" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"

    # Include error patterns
    if [ -f "$LOG_ANALYSIS_DIR/error_patterns.txt" ] && [ -s "$LOG_ANALYSIS_DIR/error_patterns.txt" ]; then
        echo "Identified Error Patterns:" >> "$REPORT_FILE"
        while IFS= read -r line; do
            # Split on literal ':::' delimiter
            pattern="${line%%:::*}"
            context="${line#*:::}"
            echo "  • Pattern: $pattern" >> "$REPORT_FILE"
            echo "    Context: $(echo "$context" | head -c 200)..." >> "$REPORT_FILE"
            echo "" >> "$REPORT_FILE"
        done < "$LOG_ANALYSIS_DIR/error_patterns.txt"
    fi

    # Include deployment timeline
    if [ -f "$LOG_ANALYSIS_DIR/timeline.txt" ] && [ -s "$LOG_ANALYSIS_DIR/timeline.txt" ]; then
        echo "Deployment Timeline (last 20 events):" >> "$REPORT_FILE"
        tail -20 "$LOG_ANALYSIS_DIR/timeline.txt" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
    fi
fi

# Generate diagnosis summary
echo "Diagnosis Summary" >> "$REPORT_FILE"
echo "=================" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# Root Cause Analysis (Dynamic based on error patterns)
echo "ROOT CAUSE ANALYSIS:" >> "$REPORT_FILE"
echo "-------------------" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# Analyze error patterns from logs
if [ -f "$LOG_ANALYSIS_DIR/error_patterns.txt" ] && [ -s "$LOG_ANALYSIS_DIR/error_patterns.txt" ]; then
    # Group errors by pattern type (bash 3.x compatible)
    # Count occurrences of each pattern
    cut -d':' -f1 < "$LOG_ANALYSIS_DIR/error_patterns.txt" | sort | uniq -c | sort -rn > "$LOG_ANALYSIS_DIR/pattern_counts.txt"

    # Determine primary failure based on most common pattern
    primary_pattern=""
    if [ -s "$LOG_ANALYSIS_DIR/pattern_counts.txt" ]; then
        # Get the first line (highest count)
        read -r count pattern_name < "$LOG_ANALYSIS_DIR/pattern_counts.txt"
        primary_pattern="$pattern_name"
    fi

    if [ -n "$primary_pattern" ]; then
        case "$primary_pattern" in
            timing_conflict)
                echo "Primary Failure Type: Timing/Race Condition" >> "$REPORT_FILE"
                echo "Multiple components attempted to manage the same resources simultaneously," >> "$REPORT_FILE"
                echo "leading to conflicts. This often occurs when operators start before Helm" >> "$REPORT_FILE"
                echo "post-install hooks complete." >> "$REPORT_FILE"
                ;;
            timeout)
                echo "Primary Failure Type: Timeout" >> "$REPORT_FILE"
                echo "One or more operations exceeded their time limits. This may indicate" >> "$REPORT_FILE"
                echo "slow network, resource constraints, or hung processes." >> "$REPORT_FILE"
                ;;
            authentication)
                echo "Primary Failure Type: Authentication/Authorization" >> "$REPORT_FILE"
                echo "Deployment failed due to insufficient permissions or invalid credentials." >> "$REPORT_FILE"
                ;;
            network)
                echo "Primary Failure Type: Network Connectivity" >> "$REPORT_FILE"
                echo "Network-related errors prevented successful deployment." >> "$REPORT_FILE"
                ;;
            resource_limit)
                echo "Primary Failure Type: Resource Constraints" >> "$REPORT_FILE"
                echo "Insufficient cluster resources (CPU, memory, or storage) to complete deployment." >> "$REPORT_FILE"
                ;;
            *)
                echo "Primary Failure Type: $primary_pattern" >> "$REPORT_FILE"
                echo "Multiple errors of this type detected in deployment logs." >> "$REPORT_FILE"
                ;;
        esac
        echo "" >> "$REPORT_FILE"
    fi
fi

# Analyze resource conflicts
if [ -f "$LOG_ANALYSIS_DIR/resource_conflicts.txt" ] && [ -s "$LOG_ANALYSIS_DIR/resource_conflicts.txt" ]; then
    echo "Resource Conflicts Detected:" >> "$REPORT_FILE"
    while IFS= read -r line; do
        # Parse CONFLICT:resource_name:resource_type:manager:fields format
        conflict_type=$(echo "$line" | awk -F: '{print $1}')
        if [ "$conflict_type" = "CONFLICT" ]; then
            resource_name=$(echo "$line" | awk -F: '{print $2}')
            # Resource type may contain colons
            resource_type=$(echo "$line" | awk -F: '{for(i=3;i<NF-1;i++) printf "%s%s", $i, (i<NF-2?":":"")}')
            manager=$(echo "$line" | awk -F: '{print $(NF-1)}')
            fields=$(echo "$line" | awk -F: '{print $NF}')

            echo "  • Resource: $resource_name ($resource_type)" >> "$REPORT_FILE"
            echo "    Conflicting manager: $manager" >> "$REPORT_FILE"
            if [ -n "$fields" ] && [ "$fields" != "" ]; then
                echo "    Fields:" >> "$REPORT_FILE"
                echo "$fields" | tr '|' '\n' | sed 's/^/      - /' >> "$REPORT_FILE"
            fi
            echo "" >> "$REPORT_FILE"
        fi
    done < "$LOG_ANALYSIS_DIR/resource_conflicts.txt"
fi

echo "" >> "$REPORT_FILE"
echo "DETAILED ISSUES:" >> "$REPORT_FILE"
echo "----------------" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# Dynamically generate issues based on discoveries
ISSUES_FOUND=0
CRITICAL_ISSUES=0

# Issue: Credential failures (if any)
if [ "$CREDENTIAL_ISSUES" -gt 0 ]; then
    ISSUES_FOUND=$((ISSUES_FOUND + 1))
    CRITICAL_ISSUES=$((CRITICAL_ISSUES + 1))
    echo "[$ISSUES_FOUND] Failed to Retrieve Cluster Credentials" >> "$REPORT_FILE"
    echo "    Severity: CRITICAL" >> "$REPORT_FILE"
    if [ -z "$SVC_KUBECONFIG" ]; then
        echo "    Failed: Service cluster ($SVC_CLUSTER_NAME)" >> "$REPORT_FILE"
    fi
    if [ -z "$MGMT_KUBECONFIG" ]; then
        echo "    Failed: Management cluster ($MGMT_CLUSTER_NAME)" >> "$REPORT_FILE"
    fi
    echo "    " >> "$REPORT_FILE"
    echo "    Recommendation:" >> "$REPORT_FILE"
    echo "      Verify Azure credentials and cluster access permissions" >> "$REPORT_FILE"
    echo "      Check that resource groups and cluster names are correct" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
fi

# Issue: Failed Helm Releases (dynamic)
if [ -n "$FAILED_RELEASES" ]; then
    for release_info in $FAILED_RELEASES; do
        if [[ "$release_info" == *":"* ]]; then
            release=$(echo "$release_info" | cut -d: -f1)
            namespace=$(echo "$release_info" | cut -d: -f2)
        else
            release="$release_info"
            namespace=$(helm --kubeconfig "$MGMT_KUBECONFIG" list -A -o json 2>/dev/null | \
                jq -r ".[] | select(.name == \"$release\") | .namespace" | head -1)
            [ -z "$namespace" ] && namespace="unknown"
        fi

        ISSUES_FOUND=$((ISSUES_FOUND + 1))
        echo "[$ISSUES_FOUND] Helm Release Failed: $release" >> "$REPORT_FILE"
        echo "    Namespace: $namespace" >> "$REPORT_FILE"

        # Determine severity based on pod status
        severity="WARNING"
        if [ "$namespace" != "unknown" ] && [ -n "$MGMT_KUBECONFIG" ]; then
            running_pods=$(kubectl --kubeconfig "$MGMT_KUBECONFIG" get pods -n "$namespace" --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
            total_pods=$(kubectl --kubeconfig "$MGMT_KUBECONFIG" get pods -n "$namespace" --no-headers 2>/dev/null | wc -l)

            if [ "$total_pods" -gt 0 ] && [ "$running_pods" -eq "$total_pods" ]; then
                severity="WARNING"
                echo "    Severity: WARNING (Helm failed but pods are running)" >> "$REPORT_FILE"
                echo "    Actual Status: ✓ All $total_pods pods are Running" >> "$REPORT_FILE"
            elif [ "$total_pods" -eq 0 ]; then
                severity="CRITICAL"
                CRITICAL_ISSUES=$((CRITICAL_ISSUES + 1))
                echo "    Severity: CRITICAL" >> "$REPORT_FILE"
                echo "    Actual Status: ✗ No pods found in namespace" >> "$REPORT_FILE"
            else
                # Partial failure: some pods running, some not - this is WARNING not CRITICAL
                severity="WARNING"
                echo "    Severity: WARNING" >> "$REPORT_FILE"
                echo "    Actual Status: ⚠ $running_pods/$total_pods pods Running" >> "$REPORT_FILE"
            fi
        else
            echo "    Severity: $severity" >> "$REPORT_FILE"
        fi

        # Add recommendation based on severity
        echo "    " >> "$REPORT_FILE"
        echo "    Recommendation:" >> "$REPORT_FILE"
        if [ "$severity" = "WARNING" ]; then
            echo "      Helm failure may be a false-positive. Verify pods are functional." >> "$REPORT_FILE"
        else
            echo "      Investigate pod failures and check Helm release logs." >> "$REPORT_FILE"
        fi
        echo "" >> "$REPORT_FILE"
    done
fi

# Issue 2: Missing deployments in service cluster
if [ -n "$SVC_KUBECONFIG" ]; then
    # Check for maestro namespace
    if ! kubectl --kubeconfig "$SVC_KUBECONFIG" get namespace maestro &>/dev/null; then
        ISSUES_FOUND=$((ISSUES_FOUND + 1))
        CRITICAL_ISSUES=$((CRITICAL_ISSUES + 1))
        echo "[$ISSUES_FOUND] Missing Deployment: maestro in Service Cluster" >> "$REPORT_FILE"
        echo "    Severity: CRITICAL" >> "$REPORT_FILE"
        echo "    Status: Namespace does not exist" >> "$REPORT_FILE"
        echo "    " >> "$REPORT_FILE"
        echo "    Likely Cause:" >> "$REPORT_FILE"
        echo "      Deployment pipeline may have halted before service cluster setup" >> "$REPORT_FILE"
        echo "    " >> "$REPORT_FILE"
        echo "    Recommendation:" >> "$REPORT_FILE"
        echo "      Option 1: Continue deployment to service cluster" >> "$REPORT_FILE"
        echo "      Option 2: Re-run complete deployment" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
    fi
fi

# Issue 3: Error patterns from logs
if [ -f "$LOG_ANALYSIS_DIR/error_patterns.txt" ] && [ -s "$LOG_ANALYSIS_DIR/error_patterns.txt" ]; then
    # Group unique error types (bash 3.x compatible)
    # Get unique patterns
    cut -d':' -f1 < "$LOG_ANALYSIS_DIR/error_patterns.txt" | sort -u > "$LOG_ANALYSIS_DIR/unique_patterns.txt"

    while IFS= read -r pattern; do
        ISSUES_FOUND=$((ISSUES_FOUND + 1))

        # Get first context for this pattern
        context=$(grep "^${pattern}:::" "$LOG_ANALYSIS_DIR/error_patterns.txt" | head -1 | cut -d':' -f4-)

        echo "[$ISSUES_FOUND] Error Pattern Detected: $pattern" >> "$REPORT_FILE"

        case "$pattern" in
            timing_conflict|helm_hook_failed)
                echo "    Severity: WARNING" >> "$REPORT_FILE"
                echo "    Description: Resource timing conflict detected" >> "$REPORT_FILE"
                ;;
            timeout)
                # Timeouts are warnings unless they prevent critical operations
                echo "    Severity: WARNING" >> "$REPORT_FILE"
                echo "    Description: Operation timed out" >> "$REPORT_FILE"
                ;;
            authentication)
                echo "    Severity: CRITICAL" >> "$REPORT_FILE"
                CRITICAL_ISSUES=$((CRITICAL_ISSUES + 1))
                echo "    Description: Authentication or authorization failure" >> "$REPORT_FILE"
                ;;
            network)
                echo "    Severity: CRITICAL" >> "$REPORT_FILE"
                CRITICAL_ISSUES=$((CRITICAL_ISSUES + 1))
                echo "    Description: Network connectivity issue" >> "$REPORT_FILE"
                ;;
            *)
                echo "    Severity: WARNING" >> "$REPORT_FILE"
                echo "    Description: $pattern error detected" >> "$REPORT_FILE"
                ;;
        esac

        echo "    Context: $(echo "$context" | head -c 150)..." >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
    done < "$LOG_ANALYSIS_DIR/unique_patterns.txt"
fi

if [ $ISSUES_FOUND -eq 0 ]; then
    echo "No issues detected." >> "$REPORT_FILE"
    echo "✓ All services appear to be running normally." >> "$REPORT_FILE"
else
    echo "" >> "$REPORT_FILE"
    echo "SUMMARY:" >> "$REPORT_FILE"
    echo "--------" >> "$REPORT_FILE"
    echo "Total Issues: $ISSUES_FOUND" >> "$REPORT_FILE"
    echo "Critical Issues: $CRITICAL_ISSUES" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    if [ $CRITICAL_ISSUES -eq 0 ]; then
        echo "Overall Status: Deployment appears successful despite Helm warnings" >> "$REPORT_FILE"
        echo "Action Required: None - Services are functional" >> "$REPORT_FILE"
    else
        echo "Overall Status: Deployment incomplete - requires intervention" >> "$REPORT_FILE"
        echo "Action Required: Complete service cluster deployment" >> "$REPORT_FILE"
    fi
fi

echo "" >> "$REPORT_FILE"
echo "End of Diagnostic Report" >> "$REPORT_FILE"
echo "========================================" >> "$REPORT_FILE"

# Display report
echo "=========================================="
echo "Diagnostic Report Generated"
echo "=========================================="
echo ""
cat "$REPORT_FILE"

# Save report to current directory
REPORT_OUTPUT="maestro-diagnosis-$(date +%Y%m%d-%H%M%S).txt"
cp "$REPORT_FILE" "$REPORT_OUTPUT"

echo ""
echo "=========================================="
echo "Report saved to: $REPORT_OUTPUT"
echo "=========================================="
echo ""

# Send to Slack if webhook is configured
if [ -n "$SLACK_WEBHOOK_URL" ]; then
    echo "Sending report to Slack..."
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    if bash "$SCRIPT_DIR/send-to-slack.sh" "$REPORT_OUTPUT"; then
        echo "✓ Report sent to Slack"
    else
        echo "⚠ Failed to send report to Slack (report still saved locally)"
    fi
    echo ""
fi

# Summary
if [ $ISSUES_FOUND -gt 0 ]; then
    echo "Found $ISSUES_FOUND issue(s). See report for details and recommendations."
    exit 1
else
    echo "No critical issues found. Deployment appears successful."
    exit 0
fi

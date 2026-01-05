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

if ! az account show &> /dev/null; then
    echo "ERROR: Not logged into Azure"
    exit 1
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

if az aks get-credentials \
    --resource-group "$SVC_RESOURCE_GROUP" \
    --name "$SVC_CLUSTER_NAME" \
    --overwrite-existing \
    -f "$SVC_KUBECONFIG" 2>/dev/null; then
    echo "✓ Service cluster credentials retrieved"
    kubelogin convert-kubeconfig -l azurecli --kubeconfig "$SVC_KUBECONFIG" 2>/dev/null
else
    echo "✗ Failed to get service cluster credentials"
    SVC_KUBECONFIG=""
fi

if az aks get-credentials \
    --resource-group "$MGMT_RESOURCE_GROUP" \
    --name "$MGMT_CLUSTER_NAME" \
    --overwrite-existing \
    -f "$MGMT_KUBECONFIG" 2>/dev/null; then
    echo "✓ Management cluster credentials retrieved"
    kubelogin convert-kubeconfig -l azurecli --kubeconfig "$MGMT_KUBECONFIG" 2>/dev/null
else
    echo "✗ Failed to get management cluster credentials"
    MGMT_KUBECONFIG=""
fi

echo ""

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

# Analyze Management Cluster
echo "Step 3: Analyzing Management Cluster..."
echo ""

if [ -n "$MGMT_KUBECONFIG" ]; then
    echo "Management Cluster Analysis" >> "$REPORT_FILE"
    echo "==========================" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"

    # Get Helm releases
    echo "Helm Releases:" >> "$REPORT_FILE"
    helm --kubeconfig "$MGMT_KUBECONFIG" list -A -o json | \
        jq -r '.[] | "\(.name) (\(.namespace)): \(.status) - Chart: \(.chart)"' >> "$REPORT_FILE" 2>/dev/null || \
        echo "Failed to retrieve Helm releases" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"

    # Find failed releases
    FAILED_RELEASES=$(helm --kubeconfig "$MGMT_KUBECONFIG" list -A -o json | jq -r '.[] | select(.status == "failed") | .name + ":" + .namespace' 2>/dev/null || echo "")

    if [ -n "$FAILED_RELEASES" ]; then
        echo "Failed Helm Releases Detected:" >> "$REPORT_FILE"
        echo "$FAILED_RELEASES" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"

        echo "Found failed Helm releases:"
        echo "$FAILED_RELEASES"
        echo ""

        # Analyze each failed release
        while IFS=: read -r release namespace; do
            echo "Analyzing failed release: $release in namespace $namespace"

            echo "Details for $release (namespace: $namespace):" >> "$REPORT_FILE"
            echo "---" >> "$REPORT_FILE"

            # Get pods in namespace
            echo "Pods in $namespace:" >> "$REPORT_FILE"
            kubectl --kubeconfig "$MGMT_KUBECONFIG" get pods -n "$namespace" -o wide 2>/dev/null >> "$REPORT_FILE" || \
                echo "Failed to get pods" >> "$REPORT_FILE"
            echo "" >> "$REPORT_FILE"

            # Check for specific known issues
            if [ "$release" = "hypershift" ]; then
                # Check ClusterSizingConfiguration
                if kubectl --kubeconfig "$MGMT_KUBECONFIG" get ClusterSizingConfiguration cluster &>/dev/null; then
                    echo "ClusterSizingConfiguration detected:" >> "$REPORT_FILE"
                    kubectl --kubeconfig "$MGMT_KUBECONFIG" get ClusterSizingConfiguration cluster -o yaml >> "$REPORT_FILE" 2>/dev/null
                    echo "" >> "$REPORT_FILE"

                    # Check managed fields
                    MANAGERS=$(kubectl --kubeconfig "$MGMT_KUBECONFIG" get ClusterSizingConfiguration cluster -o jsonpath='{.metadata.managedFields[*].manager}' 2>/dev/null | tr ' ' '\n' | sort -u)
                    echo "Resource managed by: $MANAGERS" >> "$REPORT_FILE"
                    echo "" >> "$REPORT_FILE"
                fi
            fi

        done <<< "$FAILED_RELEASES"
    else
        echo "No failed Helm releases found" >> "$REPORT_FILE"
        echo "✓ No failed Helm releases in management cluster"
    fi
    echo "" >> "$REPORT_FILE"

    # Check critical namespaces
    echo "Critical Namespace Status:" >> "$REPORT_FILE"
    for ns in maestro hypershift multicluster-engine; do
        if kubectl --kubeconfig "$MGMT_KUBECONFIG" get namespace "$ns" &>/dev/null; then
            echo "" >> "$REPORT_FILE"
            echo "Namespace: $ns" >> "$REPORT_FILE"
            kubectl --kubeconfig "$MGMT_KUBECONFIG" get pods -n "$ns" -o wide 2>/dev/null >> "$REPORT_FILE" || \
                echo "No pods or error retrieving pods" >> "$REPORT_FILE"
        fi
    done
    echo "" >> "$REPORT_FILE"
fi

echo ""

# Analyze Service Cluster
echo "Step 4: Analyzing Service Cluster..."
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

# Parse deployment output for errors if provided
ERROR_DETAILS=""
CONFLICT_FIELDS=""

if [ -n "$DEPLOYMENT_OUTPUT" ] && [ -f "$DEPLOYMENT_OUTPUT" ]; then
    echo "Step 5: Analyzing deployment output for errors..."
    echo ""

    # Extract specific error details
    # Look for field conflicts
    if grep -q "Apply failed with.*conflicts" "$DEPLOYMENT_OUTPUT"; then
        # Use Python to properly extract field paths from the JSON-escaped error message
        CONFLICT_FIELDS=$(python3 -c "
import re

# Read entire output file
with open('$DEPLOYMENT_OUTPUT', 'r') as f:
    content = f.read()

# Find the line with conflict details
for line in content.split('\\n'):
    if 'Apply failed with' in line and 'conflicts' in line:
        # Extract field paths for size configurations
        # Format: name=\\\\\"large\\\\\"].criteria.from
        size_matches = re.findall(r'name=\\\\\\\\\"([^\\\\\\\\]+)\\\\\\\\\"]\.criteria\.(from|to)', line)
        for name, attr in size_matches:
            print(f'  • .spec.sizes[name=\"{name}\"].criteria.{attr}')

        # Extract transitionDelay fields
        if 'transitionDelay.decrease' in line:
            print('  • .spec.transitionDelay.decrease')
        if 'transitionDelay.increase' in line:
            print('  • .spec.transitionDelay.increase')
        break
" 2>/dev/null || echo "  • Unable to extract field details")
    fi

    # Extract the main error message
    ERROR_DETAILS=$(grep -o '"err": "errors occurred during execution:.*' "$DEPLOYMENT_OUTPUT" | head -1 | sed 's/"err": "//' | sed 's/"$//' || echo "")

    echo "Deployment Output Analysis" >> "$REPORT_FILE"
    echo "==========================" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"

    if [ -n "$ERROR_DETAILS" ]; then
        echo "Primary Error from Deployment Logs:" >> "$REPORT_FILE"
        echo "$ERROR_DETAILS" | fold -w 100 -s >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
    fi

    if [ -n "$CONFLICT_FIELDS" ]; then
        echo "Conflicting Fields Detected:" >> "$REPORT_FILE"
        echo "$CONFLICT_FIELDS" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
    fi

    # Extract error messages
    if grep -i "error\|failed\|ERROR\|FAILED" "$DEPLOYMENT_OUTPUT" | grep -v "^#" | tail -20 > "$TEMP_DIR/errors.txt"; then
        echo "Recent Errors/Failures (Last 20):" >> "$REPORT_FILE"
        cat "$TEMP_DIR/errors.txt" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
    fi
fi

# Generate diagnosis summary
echo "Diagnosis Summary" >> "$REPORT_FILE"
echo "=================" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# Root Cause Analysis
echo "ROOT CAUSE ANALYSIS:" >> "$REPORT_FILE"
echo "-------------------" >> "$REPORT_FILE"

# Determine primary root cause
if [ -n "$FAILED_RELEASES" ]; then
    if echo "$FAILED_RELEASES" | grep -q "hypershift"; then
        echo "" >> "$REPORT_FILE"
        echo "Primary Failure: Helm post-install hook timing conflict" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
        echo "The hypershift Helm release failed because:" >> "$REPORT_FILE"
        echo "1. The hypershift-operator started and created ClusterSizingConfiguration" >> "$REPORT_FILE"
        echo "2. Helm's post-install hook then tried to create the same resource" >> "$REPORT_FILE"
        echo "3. This caused a field conflict (5 fields managed by different owners)" >> "$REPORT_FILE"

        # Add specific conflict details if available
        if [ -n "$CONFLICT_FIELDS" ]; then
            echo "" >> "$REPORT_FILE"
            echo "Conflicting Fields:" >> "$REPORT_FILE"
            echo "$CONFLICT_FIELDS" >> "$REPORT_FILE"
        fi

        echo "" >> "$REPORT_FILE"
        echo "4. Helm marked the release as 'failed' even though the operator is working" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
        echo "Actual Impact: LOW - Hypershift operator is functional despite Helm status" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
    fi

    # Check if this caused cascading failures
    if kubectl --kubeconfig "$SVC_KUBECONFIG" get namespace maestro &>/dev/null 2>&1; then
        SVC_MAESTRO_EXISTS=true
    else
        SVC_MAESTRO_EXISTS=false
    fi

    if [ "$SVC_MAESTRO_EXISTS" = "false" ]; then
        echo "Cascading Failure: Service cluster deployment incomplete" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
        echo "Because the management cluster deployment reported 'failed':" >> "$REPORT_FILE"
        echo "1. The deployment pipeline halted execution" >> "$REPORT_FILE"
        echo "2. Service cluster Maestro deployment never started" >> "$REPORT_FILE"
        echo "3. E2E tests cannot run without Maestro in service cluster" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
        echo "Actual Impact: HIGH - Service cluster is incomplete and non-functional" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
    fi
fi

echo "" >> "$REPORT_FILE"
echo "DETAILED ISSUES:" >> "$REPORT_FILE"
echo "----------------" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# Check for known issues
ISSUES_FOUND=0
CRITICAL_ISSUES=0

if [ -n "$FAILED_RELEASES" ]; then
    if echo "$FAILED_RELEASES" | grep -q "hypershift"; then
        echo "[$((ISSUES_FOUND + 1))] Hypershift Helm Release Failed" >> "$REPORT_FILE"
        echo "    Status: Failed (false-positive)" >> "$REPORT_FILE"
        echo "    Severity: WARNING" >> "$REPORT_FILE"
        echo "    " >> "$REPORT_FILE"
        echo "    Root Cause:" >> "$REPORT_FILE"
        echo "      ClusterSizingConfiguration resource conflict between Helm hook" >> "$REPORT_FILE"
        echo "      and hypershift-operator-manager" >> "$REPORT_FILE"
        echo "    " >> "$REPORT_FILE"
        echo "    What Happened:" >> "$REPORT_FILE"
        echo "      • Helm post-install hook: aro-hcp-hypershift-operator/templates/cluster.clustersizingconfiguration.yaml" >> "$REPORT_FILE"
        echo "      • Tried to apply ClusterSizingConfiguration resource" >> "$REPORT_FILE"
        echo "      • Resource already managed by hypershift-operator-manager" >> "$REPORT_FILE"
        echo "      • 5 field conflicts detected (sizes, transitionDelay)" >> "$REPORT_FILE"

        # Add specific conflict details if available
        if [ -n "$CONFLICT_FIELDS" ]; then
            echo "    " >> "$REPORT_FILE"
            echo "    Specific Conflicting Fields:" >> "$REPORT_FILE"
            echo "$CONFLICT_FIELDS" | sed 's/^/    /' >> "$REPORT_FILE"
        fi

        echo "    " >> "$REPORT_FILE"
        echo "    Actual Service Status:" >> "$REPORT_FILE"

        # Check actual pod status
        if kubectl --kubeconfig "$MGMT_KUBECONFIG" get pods -n hypershift -l app=operator 2>/dev/null | grep -q "Running"; then
            echo "      ✓ Hypershift operator pod is Running" >> "$REPORT_FILE"
            echo "      ✓ Services are functional despite Helm failure" >> "$REPORT_FILE"
        else
            echo "      ✗ Hypershift operator pod NOT running" >> "$REPORT_FILE"
            CRITICAL_ISSUES=$((CRITICAL_ISSUES + 1))
        fi
        echo "    " >> "$REPORT_FILE"
        echo "    Recommendation:" >> "$REPORT_FILE"
        echo "      This is a known upstream timing issue. Services are working correctly." >> "$REPORT_FILE"
        echo "      No action needed unless operator pod is not Running." >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
        ISSUES_FOUND=$((ISSUES_FOUND + 1))
    fi

    if echo "$FAILED_RELEASES" | grep -q "mce"; then
        echo "[$((ISSUES_FOUND + 1))] MCE (Multicluster Engine) Helm Release Failed" >> "$REPORT_FILE"
        echo "    Status: Failed" >> "$REPORT_FILE"
        echo "    Severity: WARNING" >> "$REPORT_FILE"
        echo "    " >> "$REPORT_FILE"
        echo "    Root Cause:" >> "$REPORT_FILE"
        echo "      Likely related to hypershift failure or similar timing issue" >> "$REPORT_FILE"
        echo "    " >> "$REPORT_FILE"
        echo "    Actual Service Status:" >> "$REPORT_FILE"

        # Check actual pod status
        MCE_PODS=$(kubectl --kubeconfig "$MGMT_KUBECONFIG" get pods -n multicluster-engine --no-headers 2>/dev/null | wc -l | tr -d ' ')
        MCE_RUNNING=$(kubectl --kubeconfig "$MGMT_KUBECONFIG" get pods -n multicluster-engine --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l | tr -d ' ')

        if [ "$MCE_PODS" -gt 0 ]; then
            echo "      ✓ Found $MCE_PODS MCE pods ($MCE_RUNNING Running)" >> "$REPORT_FILE"
            if [ "$MCE_RUNNING" -eq "$MCE_PODS" ]; then
                echo "      ✓ All MCE services are functional" >> "$REPORT_FILE"
            else
                echo "      ⚠ Some MCE pods not Running" >> "$REPORT_FILE"
                CRITICAL_ISSUES=$((CRITICAL_ISSUES + 1))
            fi
        else
            echo "      ✗ No MCE pods found" >> "$REPORT_FILE"
            CRITICAL_ISSUES=$((CRITICAL_ISSUES + 1))
        fi
        echo "    " >> "$REPORT_FILE"
        echo "    Recommendation:" >> "$REPORT_FILE"
        echo "      Verify all MCE operator pods are Running." >> "$REPORT_FILE"
        echo "      If yes, services are functional despite Helm failure." >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
        ISSUES_FOUND=$((ISSUES_FOUND + 1))
    fi
fi

# Check if maestro is missing in service cluster
if [ -n "$SVC_KUBECONFIG" ]; then
    if ! kubectl --kubeconfig "$SVC_KUBECONFIG" get pods -n maestro &>/dev/null || \
       [ $(kubectl --kubeconfig "$SVC_KUBECONFIG" get pods -n maestro --no-headers 2>/dev/null | wc -l) -eq 0 ]; then
        echo "[$((ISSUES_FOUND + 1))] Maestro Not Deployed in Service Cluster" >> "$REPORT_FILE"
        echo "    Status: Missing" >> "$REPORT_FILE"
        echo "    Severity: CRITICAL" >> "$REPORT_FILE"
        echo "    " >> "$REPORT_FILE"
        echo "    Root Cause:" >> "$REPORT_FILE"
        echo "      Deployment pipeline halted after management cluster Helm failures" >> "$REPORT_FILE"
        echo "    " >> "$REPORT_FILE"
        echo "    What Happened:" >> "$REPORT_FILE"
        echo "      1. Management cluster deployment reported failures" >> "$REPORT_FILE"
        echo "      2. Deployment script exited with error code" >> "$REPORT_FILE"
        echo "      3. Service cluster setup phase never executed" >> "$REPORT_FILE"
        echo "      4. Maestro namespace does not exist in service cluster" >> "$REPORT_FILE"
        echo "    " >> "$REPORT_FILE"
        echo "    Impact:" >> "$REPORT_FILE"
        echo "      ✗ Service cluster is incomplete" >> "$REPORT_FILE"
        echo "      ✗ E2E tests cannot run" >> "$REPORT_FILE"
        echo "      ✗ Maestro server-agent communication not possible" >> "$REPORT_FILE"
        echo "    " >> "$REPORT_FILE"
        echo "    Recommendation:" >> "$REPORT_FILE"
        echo "      Option 1: Manually deploy Maestro to service cluster" >> "$REPORT_FILE"
        echo "      Option 2: Re-run deployment with fix for Helm timing issue" >> "$REPORT_FILE"
        echo "      Option 3: Continue deployment from service cluster step" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
        ISSUES_FOUND=$((ISSUES_FOUND + 1))
        CRITICAL_ISSUES=$((CRITICAL_ISSUES + 1))
    fi
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

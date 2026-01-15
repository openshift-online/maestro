#!/bin/bash
set -e

# Complete ManifestWork trace script
# Traces ManifestWorks through Maestro system from any entry point

# Parse arguments
RESOURCE_ID=""
WORK_NAME=""
MANIFEST_KIND=""
MANIFEST_NAME=""
MANIFEST_NAMESPACE="default"
SVC_CONTEXT=""
MGMT_CONTEXT=""
SVC_KUBECONFIG=""
MGMT_KUBECONFIG=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --resource-id)
            [[ -z "$2" || "$2" == --* ]] && { echo "ERROR: --resource-id requires a value"; exit 1; }
            RESOURCE_ID="$2"
            shift 2
            ;;
        --work-name)
            [[ -z "$2" || "$2" == --* ]] && { echo "ERROR: --work-name requires a value"; exit 1; }
            WORK_NAME="$2"
            shift 2
            ;;
        --manifest-kind)
            [[ -z "$2" || "$2" == --* ]] && { echo "ERROR: --manifest-kind requires a value"; exit 1; }
            MANIFEST_KIND="$2"
            shift 2
            ;;
        --manifest-name)
            [[ -z "$2" || "$2" == --* ]] && { echo "ERROR: --manifest-name requires a value"; exit 1; }
            MANIFEST_NAME="$2"
            shift 2
            ;;
        --manifest-namespace)
            [[ -z "$2" || "$2" == --* ]] && { echo "ERROR: --manifest-namespace requires a value"; exit 1; }
            MANIFEST_NAMESPACE="$2"
            shift 2
            ;;
        --svc-context)
            [[ -z "$2" || "$2" == --* ]] && { echo "ERROR: --svc-context requires a value"; exit 1; }
            SVC_CONTEXT="$2"
            shift 2
            ;;
        --mgmt-context)
            [[ -z "$2" || "$2" == --* ]] && { echo "ERROR: --mgmt-context requires a value"; exit 1; }
            MGMT_CONTEXT="$2"
            shift 2
            ;;
        --svc-kubeconfig)
            [[ -z "$2" || "$2" == --* ]] && { echo "ERROR: --svc-kubeconfig requires a value"; exit 1; }
            SVC_KUBECONFIG="$2"
            shift 2
            ;;
        --mgmt-kubeconfig)
            [[ -z "$2" || "$2" == --* ]] && { echo "ERROR: --mgmt-kubeconfig requires a value"; exit 1; }
            MGMT_KUBECONFIG="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo ""
            echo "Usage: $0 <entry-point-options> [cluster-options]"
            echo ""
            echo "Entry points (choose one):"
            echo "  --resource-id <id>              Trace from database resource ID"
            echo "  --work-name <name>              Trace from user-created work name"
            echo "  --manifest-kind <kind>          Trace from manifest (requires --manifest-name)"
            echo "    --manifest-name <name>"
            echo "    [--manifest-namespace <ns>]   (default: default)"
            echo ""
            echo "Cluster access (choose one method):"
            echo ""
            echo "  Method 1: Contexts in same kubeconfig"
            echo "    --svc-context <context>         Service cluster context (for database)"
            echo "    --mgmt-context <context>        Management cluster context (for agent)"
            echo ""
            echo "  Method 2: Separate kubeconfig files"
            echo "    --svc-kubeconfig <path>         Service cluster kubeconfig file"
            echo "    --mgmt-kubeconfig <path>        Management cluster kubeconfig file"
            echo ""
            echo "Examples:"
            echo "  # Using contexts"
            echo "  $0 --resource-id '55c61e54...' --svc-context svc-cluster --mgmt-context mgmt-cluster"
            echo ""
            echo "  # Using separate kubeconfig files"
            echo "  $0 --resource-id '55c61e54...' --svc-kubeconfig ./svc-kube.yaml --mgmt-kubeconfig ./mgmt-kube.yaml"
            echo ""
            echo "  # Trace from manifest with separate files"
            echo "  $0 --manifest-kind deployment --manifest-name test \\"
            echo "     --svc-kubeconfig ~/svc-cluster.yaml --mgmt-kubeconfig ~/mgmt-cluster.yaml"
            exit 1
            ;;
    esac
done

# Validate inputs
entry_point_count=0
[[ -n "$RESOURCE_ID" ]] && ((entry_point_count++)) || true
[[ -n "$WORK_NAME" ]] && ((entry_point_count++)) || true
[[ -n "$MANIFEST_NAME" ]] && ((entry_point_count++)) || true

if [[ $entry_point_count -eq 0 ]]; then
    echo "ERROR: Must specify at least one entry point"
    echo "Use --resource-id, --work-name, or --manifest-kind with --manifest-name"
    echo "    (--manifest-namespace is optional, defaults to 'default')"
    exit 1
fi

if [[ $entry_point_count -gt 1 ]]; then
    echo "ERROR: Specify only ONE entry point"
    exit 1
fi

if [[ -n "$MANIFEST_NAME" && -z "$MANIFEST_KIND" ]]; then
    echo "ERROR: --manifest-kind required when using --manifest-name"
    exit 1
fi

# Validate input formats to prevent SQL injection
if [[ -n "$RESOURCE_ID" ]]; then
    # Resource ID should be a UUID (8-4-4-4-12 hex format)
    if ! [[ "$RESOURCE_ID" =~ ^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$ ]]; then
        echo "ERROR: Invalid resource ID format. Expected UUID format (e.g., 55c61e54-1234-5678-9abc-def012345678)"
        exit 1
    fi
fi

if [[ -n "$WORK_NAME" ]]; then
    # Work name should be a valid Kubernetes resource name (alphanumeric, -, ., max 253 chars)
    if ! [[ "$WORK_NAME" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$ ]] || [[ ${#WORK_NAME} -gt 253 ]]; then
        echo "ERROR: Invalid work name format. Must be a valid Kubernetes resource name"
        exit 1
    fi
fi

echo "═══════════════════════════════════════════════════"
echo "  ManifestWork Trace"
echo "═══════════════════════════════════════════════════"
echo ""

# Helper function to build kubectl command for service cluster
kubectl_svc() {
    if [[ -n "$SVC_KUBECONFIG" ]]; then
        kubectl --kubeconfig="$SVC_KUBECONFIG" "$@"
    elif [[ -n "$SVC_CONTEXT" ]]; then
        kubectl --context="$SVC_CONTEXT" "$@"
    else
        kubectl "$@"
    fi
}

# Helper function to build kubectl command for management cluster
kubectl_mgmt() {
    if [[ -n "$MGMT_KUBECONFIG" ]]; then
        kubectl --kubeconfig="$MGMT_KUBECONFIG" "$@"
    elif [[ -n "$MGMT_CONTEXT" ]]; then
        kubectl --context="$MGMT_CONTEXT" "$@"
    else
        kubectl "$@"
    fi
}

# Function to query database (requires svc cluster access)
query_db() {
    local sql_query="$1"
    local pod_name

    # Display which service cluster access method is being used
    if [[ -n "$SVC_KUBECONFIG" ]]; then
        echo "Using service cluster kubeconfig: $SVC_KUBECONFIG"
    elif [[ -n "$SVC_CONTEXT" ]]; then
        echo "Using service cluster context: $SVC_CONTEXT"
    else
        echo "⚠ WARNING: No service cluster access specified. Using current context."
        echo "  Specify with: --svc-context <context> or --svc-kubeconfig <path>"
        echo ""
    fi

    # Check for postgres-breakglass (ARO-HCP INT - CRITICAL ENVIRONMENT)
    local breakglass_deployment
    breakglass_deployment=$(kubectl_svc -n maestro get deployment postgres-breakglass -o jsonpath='{.metadata.name}' 2>/dev/null || echo "")

    if [[ -n "$breakglass_deployment" ]]; then
        echo "Environment: ARO-HCP INT (CRITICAL)"
        echo "Database: postgres-breakglass"
        echo ""

        # Check if pod is running
        pod_name=$(kubectl_svc -n maestro get pods -l app=postgres-breakglass -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

        if [[ -z "$pod_name" ]]; then
            echo "⚠️  postgres-breakglass pod is not running"
            echo ""
            echo "To start the pod, run:"
            if [[ -n "$SVC_KUBECONFIG" ]]; then
                echo "  kubectl --kubeconfig=$SVC_KUBECONFIG -n maestro scale deployment postgres-breakglass --replicas 1"
            elif [[ -n "$SVC_CONTEXT" ]]; then
                echo "  kubectl --context=$SVC_CONTEXT -n maestro scale deployment postgres-breakglass --replicas 1"
            else
                echo "  kubectl -n maestro scale deployment postgres-breakglass --replicas 1"
            fi
            echo ""
            read -p "Would you like to scale up the pod now? (yes/no): " -r
            echo ""
            if [[ $REPLY =~ ^[Yy]([Ee][Ss])?$ ]]; then
                echo "Scaling up postgres-breakglass deployment..."
                kubectl_svc -n maestro scale deployment postgres-breakglass --replicas 1

                echo "Waiting for pod to be ready (timeout: 60s)..."
                if kubectl_svc -n maestro wait --for=condition=ready pod -l app=postgres-breakglass --timeout=60s 2>/dev/null; then
                    pod_name=$(kubectl_svc -n maestro get pods -l app=postgres-breakglass -o jsonpath='{.items[0].metadata.name}')
                    echo "✓ Pod ready: $pod_name"
                    echo ""
                else
                    echo "ERROR: Pod failed to become ready within 60 seconds"
                    echo "Check pod status:"
                    echo "  kubectl_svc -n maestro get pods -l app=postgres-breakglass"
                    exit 1
                fi
            else
                echo "Aborted. Please start the postgres-breakglass pod manually and re-run the trace."
                exit 1
            fi
        else
            echo "Pod: $pod_name"
            echo ""
        fi

        # Show manual execution instructions
        echo "⚠️  ARO-HCP INT - Manual database access required"
        echo ""
        echo "1. Connect to the pod:"
        if [[ -n "$SVC_KUBECONFIG" ]]; then
            echo "   kubectl --kubeconfig=$SVC_KUBECONFIG -n maestro exec -it $pod_name -- /bin/bash"
        elif [[ -n "$SVC_CONTEXT" ]]; then
            echo "   kubectl --context=$SVC_CONTEXT -n maestro exec -it $pod_name -- /bin/bash"
        else
            echo "   kubectl -n maestro exec -it $pod_name -- /bin/bash"
        fi
        echo ""
        echo "2. Inside the pod, run:"
        echo "   connect"
        echo ""
        echo "3. Execute this SQL query:"
        echo "────────────────────────────────────────"
        echo "$sql_query"
        echo "────────────────────────────────────────"
        echo ""
        echo "Press Enter after you've executed the query to continue the trace..."
        read -r
        return 0
    fi

    # Check for maestro-db (Service cluster)
    pod_name=$(kubectl_svc -n maestro get pods -l name=maestro-db -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

    if [[ -n "$pod_name" ]]; then
        echo "Database: maestro-db (Service cluster)"
        echo ""
        kubectl_svc -n maestro exec -i "$pod_name" -- psql -U maestro -d maestro -c "$sql_query"
        return 0
    fi

    echo "ERROR: No database pod found on service cluster"
    echo "Checked for:"
    echo "  - postgres-breakglass deployment (ARO-HCP INT)"
    echo "  - maestro-db pod (Service cluster)"
    echo ""
    if [[ -z "$SVC_CONTEXT" && -z "$SVC_KUBECONFIG" ]]; then
        echo "TIP: Specify service cluster access with --svc-context or --svc-kubeconfig"
    fi
    exit 1
}

# Function to get AppliedManifestWork by manifestWorkName (requires mgmt cluster access)
get_amw_by_resource_id() {
    local resource_id="$1"

    # Display which management cluster access method is being used
    if [[ -n "$MGMT_KUBECONFIG" ]]; then
        echo "Using management cluster kubeconfig: $MGMT_KUBECONFIG" >&2
    elif [[ -n "$MGMT_CONTEXT" ]]; then
        echo "Using management cluster context: $MGMT_CONTEXT" >&2
    else
        echo "⚠ WARNING: No management cluster access specified. Using current context." >&2
        echo "  Specify with: --mgmt-context <context> or --mgmt-kubeconfig <path>" >&2
        echo "" >&2
    fi

    kubectl_mgmt get appliedmanifestworks -o json 2>/dev/null | \
        jq -r ".items[] | select(.spec.manifestWorkName == \"$resource_id\") | .metadata.name" | head -n1
}

# Function to display applied resources (requires mgmt cluster access)
show_applied_resources() {
    local amw_name="$1"

    echo "Applied Manifests:"
    echo "────────────────────────────"

    local resources
    resources=$(kubectl_mgmt get appliedmanifestwork "$amw_name" -o json 2>/dev/null | \
        jq -r '.status.appliedResources[]? | "\(.resource)\t\(.namespace // "cluster-scoped")\t\(.name)"')

    if [[ -z "$resources" ]]; then
        echo "  No applied resources found"
        return
    fi

    echo "Resource Type       Namespace           Name"
    echo "───────────────     ──────────────      ────────────────────"
    echo "$resources" | while IFS=$'\t' read -r resource ns name; do
        printf "%-19s %-19s %s\n" "$resource" "$ns" "$name"
    done
}

# ENTRY POINT 1: From Manifest Details
if [[ -n "$MANIFEST_NAME" ]]; then
    echo "Entry Point: Manifest Details"
    echo "  Kind:      $MANIFEST_KIND"
    echo "  Name:      $MANIFEST_NAME"
    echo "  Namespace: $MANIFEST_NAMESPACE"
    echo ""

    # Step 1: Get AppliedManifestWork from manifest (on mgmt cluster)
    echo "[1/4] Getting AppliedManifestWork from manifest..."

    if [[ -n "$MGMT_KUBECONFIG" ]]; then
        echo "Using management cluster kubeconfig: $MGMT_KUBECONFIG"
    elif [[ -n "$MGMT_CONTEXT" ]]; then
        echo "Using management cluster context: $MGMT_CONTEXT"
    fi

    AMW_NAME=$(kubectl_mgmt get "$MANIFEST_KIND" "$MANIFEST_NAME" -n "$MANIFEST_NAMESPACE" \
        -o jsonpath='{.metadata.ownerReferences[?(@.kind=="AppliedManifestWork")].name}' 2>/dev/null || echo "")

    if [[ -z "$AMW_NAME" ]]; then
        echo "ERROR: Manifest not found or has no AppliedManifestWork owner"
        echo ""
        echo "Possible reasons:"
        echo "  - Manifest does not exist"
        echo "  - Manifest is not managed by Maestro"
        echo "  - Wrong namespace specified"
        echo "  - Wrong management cluster specified"
        exit 1
    fi

    echo "  AppliedManifestWork: $AMW_NAME"
    echo ""

    # Step 2: Extract Resource ID
    echo "[2/4] Extracting Resource ID from AppliedManifestWork..."
    RESOURCE_ID=$(kubectl_mgmt get appliedmanifestwork "$AMW_NAME" \
        -o jsonpath='{.spec.manifestWorkName}' 2>/dev/null || echo "")

    if [[ -z "$RESOURCE_ID" ]]; then
        echo "ERROR: Cannot extract manifestWorkName from AppliedManifestWork"
        exit 1
    fi

    echo "  Resource ID: $RESOURCE_ID"
    echo ""

    # Step 3: Query database for user work name
    echo "[3/4] Querying database for user work name..."
    SQL_QUERY="SELECT id, payload->'metadata'->>'name' AS user_work_name, created_at, updated_at, deleted_at FROM resources WHERE id = '$RESOURCE_ID';"
    query_db "$SQL_QUERY"
    echo ""

    # Step 4: Show all applied resources
    echo "[4/4] Listing all applied resources..."
    show_applied_resources "$AMW_NAME"
    echo ""

    # Exit to prevent entry point 2 from running
    exit 0
fi

# ENTRY POINT 2: From Resource ID
if [[ -n "$RESOURCE_ID" ]]; then
    echo "Entry Point: Resource ID"
    echo "  Resource ID: $RESOURCE_ID"
    echo ""

    # Step 1: Query database
    echo "[1/3] Querying database for user work name..."
    SQL_QUERY="SELECT id, payload->'metadata'->>'name' AS user_work_name, payload->'spec'->'workload'->'manifests' AS manifests, created_at, updated_at, deleted_at FROM resources WHERE id = '$RESOURCE_ID';"
    query_db "$SQL_QUERY"
    echo ""

    # Step 2: Find AppliedManifestWork
    echo "[2/3] Finding AppliedManifestWork on cluster..."
    AMW_NAME=$(get_amw_by_resource_id "$RESOURCE_ID")

    if [[ -z "$AMW_NAME" ]]; then
        echo "  ⚠ AppliedManifestWork not found on cluster"
        echo "  Work may be deleted or not yet applied"
        echo ""
        exit 0
    fi

    echo "  AppliedManifestWork: $AMW_NAME"
    echo ""

    # Step 3: Show applied resources
    echo "[3/3] Listing applied resources..."
    show_applied_resources "$AMW_NAME"
    echo ""
fi

# ENTRY POINT 3: From User Work Name
if [[ -n "$WORK_NAME" ]]; then
    echo "Entry Point: User-Created Work Name"
    echo "  Work Name: $WORK_NAME"
    echo ""

    # Step 1: Query database
    echo "[1/3] Querying database for resource ID..."
    SQL_QUERY="SELECT id, payload->'metadata'->>'name' AS user_work_name, payload->'spec'->'workload'->'manifests' AS manifests, created_at, updated_at, deleted_at FROM resources WHERE payload->'metadata'->>'name' = '$WORK_NAME';"
    query_db "$SQL_QUERY"

    # If using maestro-db, extract the resource ID from the query result
    if kubectl_svc -n maestro get pods -l name=maestro-db &>/dev/null; then
        echo ""
        echo "Please copy the resource ID from the query result above and provide it to find AppliedManifestWork."
        echo "Then run: $0 --resource-id '<resource-id>'"
        echo ""
        exit 0
    fi

    echo ""
    echo "⚠ Manual step required:"
    echo "1. Copy the resource ID from the database query result above"
    echo "2. Run: $0 --resource-id '<resource-id>' to complete the trace"
    echo ""
fi

echo "═══════════════════════════════════════════════════"
echo "  Trace Complete"
echo "═══════════════════════════════════════════════════"

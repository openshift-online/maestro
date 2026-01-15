#!/bin/bash
set -e

echo "Starting E2E tests on long-running Maestro clusters..."

# Parse test type argument
TEST_TYPE="${1:-upgrade}"

# Step 1: Verify prerequisites
echo "Step 1: Verifying prerequisites..."

# Check Azure CLI
if ! command -v az &> /dev/null; then
    echo "ERROR: Azure CLI is not installed."
    echo "Please install it from: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli"
    exit 1
fi
echo "✓ Azure CLI is installed"

# Check kubectl
if ! command -v kubectl &> /dev/null; then
    echo "ERROR: kubectl is not installed."
    exit 1
fi
echo "✓ kubectl is installed"

# Check kubelogin
if ! command -v kubelogin &> /dev/null; then
    echo "Installing kubelogin..."
    az aks install-cli
fi
echo "✓ kubelogin is installed ($(kubelogin --version))"

# Check jq
if ! command -v jq &> /dev/null; then
    echo "ERROR: jq is not installed."
    echo "Please install it: brew install jq (macOS) or sudo apt install jq (Linux)"
    exit 1
fi
echo "✓ jq is installed"

# Verify Azure account login
if ! az account show &> /dev/null; then
    echo "ERROR: Not logged into Azure."
    echo "Please run: az login"
    exit 1
fi

ACCOUNT_NAME=$(az account show --query "name" -o tsv)
echo "✓ Logged into Azure account: $ACCOUNT_NAME"

# Check required environment variables
if [ -z "$SVC_RESOURCE_GROUP" ] || [ -z "$SVC_CLUSTER_NAME" ] || \
   [ -z "$MGMT_RESOURCE_GROUP" ] || [ -z "$MGMT_CLUSTER_NAME" ]; then
    echo "ERROR: Required environment variables are not set."
    echo "Please set:"
    echo "  export SVC_RESOURCE_GROUP=<your-svc-resource-group>"
    echo "  export SVC_CLUSTER_NAME=<your-svc-cluster-name>"
    echo "  export MGMT_RESOURCE_GROUP=<your-mgmt-resource-group>"
    echo "  export MGMT_CLUSTER_NAME=<your-mgmt-cluster-name>"
    exit 1
fi

echo "Using clusters:"
echo "  Service: $SVC_RESOURCE_GROUP/$SVC_CLUSTER_NAME"
echo "  Management: $MGMT_RESOURCE_GROUP/$MGMT_CLUSTER_NAME"
echo ""

# Step 2: Get AKS credentials
echo "Step 2: Getting AKS credentials..."

az aks get-credentials \
    --resource-group "$SVC_RESOURCE_GROUP" \
    --name "$SVC_CLUSTER_NAME" \
    --overwrite-existing \
    -f ./svc-cluster.kubeconfig

az aks get-credentials \
    --resource-group "$MGMT_RESOURCE_GROUP" \
    --name "$MGMT_CLUSTER_NAME" \
    --overwrite-existing \
    -f ./mgmt-cluster.kubeconfig

echo "✓ Credentials downloaded"

# Step 3: Convert kubeconfig for non-interactive login
echo "Step 3: Converting kubeconfig for azurecli..."

kubelogin convert-kubeconfig -l azurecli --kubeconfig ./svc-cluster.kubeconfig
kubelogin convert-kubeconfig -l azurecli --kubeconfig ./mgmt-cluster.kubeconfig

echo "✓ Kubeconfig converted"

# Verify cluster access
echo "Verifying cluster access..."
kubectl --kubeconfig ./svc-cluster.kubeconfig get pods -A -l app=maestro
kubectl --kubeconfig ./mgmt-cluster.kubeconfig get pods -A -l app=maestro-agent

echo "✓ Cluster access verified"
echo ""

# Step 4: Generate in-cluster kubeconfig
echo "Step 4: Generating in-cluster kubeconfig..."

generate_in_cluster_kube() {
    local kubeconfig=$1
    local type=$2

    echo "  Generating for $type cluster..."

    # Create service account
    kubectl --kubeconfig "$kubeconfig" -n default create serviceaccount e2e-test-admin 2>/dev/null || true

    # Create cluster role binding
    cat << EOF | kubectl --kubeconfig "$kubeconfig" apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: e2e-test-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: e2e-test-admin
  namespace: default
EOF

    # Create token
    local token
    token=$(kubectl --kubeconfig "$kubeconfig" create token e2e-test-admin --namespace default --duration=8h)

    # Get cluster info
    local api_server
    local ca_cert
    api_server=$(kubectl --kubeconfig "$kubeconfig" config view -o jsonpath='{.clusters[0].cluster.server}')
    ca_cert=$(kubectl --kubeconfig "$kubeconfig" config view --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')

    # Create in-cluster kubeconfig
    cat > "${type}-incluster.kubeconfig" << EOF
apiVersion: v1
kind: Config
clusters:
- name: my-cluster
  cluster:
    server: "$api_server"
    certificate-authority-data: "$ca_cert"
users:
- name: e2e-test-admin
  user:
    token: "$token"
contexts:
- name: e2e-test-admin-context
  context:
    cluster: my-cluster
    user: e2e-test-admin
    namespace: default
current-context: e2e-test-admin-context
EOF
}

generate_in_cluster_kube "$(pwd)/svc-cluster.kubeconfig" "svc"
generate_in_cluster_kube "$(pwd)/mgmt-cluster.kubeconfig" "mgmt"

echo "✓ In-cluster kubeconfig files generated"
echo ""

# Step 5: Extract deployment information
echo "Step 5: Extracting deployment information..."

# Get pod template hash for active replicaset
pod_template_hash=$(kubectl --kubeconfig "$(pwd)/svc-cluster.kubeconfig" get rs -l app=maestro -n maestro -o jsonpath='{range .items[?(@.spec.replicas>0)]}{.metadata.labels.pod-template-hash}{"\n"}{end}' | head -1)
if [ -z "$pod_template_hash" ]; then
    echo "ERROR: No active replicaset found"
    exit 1
fi
echo "  Pod template hash: $pod_template_hash"

# Get pod name
pod_name=$(kubectl --kubeconfig "$(pwd)/svc-cluster.kubeconfig" get pods -n maestro -l pod-template-hash="$pod_template_hash" -o jsonpath='{.items[0].metadata.name}')
if [ -z "$pod_name" ]; then
    echo "ERROR: No pod found for replicaset hash $pod_template_hash"
    exit 1
fi
echo "  Pod name: $pod_name"

# Extract commit SHA
commit_sha=$(kubectl --kubeconfig "$(pwd)/svc-cluster.kubeconfig" logs -n maestro "$pod_name" | grep -i "Git Commit" | grep -oE '[a-f0-9]{40}')
if [ -z "$commit_sha" ]; then
    echo "ERROR: Could not extract commit SHA from pod logs"
    exit 1
fi
echo "  Commit SHA: $commit_sha"

# Get consumer name
consumer_name=$(kubectl --kubeconfig "$(pwd)/mgmt-cluster.kubeconfig" get deployment maestro-agent -n maestro -o yaml | grep -E "^\s+- --consumer-name=" | sed 's/.*--consumer-name=//' | head -1)
if [ -z "$consumer_name" ]; then
    echo "ERROR: Could not extract consumer name from agent deployment"
    exit 1
fi
echo "  Consumer name: $consumer_name"
echo ""

# Step 6: Run tests
echo "Step 6: Running $TEST_TYPE tests..."
echo "=========================================="
echo ""

TEST_FAILED=0

run_upgrade_test() {
    echo "Running upgrade tests..."

    if IMAGE="quay.io/redhat-user-workloads/maestro-rhtap-tenant/maestro-e2e:$commit_sha" \
        CONSUMER_NAME="$consumer_name" \
        SERVER_KUBECONFIG="$(pwd)/svc-cluster.kubeconfig" \
        AGENT_IN_CLUSTER_KUBECONFIG="$(pwd)/mgmt-incluster.kubeconfig" \
        SERVICE_ACCOUNT_NAME=clusters-service \
        ENABLE_AUTHORIZATION_POLICY=true \
        bash -x test/upgrade/script/run.sh; then
        echo "✓ Upgrade test passed"
        return 0
    else
        echo "✗ Upgrade test failed"
        return 1
    fi
}

run_e2e_test() {
    echo "Running E2E tests with istio..."

    if AGENT_NAMESPACE=maestro \
        IMAGE="quay.io/redhat-user-workloads/maestro-rhtap-tenant/maestro-e2e:$commit_sha" \
        CONSUMER_NAME="$consumer_name" \
        SERVER_KUBECONFIG="$(pwd)/svc-cluster.kubeconfig" \
        AGENT_KUBECONFIG="$(pwd)/mgmt-cluster.kubeconfig" \
        SERVER_IN_CLUSTER_KUBECONFIG="$(pwd)/svc-incluster.kubeconfig" \
        AGENT_IN_CLUSTER_KUBECONFIG="$(pwd)/mgmt-incluster.kubeconfig" \
        SERVICE_ACCOUNT_NAME=clusters-service \
        bash -x test/e2e/istio/test.sh; then
        echo "✓ E2E test passed"
        return 0
    else
        echo "✗ E2E test failed"
        return 1
    fi
}

case "$TEST_TYPE" in
    upgrade)
        run_upgrade_test || TEST_FAILED=1
        ;;
    e2e)
        run_e2e_test || TEST_FAILED=1
        ;;
    all)
        run_upgrade_test || TEST_FAILED=1
        run_e2e_test || TEST_FAILED=1
        ;;
    *)
        echo "ERROR: Invalid test type: $TEST_TYPE"
        echo "Valid options: upgrade, e2e, all"
        exit 1
        ;;
esac

echo ""
echo "=========================================="

# Step 7: Summarize results
echo "Step 7: Test Summary"
echo "=========================================="

if [ $TEST_FAILED -eq 0 ]; then
    echo "✓ All tests PASSED"
    echo ""
    echo "Test configuration:"
    echo "  Image: quay.io/redhat-user-workloads/maestro-rhtap-tenant/maestro-e2e:$commit_sha"
    echo "  Consumer: $consumer_name"
    echo "  Test type: $TEST_TYPE"
else
    echo "✗ Tests FAILED"
    echo ""
    echo "Check the test output above for failure details."
    echo "Common failure locations:"
    echo "  - test/upgrade/script/run.sh output"
    echo "  - test/e2e/istio/test.sh output"
    echo "  - Pod logs: kubectl --kubeconfig ./svc-cluster.kubeconfig logs -n maestro -l app=maestro"
fi

echo ""

# Step 8: Cleanup
echo "Step 8: Cleaning up test resources..."

kubectl --kubeconfig "$(pwd)/svc-cluster.kubeconfig" delete serviceaccount e2e-test-admin -n default 2>/dev/null || true
kubectl --kubeconfig "$(pwd)/svc-cluster.kubeconfig" delete clusterrolebinding e2e-test-admin 2>/dev/null || true
kubectl --kubeconfig "$(pwd)/mgmt-cluster.kubeconfig" delete serviceaccount e2e-test-admin -n default 2>/dev/null || true
kubectl --kubeconfig "$(pwd)/mgmt-cluster.kubeconfig" delete clusterrolebinding e2e-test-admin 2>/dev/null || true

# Remove kubeconfig files containing sensitive credentials
echo "Removing temporary kubeconfig files..."
rm -f "$(pwd)/svc-cluster.kubeconfig" "$(pwd)/mgmt-cluster.kubeconfig"
rm -f "$(pwd)/svc-incluster.kubeconfig" "$(pwd)/mgmt-incluster.kubeconfig"

echo "✓ Cleanup complete"
echo ""

if [ $TEST_FAILED -eq 0 ]; then
    echo "E2E testing completed successfully!"
    exit 0
else
    echo "E2E testing completed with failures."
    exit 1
fi

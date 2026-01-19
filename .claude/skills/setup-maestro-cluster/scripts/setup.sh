#!/bin/bash
set -e

# Parse command line arguments
OVERRIDE_USER=""
OVERRIDE_PERSIST=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --user)
            OVERRIDE_USER="$2"
            shift 2
            ;;
        --persist)
            OVERRIDE_PERSIST="true"
            shift
            ;;
        --no-persist)
            OVERRIDE_PERSIST="false"
            shift
            ;;
        *)
            echo "Unknown argument: $1"
            echo "Usage: $0 [--user <username>] [--persist|--no-persist]"
            exit 1
            ;;
    esac
done

echo "Starting Maestro long-running cluster setup..."

# Cross-platform timeout function
run_with_timeout() {
    local timeout_duration=$1
    shift

    if command -v timeout &> /dev/null; then
        # Linux: use GNU timeout
        timeout "$timeout_duration" "$@"
    elif command -v gtimeout &> /dev/null; then
        # macOS with coreutils installed
        gtimeout "$timeout_duration" "$@"
    else
        # macOS fallback: run without timeout
        # Note: This is less ideal but allows the script to proceed
        echo "Warning: timeout command not available, running without timeout limit"
        "$@"
    fi
}

# Cleanup function
cleanup() {
    if [ -n "$TEMP_DIR" ] && [ -d "$TEMP_DIR" ]; then
        echo "Cleaning up temporary directory: $TEMP_DIR"
        rm -rf "$TEMP_DIR"
    fi
}

# Register cleanup on exit
trap cleanup EXIT INT TERM

# Step 1: Check if Azure CLI is installed
if ! command -v az &> /dev/null; then
    echo "ERROR: Azure CLI is not installed."
    echo "Please install it from: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli"
    exit 1
fi

echo "✓ Azure CLI is installed"

# Step 2: Verify Azure account login
if ! az account show &> /dev/null; then
    echo "ERROR: Not logged into Azure."
    echo "Please run: az login"
    exit 1
fi

ACCOUNT_NAME=$(az account show --query "name" -o tsv)
echo "Current Azure account: $ACCOUNT_NAME"

if [[ ! "$ACCOUNT_NAME" =~ "ARO Hosted Control Planes" ]]; then
    echo "ERROR: Not logged into the correct Azure account."
    echo "Expected account containing 'ARO Hosted Control Planes', but got: $ACCOUNT_NAME"
    echo "Please login to the correct account using: az login"
    exit 1
fi

echo "✓ Logged into correct Azure account"

# Step 3: Clone ARO-HCP repository
TEMP_DIR=$(mktemp -d)

# Validate mktemp succeeded
if [ -z "$TEMP_DIR" ] || [ ! -d "$TEMP_DIR" ]; then
    echo "ERROR: Failed to create temporary directory"
    exit 1
fi

echo "Cloning ARO-HCP repository to: $TEMP_DIR"

if ! run_with_timeout 300 git clone https://github.com/Azure/ARO-HCP "$TEMP_DIR/ARO-HCP"; then
    echo "ERROR: Failed to clone ARO-HCP repository (timeout: 300s)"
    rm -rf "$TEMP_DIR"
    exit 1
fi

echo "✓ Repository cloned successfully"

# Step 4 & 5: Configure environment and deploy
pushd "$TEMP_DIR/ARO-HCP" > /dev/null

echo "Setting environment variables..."
# Set USER: use --user flag value, then environment variable, then default to oasis
if [ -n "$OVERRIDE_USER" ]; then
    export USER="$OVERRIDE_USER"
else
    export USER="${USER:-oasis}"
fi
# Set PERSIST: use --persist flag value, then environment variable, then default to true
if [ -n "$OVERRIDE_PERSIST" ]; then
    export PERSIST="$OVERRIDE_PERSIST"
else
    export PERSIST="${PERSIST:-true}"
fi
export GITHUB_ACTIONS=true
export GOTOOLCHAIN=go1.24.4

echo "USER=$USER"
echo "PERSIST=$PERSIST"
echo "GITHUB_ACTIONS=$GITHUB_ACTIONS"
echo "GOTOOLCHAIN=$GOTOOLCHAIN"

echo ""
echo "Starting personal-dev-env deployment..."
echo "This may take several minutes..."
echo ""

if run_with_timeout 3600 make personal-dev-env; then
    echo ""
    echo "✓ Deployment completed successfully!"
    echo "ARO-HCP repository location: $TEMP_DIR/ARO-HCP"
else
    echo ""
    echo "ERROR: Deployment failed or timed out (timeout: 3600s)!"
    popd > /dev/null
    exit 1
fi

popd > /dev/null

# Cleanup temporary directory
echo "Cleaning up temporary clone..."
rm -rf "$TEMP_DIR"

echo ""
echo "Setup complete!"

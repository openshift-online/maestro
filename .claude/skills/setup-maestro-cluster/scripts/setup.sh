#!/bin/bash
set -e

echo "Starting Maestro long-running cluster setup..."

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

if ! git clone https://github.com/Azure/ARO-HCP "$TEMP_DIR/ARO-HCP"; then
    echo "ERROR: Failed to clone ARO-HCP repository"
    rm -rf "$TEMP_DIR"
    exit 1
fi

echo "✓ Repository cloned successfully"

# Step 4 & 5: Configure environment and deploy
pushd "$TEMP_DIR/ARO-HCP" > /dev/null

echo "Setting environment variables..."
# Set USER to oasis if not already set (required by ARO-HCP)
export USER="${USER:-oasis}"
# PERSIST can be set via environment variable (default: true for not auto-cleanup after testing)
export PERSIST="${PERSIST:-true}"
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

if make personal-dev-env; then
    echo ""
    echo "✓ Deployment completed successfully!"
    echo "ARO-HCP repository location: $TEMP_DIR/ARO-HCP"
else
    echo ""
    echo "ERROR: Deployment failed!"
    popd > /dev/null
    exit 1
fi

popd > /dev/null

echo ""
echo "Setup complete!"
echo "Note: The ARO-HCP repository has been cloned to: $TEMP_DIR/ARO-HCP"
echo "You can navigate there to manage the environment."

# Maestro Claude Skills

This directory contains custom Claude Code skills for Maestro development and operations.

## Skills Directory Structure

Each skill is organized in its own folder with a `SKILL.md` file that defines the skill implementation:

```
.claude/skills/
├── README.md
├── setup-maestro-cluster/
│   ├── SKILL.md
│   └── scripts/
│       └── setup.sh
├── run-e2e-tests/
│   ├── SKILL.md
│   └── scripts/
│       └── run-tests.sh
└── diagnose-maestro-deployment/
    ├── SKILL.md
    └── scripts/
        └── diagnose.sh
```

## Available Skills

### 1. setup-maestro-cluster

Sets up a long-running Maestro cluster environment using Azure ARO-HCP infrastructure.

**Usage:**
```bash
/setup-maestro-cluster
```

**What it does:**
1. Verifies Azure CLI installation and login status
2. Checks that you're logged into the "ARO Hosted Control Planes" Azure account
3. Clones the ARO-HCP repository to a temporary location
4. Sets required environment variables (USER, PERSIST, GITHUB_ACTIONS, GOTOOLCHAIN)
5. Runs `make personal-dev-env` to deploy the environment
6. Monitors and reports deployment status

**Prerequisites:**
- Azure CLI installed (`brew install azure-cli` on macOS)
- Logged into correct Azure account: `az login`
- Valid Azure permissions for resource creation

**Environment Variables Set:**
- `USER=oasis` (only if not already set)
- `PERSIST=true`
- `GITHUB_ACTIONS=true`
- `GOTOOLCHAIN=go1.24.4`

**Documentation:** See [setup-maestro-cluster/SKILL.md](setup-maestro-cluster/SKILL.md)

---

### 2. run-e2e-tests

Runs end-to-end or upgrade tests on existing long-running Maestro clusters deployed in Azure AKS.

**Usage:**
```bash
/run-e2e-tests [test-type]
```

Where `test-type` can be:
- `upgrade`: Run upgrade tests (default)
- `e2e`: Run standard E2E tests with Istio
- `all`: Run both upgrade and e2e tests

**What it does:**
1. Verifies required tools (az, kubectl, kubelogin, jq)
2. Fetches AKS credentials for svc-cluster and mgmt-cluster
3. Converts kubeconfig for azurecli authentication
4. Generates in-cluster kubeconfig with service account tokens
5. Extracts deployment information (commit SHA, consumer name)
6. Runs selected test type(s)
7. Summarizes test results and failures
8. Cleans up test resources

**Prerequisites:**
- Azure CLI, kubectl, kubelogin, jq must be installed
- Logged into Azure with cluster access
- Long-running clusters must be already deployed
- Required environment variables:
  ```bash
  export SVC_RESOURCE_GROUP="your-svc-rg"
  export SVC_CLUSTER_NAME="your-svc-cluster"
  export MGMT_RESOURCE_GROUP="your-mgmt-rg"
  export MGMT_CLUSTER_NAME="your-mgmt-cluster"
  ```

**Test Types:**
- **upgrade**: Pre-upgrade tests, server upgrade, post-upgrade tests, agent upgrade
- **e2e**: E2E tests with Istio service mesh
- **all**: Runs both upgrade and e2e tests sequentially

**Documentation:** See [run-e2e-tests/SKILL.md](run-e2e-tests/SKILL.md)

---

### 3. diagnose-maestro-deployment

Automatically diagnoses failed Maestro cluster deployments by analyzing Helm releases, pod status, and resource conflicts.

**Usage:**
```bash
# Diagnose using deployment output file
/diagnose-maestro-deployment /path/to/deployment.output

# Diagnose using cluster information directly
/diagnose-maestro-deployment --svc-rg <resource-group> --svc-cluster <cluster-name> --mgmt-rg <resource-group> --mgmt-cluster <cluster-name>
```

**What it does:**
1. Analyzes deployment output to identify resource groups and cluster names
2. Retrieves credentials for both service and management clusters
3. Lists all Helm releases and identifies failed ones
4. Inspects pod states in critical namespaces
5. Checks for known issues (e.g., ClusterSizingConfiguration conflicts)
6. Identifies resource conflicts and timing issues
7. Generates a comprehensive diagnostic report
8. Saves the report to a timestamped file

**Prerequisites:**
- Azure CLI, kubectl, helm must be installed
- Logged into Azure with cluster access
- jq installed for JSON parsing
- Access to deployment output or cluster information

**Known Issues Detected:**
- **Hypershift ClusterSizingConfiguration conflict**: Helm post-install hook conflicts with operator-managed resources
- **MCE deployment failures**: Multicluster Engine Helm release issues
- **Missing Maestro in service cluster**: Deployment halted before service cluster setup

**Output:**
The skill generates a detailed report saved as `maestro-diagnosis-YYYYMMDD-HHMMSS.txt` containing:
- Helm release status for both clusters
- Pod status in critical namespaces
- Failed release details
- Resource conflict analysis
- Root cause identification
- Recommended remediation steps

**Exit Codes:**
- `0`: No critical issues found
- `1`: Issues detected (see report for details)

**Documentation:** See [diagnose-maestro-deployment/SKILL.md](diagnose-maestro-deployment/SKILL.md)

---

## Hooks

### deployment-monitor.sh

A hook that monitors long-running deployment processes and sends notifications.

**Features:**
- Desktop notifications (macOS/Linux)
- Slack notifications via webhook
- Customizable status messages
- Real-time deployment monitoring
- Configurable timeout (default: 2 hours)

**Dependencies:**
- Required: `bash`, `wc`, `tail`, `sed`, `grep`, `cat`, `tr`, `date`, `sleep` (standard Unix tools)
- Optional: `curl` (for Slack notifications), `osascript` (for macOS notifications), `notify-send` (for Linux notifications)

**Configuration:**

To enable Slack notifications:

1. Create a Slack webhook:
   - Go to <https://api.slack.com/messaging/webhooks>
   - Create an Incoming Webhook for your channel
   - Copy the webhook URL

2. Set the webhook URL as an environment variable:
   ```bash
   export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
   ```

3. (Optional) Configure timeout:
   ```bash
   export MONITOR_TIMEOUT=3600  # 1 hour in seconds
   ```

**Usage:**
```bash
# Monitor a deployment task in real-time
.claude/hooks/deployment-monitor.sh monitor <task_id>

# Example:
.claude/hooks/deployment-monitor.sh monitor b4ac6c1

# Send a manual completion notification
.claude/hooks/deployment-monitor.sh notify "COMPLETE" "Deployment finished successfully"

# Send a failure notification
.claude/hooks/deployment-monitor.sh notify "FAILED" "Deployment failed with errors"
```

**What the monitor does:**
1. Tracks the deployment task by its task ID
2. Shows real-time progress updates (line count, elapsed time)
3. Displays the latest deployment activity
4. Detects when the task completes or times out
5. Automatically sends notifications (Slack + desktop) when done
6. Reports final status and deployment duration

The hook will:
1. Send a Slack notification (if configured) with color-coded messages
2. Send desktop notifications on macOS (via osascript) or Linux (via notify-send)
3. Return proper exit codes: 0 for success, 1 for failure, 2 for timeout

---

## How Skills Work

Skills are invoked in Claude Code using the `/` prefix followed by the skill name. When you run a skill:

1. Claude Code reads the `SKILL.md` file from the skill's folder
2. Executes the bash script in the Implementation section
3. Returns the output to you in the chat

Skills are a powerful way to automate complex, multi-step workflows that you perform frequently.

## Creating New Skills

To create a new skill:

1. Create a new folder in `.claude/skills/` with a descriptive name:
   ```bash
   mkdir -p .claude/skills/my-new-skill
   ```

2. Create a `SKILL.md` file in that folder with these sections:
   - Title and description
   - Prerequisites
   - Usage example
   - Steps (what the skill does)
   - Implementation (bash script in a code block)
   - Notes

3. Make sure the bash script is well-commented and handles errors

4. Update this README.md to document the new skill

See existing skills as examples:
- [setup-maestro-cluster/SKILL.md](setup-maestro-cluster/SKILL.md)
- [run-e2e-tests/SKILL.md](run-e2e-tests/SKILL.md)

## Tips for Writing Skills

- **Error Handling**: Always check exit codes and provide clear error messages
- **Prerequisites**: Document all required tools and environment variables
- **Idempotency**: Skills should be safe to run multiple times
- **Cleanup**: Clean up temporary files and resources
- **Progress Updates**: Provide clear progress indicators (✓, step numbers, etc.)
- **Exit Codes**: Use proper exit codes (0 for success, non-zero for failures)
- **Environment Variables**: Use environment variables for configuration instead of hard-coded values

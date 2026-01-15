# Maestro Claude Skills

This directory contains custom Claude Code skills for Maestro development, operations, and troubleshooting.

## Available Skills

### Deployment & Testing Skills

#### `/setup-maestro-cluster`

Sets up a long-running Maestro cluster environment using Azure ARO-HCP infrastructure.

**Use when:**
- Setting up a new development or testing environment
- Deploying both service and management clusters
- Need a persistent Maestro environment for testing

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

[→ Full documentation](./setup-maestro-cluster/)

---

#### `/run-e2e-tests`

Runs end-to-end or upgrade tests on existing long-running Maestro clusters deployed in Azure AKS.

**Use when:**
- Running regression tests on deployed clusters
- Testing upgrades before production deployment
- Validating cluster health after changes

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
- `upgrade`: Pre-upgrade tests, server upgrade, post-upgrade tests, agent upgrade
- `e2e`: E2E tests with Istio service mesh
- `all`: Runs both upgrade and e2e tests sequentially

[→ Full documentation](./run-e2e-tests/)

---

#### `/diagnose-maestro-deployment`

Automatically diagnoses failed Maestro cluster deployments by analyzing Helm releases, pod status, and resource conflicts.

**Use when:**
- A deployment has failed and you need to understand why
- Helm releases are in a failed state
- Pods are crashing or not starting
- Need a comprehensive diagnostic report

**What it does:**
1. Analyzes deployment output to identify resource groups and cluster names
2. Retrieves credentials for both service and management clusters
3. Lists all Helm releases and identifies failed ones
4. Inspects pod states in critical namespaces
5. Checks for known issues (e.g., ClusterSizingConfiguration conflicts)
6. Identifies resource conflicts and timing issues
7. Generates a comprehensive diagnostic report
8. Saves the report to a timestamped file

**Known Issues Detected:**
- Hypershift ClusterSizingConfiguration conflict
- MCE deployment failures
- Missing Maestro in service cluster

[→ Full documentation](./diagnose-maestro-deployment/)

---

### Troubleshooting Skills

#### `/trace-manifestwork`

Trace and display the manifests of a ManifestWork and its corresponding AppliedManifestWork.

**Use when:**
- Verifying what manifests are in a ManifestWork
- Checking if a ManifestWork has been applied to the cluster
- Finding the work that owns a specific manifest
- Debugging manifest application issues

**Replaces:**
- `troubleshooting/runbooks/trace_work_manifests.md`
- `troubleshooting/scripts/trace_work_manifests.sh`

[→ Full documentation](./trace-manifestwork/)

---

#### `/trace-resource-request`

Trace resource requests through the complete Maestro lifecycle from client to agent and back.

**Use when:**
- A resource request (create/update/delete) is not completing as expected
- Status updates from the agent are not reaching the client
- Investigating why manifests are not being applied to the target cluster
- Debugging message broker (MQTT/gRPC) communication issues
- Tracing the full path of a request using operation ID, resource ID, work name, or manifest name
- Understanding the timeline and flow of events for a specific resource

**Replaces:**
- Manual log analysis and grep commands
- Ad-hoc troubleshooting procedures
- Scattered debugging knowledge

[→ Full documentation](./trace-resource-request/)

---

## How to Use Skills

Simply invoke a skill by name:

```bash
/setup-maestro-cluster
/run-e2e-tests
/diagnose-maestro-deployment
/trace-manifestwork
/trace-resource-request
```

Claude will:
1. Ask you for required information interactively
2. Verify prerequisites automatically
3. Execute the workflow
4. Present results in a formatted, readable way
5. Suggest next steps based on findings

## Skill Structure

Each skill follows this organized structure:

```
skills/
└── skill-name/
    ├── SKILL.md              # Main skill definition (read by Claude)
    ├── README.md             # Human-readable documentation (optional)
    ├── scripts/              # Executable scripts (optional)
    │   └── *.sh
    ├── references/           # Reference documentation (optional)
    │   └── *.md
    └── examples/             # Example outputs (optional)
        └── *.md
```

### What's in Each Skill

**SKILL.md** - The main file that Claude reads, containing:
- Step-by-step execution instructions
- Error handling logic
- Output formatting guidelines
- Technical details

**README.md** - Human-readable documentation with:
- Quick start guide
- Use cases
- Prerequisites
- Troubleshooting tips

**scripts/** - Standalone bash/shell scripts that can be run independently

**references/** - Detailed reference documentation, troubleshooting guides, and schemas

**examples/** - Example outputs showing different scenarios

## Why Skills Are Better Than Manual Runbooks

### Before (Manual Process)
1. Find and read the runbook
2. Locate the script in another directory
3. Figure out which parameters you need
4. Set environment variables
5. Run commands manually
6. Parse raw output yourself
7. Look up error messages
8. Decide what to do next

### After (Automated Skill)
1. Type the skill name (e.g., `/trace-manifestwork`)
2. Answer a few questions
3. Get formatted results with next steps

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

**Configuration:**

To enable Slack notifications:

1. Create a Slack webhook at <https://api.slack.com/messaging/webhooks>
2. Set the webhook URL as an environment variable:
   ```bash
   export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
   ```

**Usage:**
```bash
# Monitor a deployment task in real-time
.claude/hooks/deployment-monitor.sh monitor <task_id>

# Send a manual notification
.claude/hooks/deployment-monitor.sh notify "COMPLETE" "Deployment finished successfully"
```

---

## Creating New Skills

1. **Create directory structure:**
   ```bash
   mkdir -p .claude/skills/my-skill/{scripts,references,examples}
   ```

2. **Create SKILL.md** - Main skill definition with YAML frontmatter:
   ```markdown
   ---
   name: my-skill
   description: Brief description of when to use this skill
   ---

   # Skill Name

   ## Overview
   ...

   ## When to Use This Skill
   ...

   ## Step 1: ...
   ## Step 2: ...
   ...
   ```

3. **Add supporting files:**
   - Scripts in `scripts/`
   - Reference docs in `references/`
   - Example outputs in `examples/`

4. **Create README.md** for human readers (optional but recommended)

5. **Test thoroughly** with real scenarios

6. **Update this file** to list the new skill

### Skill Guidelines

**Good skills are:**
- ✅ Self-contained (all logic and assets in one directory)
- ✅ Clear and actionable (step-by-step instructions)
- ✅ Error-aware (handle common failures gracefully)
- ✅ User-friendly (formatted output, helpful suggestions)
- ✅ Well-documented (examples and edge cases)
- ✅ Idempotent (safe to run multiple times)

**Avoid:**
- ❌ Vague instructions that leave Claude guessing
- ❌ Missing error handling
- ❌ Raw command output without formatting
- ❌ Assuming prerequisites are met
- ❌ Dead ends (always suggest next steps)
- ❌ Hard-coded values (use environment variables for configuration)

## Tips for Writing Skills

- **Error Handling**: Always check exit codes and provide clear error messages
- **Prerequisites**: Document all required tools and environment variables
- **Idempotency**: Skills should be safe to run multiple times
- **Cleanup**: Clean up temporary files and resources
- **Progress Updates**: Provide clear progress indicators (✓, step numbers, etc.)
- **Exit Codes**: Use proper exit codes (0 for success, non-zero for failures)
- **Environment Variables**: Use environment variables for configuration instead of hard-coded values

## Migrating from Runbooks

When converting a runbook to a skill:

1. **Analyze the runbook** - Understand the workflow and decision points
2. **Copy the script** to `scripts/` directory
3. **Create SKILL.md** with automated instructions and YAML frontmatter
4. **Add reference docs** to `references/` for detailed troubleshooting
5. **Add examples** showing expected outputs in `examples/`
6. **Test end-to-end** with real scenarios
7. **Update runbook** with deprecation notice pointing to skill

## Development Workflow

```bash
# Create new skill structure
mkdir -p .claude/skills/my-skill/{scripts,references,examples}

# Add script
cp troubleshooting/scripts/my-script.sh .claude/skills/my-skill/scripts/

# Add reference documentation
vim .claude/skills/my-skill/references/troubleshooting-guide.md

# Create SKILL.md with YAML frontmatter
vim .claude/skills/my-skill/SKILL.md

# Add examples
vim .claude/skills/my-skill/examples/successful-case.md

# Create README (optional)
vim .claude/skills/my-skill/README.md

# Test the skill
/my-skill
```

## Skills Roadmap

**Completed:**
- ✅ `/setup-maestro-cluster` - Set up long-running Maestro cluster environment
- ✅ `/run-e2e-tests` - Run end-to-end and upgrade tests
- ✅ `/diagnose-maestro-deployment` - Diagnose failed deployments
- ✅ `/trace-manifestwork` - Trace ManifestWork and AppliedManifestWork
- ✅ `/trace-resource-request` - Trace resource requests through the system

**Planned:**
- ⏳ `/check-agent-health` - Verify Maestro agent health
- ⏳ `/check-server-health` - Verify Maestro server health
- ⏳ `/analyze-logs` - Analyze Maestro logs for patterns and errors
- ⏳ `/check-mqtt-broker` - Verify MQTT broker health and connectivity

## Contributing

We welcome contributions! To add or improve skills:

1. Follow the skill structure and guidelines above
2. Test thoroughly with real scenarios
3. Add comprehensive examples
4. Document edge cases and errors
5. Submit a pull request

## Resources

- [Claude Code Skills Documentation](https://code.claude.com/docs/en/skills)
- [Maestro Project Documentation](../../README.md)
- [Original Troubleshooting Runbooks](../../troubleshooting/runbooks/)
- [Original Scripts](../../troubleshooting/scripts/)

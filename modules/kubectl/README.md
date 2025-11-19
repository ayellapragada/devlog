# modules/kubectl/

This module captures kubectl operations automatically by wrapping the `kubectl` command. It intercepts kubectl commands and sends events to DevLog after successful operations.

## Files

### module.go
**Location:** [module.go](module.go)

Module registration and install/uninstall logic.

### hooks/kubectl-wrapper.sh
**Location:** [hooks/kubectl-wrapper.sh](hooks/kubectl-wrapper.sh)

Shell script that wraps the real `kubectl` binary and captures operations.

### hooks/devlog-kubectl-common.sh
**Location:** [hooks/devlog-kubectl-common.sh](hooks/devlog-kubectl-common.sh)

Shared library functions for kubectl event capture logic.

## Installation

```bash
./bin/devlog module install kubectl
```

### What Gets Installed

1. **Kubectl wrapper script** → `~/.local/bin/kubectl`
   - Intercepts all kubectl commands
   - Calls the real kubectl binary
   - Captures successful operations

2. **K alias wrapper** → `~/.local/bin/k`
   - Same functionality as kubectl wrapper
   - Supports the common `k` alias

3. **Common library** → `~/.local/bin/devlog-kubectl-common.sh`
   - Shared functions for event creation
   - Parses kubectl output and context
   - Sends events to daemon

4. **PATH modification required**
   - Add `~/.local/bin` to start of PATH
   - Must come before `/usr/local/bin` to intercept kubectl

### Post-Install

Add to your shell RC file (`~/.zshrc` or `~/.bashrc`):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Then restart your shell:
```bash
source ~/.zshrc  # or ~/.bashrc
```

## Captured Events

The module captures these kubectl operations:

### apply
Triggered after `kubectl apply`

**Event Type:** `kubectl_apply`

**Payload:**
```json
{
  "context": "production",
  "cluster": "my-cluster",
  "namespace": "default",
  "resource_type": "deployment",
  "resource_count": "3",
  "exit_code": 0
}
```

### create
Triggered after `kubectl create`

**Event Type:** `kubectl_create`

**Payload:**
```json
{
  "context": "staging",
  "cluster": "staging-cluster",
  "namespace": "app",
  "resource_type": "pod",
  "resource_count": "1",
  "exit_code": 0
}
```

### delete
Triggered after `kubectl delete`

**Event Type:** `kubectl_delete`

**Payload:**
```json
{
  "context": "development",
  "cluster": "dev-cluster",
  "namespace": "test",
  "resource_type": "service",
  "resource_names": "my-service nginx-service",
  "exit_code": 0
}
```

### get
Triggered after `kubectl get`

**Event Type:** `kubectl_get`

**Payload:**
```json
{
  "context": "production",
  "cluster": "prod-cluster",
  "namespace": "monitoring",
  "resource_type": "pods",
  "resource_names": "all",
  "exit_code": 0
}
```

### describe
Triggered after `kubectl describe`

**Event Type:** `kubectl_describe`

**Payload:**
```json
{
  "context": "production",
  "cluster": "prod-cluster",
  "namespace": "default",
  "resource_type": "pod",
  "resource_names": "nginx-pod",
  "exit_code": 0
}
```

### edit
Triggered after `kubectl edit`

**Event Type:** `kubectl_edit`

**Payload:**
```json
{
  "context": "staging",
  "cluster": "staging-cluster",
  "namespace": "app",
  "resource_type": "deployment",
  "resource_names": "api-deployment",
  "exit_code": 0
}
```

### patch
Triggered after `kubectl patch`

**Event Type:** `kubectl_patch`

**Payload:**
```json
{
  "context": "production",
  "cluster": "prod-cluster",
  "namespace": "default",
  "resource_type": "service",
  "resource_names": "my-service",
  "exit_code": 0
}
```

### logs
Triggered after `kubectl logs`

**Event Type:** `kubectl_logs`

**Payload:**
```json
{
  "context": "production",
  "cluster": "prod-cluster",
  "namespace": "app",
  "resource_type": "pod",
  "resource_names": "api-pod-123",
  "exit_code": 0
}
```

### exec
Triggered after `kubectl exec`

**Event Type:** `kubectl_exec`

**Payload:**
```json
{
  "context": "production",
  "cluster": "prod-cluster",
  "namespace": "default",
  "resource_type": "pod",
  "resource_names": "debug-pod",
  "exit_code": 0
}
```

### debug
Triggered after `kubectl debug`

**Event Type:** `kubectl_debug`

**Payload:**
```json
{
  "context": "staging",
  "cluster": "staging-cluster",
  "namespace": "app",
  "resource_type": "pod",
  "resource_names": "broken-pod",
  "exit_code": 0
}
```

## How It Works

### 1. Command Interception

When you run `kubectl apply`:
```
you type: kubectl apply -f deployment.yaml
         ↓
~/.local/bin/kubectl (wrapper)
         ↓
/usr/local/bin/kubectl (real kubectl)
         ↓
wrapper captures success
         ↓
sends event to devlogd
```

### 2. Event Creation

The wrapper script:
1. Executes real kubectl command
2. Captures output and exit code
3. Extracts context, namespace, and resource info
4. Creates event JSON
5. POSTs to `http://127.0.0.1:8573/api/v1/ingest`

### 3. Context Detection

The wrapper automatically detects:
- Current kubectl context
- Current cluster name
- Namespace (from flag or context default)
- Resource type and names from command arguments

## Uninstallation

```bash
./bin/devlog module uninstall kubectl
```

This removes:
- `~/.local/bin/kubectl` (wrapper script)
- `~/.local/bin/k` (alias wrapper)
- `~/.local/bin/devlog-kubectl-common.sh` (library)

**Note:** Only removes files if they match DevLog's version. If you've modified the wrapper, it will skip removal with a warning.

## Configuration

No configuration required. The module works globally for all kubectl contexts once installed.

## Disabling Temporarily

Set the environment variable to temporarily disable event capture:

```bash
export DEVLOG_KUBECTL_ENABLED=false
kubectl apply -f secret-deployment.yaml
unset DEVLOG_KUBECTL_ENABLED
```

## Troubleshooting

### Kubectl commands not being captured

**Check PATH order:**
```bash
which kubectl
# Should show: /Users/username/.local/bin/kubectl
```

If it shows `/usr/local/bin/kubectl`, your PATH is incorrect.

**Fix:**
```bash
export PATH="$HOME/.local/bin:$PATH"
```

### Daemon not receiving events

**Check daemon is running:**
```bash
./bin/devlog daemon status
```

**Check daemon logs:**
```bash
./bin/devlog status
```

### Wrapper conflicts

If another tool also wraps kubectl, you may see issues. Check:
```bash
cat ~/.local/bin/kubectl
```

Ensure it contains DevLog's wrapper code.

### K alias not working

Make sure both wrappers are executable:
```bash
ls -la ~/.local/bin/kubectl ~/.local/bin/k
```

Both should have execute permissions (`-rwxr-xr-x`).

## Testing

After installation, test with:

```bash
# Test with a safe get command
kubectl get pods
./bin/devlog status

# Test with apply (use a test namespace)
kubectl apply -f test-deployment.yaml
./bin/devlog status
```

You should see the kubectl events in the output.

## Use Cases

### Deployment Tracking

Track when deployments are applied to different environments:
- Which resources were deployed
- What context/cluster was targeted
- When deployments happened

### Debug Session Tracking

Capture debugging activities:
- When you exec into pods
- Which logs you viewed
- Resource modifications via edit/patch

### Operations Audit

Maintain an audit trail of kubectl operations:
- Resource creation and deletion
- Configuration changes
- Context switches

## Performance

The kubectl module is lightweight:
- **Zero overhead** when disabled via env var
- **Async execution** - events sent in background
- **No polling** - event-driven only
- **Fast ingestion** - events queue if daemon offline

Typical overhead per command: <10ms

## Dependencies

- kubectl (any version)
- DevLog daemon running at `http://127.0.0.1:8573`
- devlog binary in PATH

## See Also

- [Module system overview](../README.md)
- [Event model](../../internal/events/)
- [API endpoints](../../internal/api/)

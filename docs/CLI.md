# kubeestimate CLI

`kubeestimate` is an optional supporting tool for [Kube Cost Workspace](README.md).
It estimates manifests, summarizes clusters, prices them, and simulates a rollout
using the same services and cost model as the desktop application. This guide
covers installing the CLI, [commands](#usage), [examples](#examples), and
troubleshooting.

## Install

From the root of a local repository:

```sh
go install ./cmd/kubeestimate
```

This builds the CLI and places the binary in Go's bin directory:

| System | Location |
|---|---|
| Linux, macOS, WSL | `~/go/bin/kubeestimate` |
| Windows | `%USERPROFILE%\go\bin\kubeestimate.exe` |

To see where it went on your machine, run `go env GOPATH`. The binary is in the `bin` folder inside that path.

Build it from this repository's local checkout; `go install github.com/...@latest` is not supported.

## Add Go's bin directory to your PATH

Skip this step if `kubeestimate help` already works in a new terminal.

**Linux and WSL (bash)**

```bash
echo 'export PATH="$PATH:$HOME/go/bin"' >> ~/.bashrc
source ~/.bashrc
```

**macOS (zsh)**

```zsh
echo 'export PATH="$PATH:$HOME/go/bin"' >> ~/.zshrc
source ~/.zshrc
```

**Windows (PowerShell)**

```powershell
$bin = "$env:USERPROFILE\go\bin"
[Environment]::SetEnvironmentVariable("Path", [Environment]::GetEnvironmentVariable("Path", "User") + ";$bin", "User")
```

Then open a new terminal so the change applies.

### Using WSL on Windows

Install and run the CLI where your kubeconfig and AWS CLI are. If you use `kubectl`, `kind` or `aws` inside WSL, install Go inside WSL and run every command there. The repository can stay on the Windows drive:

```bash
cd /mnt/c/Users/<you>/path/to/kube-budget
go install ./cmd/kubeestimate
```

A binary built in WSL works only in WSL. A Windows build reads the Windows kubeconfig (`%USERPROFILE%\.kube\config`), not the one in WSL.

## Verify the installation

```sh
kubeestimate help
```

Estimate a sample manifest. This needs no cluster:

```sh
kubeestimate manifest -f data/manifests/valid-deployment.yaml --provider aws --region us-east-1
```

You should see the workload `api` with hourly, daily and monthly totals.

If you have a cluster, list your contexts and summarize the current one:

```sh
kubeestimate contexts
kubeestimate cluster
```

## Usage

| Command | Description |
|---------|-------------|
| `kubeestimate help` | Show commands and options |
| `kubeestimate manifest -f app.yaml` | Estimate a Deployment using provider list prices, without a cluster |
| `kubeestimate contexts` | List kubeconfig contexts with numbers, short names, and the current context |
| `kubeestimate cluster` | Summarize platform, nodes, namespaces, request coverage, and warnings; add `--workloads` for workload details |
| `kubeestimate cost` | Report billed, requested, idle, and shared costs, forecast, and budget; group `--by namespace`, `workload`, or `nodegroup` |
| `kubeestimate simulate -f app.yaml` | Simulate placement, new nodes, cost changes, optimizations, and month forecast |

Cluster commands use the current kubeconfig context unless `--context` selects
another by number, short name, full name, or a unique part of its name. EKS ARN
and eksctl contexts use the cluster name as their short name. Use `--kubeconfig`
to select a different kubeconfig file.

`cost` and `simulate` detect the provider and region from nodes; `--provider`
and `--region` override them. Each node is priced as its own machine type.

`manifest` uses provider defaults for omitted pricing options and prints an
advisory for each assumption. The older `kubeestimate -f app.yaml` form still works.

Every command supports JSON output with `-o json`. Exit codes are `0` for
success, `1` for command failure, and `2` for invalid usage.

## Examples

Run these examples from the repository root to use the bundled sample manifests.

### Estimate without a cluster

```bash
# Defaults: AWS, us-east-1, m6i.large
kubeestimate manifest -f data/manifests/valid-deployment.yaml

# Compare with a Google Cloud machine type
kubeestimate manifest -f data/manifests/valid-deployment.yaml --provider gcp --region us-central1 --instance-type e2-standard-2
```

### Inspect a cluster and its costs

```bash
kubeestimate contexts
kubeestimate cluster --context 2 --workloads
kubeestimate cost --by workload --top 10
kubeestimate cost -n payments
```

### Simulate a rollout and export structured output

```bash
kubeestimate simulate -f data/manifests/valid-deployment.yaml
kubeestimate cost -o json | jq '.report.totals.provisioned.monthly'
```

### Behavior and limits

- Every command is read-only towards the cluster.
- Each `cost` run records a point in the spend history shared with the desktop
	application. Pass `--no-history` to skip it.
- `simulate` reads budgets and dismissed recommendations but writes nothing.
- Setting budgets, viewing cost-history trends, and tracking optimization steps
	through apply/confirm are desktop workflows. The CLI shows the forecast
	against the budget set there.
- Manifest estimates support one `apps/v1` Deployment file at a time.
- Only supported AWS, Google Cloud, and Azure nodes can be priced; local cluster
	nodes cannot. Prices are bundled list-price snapshots, not cloud invoices,
	and exclude discounts, savings plans, root disks, and public IPs.
- Monthly figures use the same 730-hour month as the desktop application.

## Updating

Pull the latest code and install again. The new binary replaces the old one.

```sh
git pull
go install ./cmd/kubeestimate
```

If you change the code yourself, run `go install ./cmd/kubeestimate` again to use your changes.

## Uninstalling

Delete the binary:

```sh
rm ~/go/bin/kubeestimate                               # Linux, macOS, WSL
Remove-Item "$env:USERPROFILE\go\bin\kubeestimate.exe" # Windows PowerShell
```

The CLI and the desktop app keep cost history, budgets and recommendation state in a `kube-cost-workspace` folder in your user config directory:

| System | Folder |
|---|---|
| Linux, WSL | `~/.config/kube-cost-workspace/` |
| macOS | `~/Library/Application Support/kube-cost-workspace/` |
| Windows | `%AppData%\kube-cost-workspace\` |

Delete that folder too if you also want to remove this data. The desktop app loses it as well.

## Troubleshooting

| Problem | Fix |
|---|---|
| `kubeestimate: command not found` or `is not recognized` | Go's bin directory is not on your PATH. Follow [Add Go's bin directory to your PATH](#add-gos-bin-directory-to-your-path) and open a new terminal. |
| `go: command not found` | Go is not installed, or not in this environment. On Windows, check whether Go is installed in WSL or in Windows. |
| `the kubeconfig has no current context` | Pass `--context`. It takes the number, short name or full name shown by `kubeestimate contexts`, or a unique part of the name. |
| `context "<name>" was not found` | Nothing in your kubeconfig matches. Check `kubeestimate contexts`, or pass `--kubeconfig <path>` if the context is in another file. |
| `"<name>" matches 2 contexts` | Two contexts share that short name, for example the ARN and eksctl contexts of one EKS cluster. Use the number from `kubeestimate contexts` or the full name. |
| `this cluster cannot be priced` | The cluster runs locally (kind, minikube, k3s) or in a region missing from the price catalog. `cluster` still works, but `cost` and `simulate` need nodes on AWS, Google Cloud or Azure. For a local cluster, use `manifest` to estimate workloads at cloud prices instead. |
| EKS: `getting credentials: exec: executable aws not found` | Install AWS CLI v2 in the same environment as the CLI (for example inside WSL). |
| EKS: expired token or `AccessDenied` | Log in again, for example `aws sso login --profile <profile>`. Your IAM principal needs Kubernetes access to the cluster. |
| Timeout while collecting the cluster | The API server is unreachable. Check your VPN or network access to private EKS endpoints, and that the cluster still exists. |
| Usage figures are missing from the cost report | Install metrics-server on the cluster. Costs are still calculated from requests without it. |

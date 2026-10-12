# Installing the kubeestimate CLI

`kubeestimate` is the command-line version of Kube Cost Workspace. It estimates manifests, summarizes clusters, prices them and simulates a rollout, using the same code as the desktop app. This guide covers installing it, checking that it works and fixing common problems. For the full list of commands, see [CLI usage](README.md#cli-usage) in the README.

## Requirements

| You need | For |
|---|---|
| [Go 1.25 or later](https://go.dev/dl/) and Git | Building and installing the CLI |
| A kubeconfig with access to a cluster | `contexts`, `cluster`, `cost` and `simulate` |
| [AWS CLI v2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) with an active session | EKS clusters, whose kubeconfig contexts authenticate through `aws eks get-token` |
| metrics-server on the cluster (optional) | Usage figures in the cost report |

`manifest` needs only Go. It runs without a cluster or cloud account.

Check your Go version with:

```sh
go version
```

## 1. Get the code

```sh
git clone https://github.com/Kilianrm/kube-cost-workspace.git
cd kube-cost-workspace
```

## 2. Install

From the repository root:

```sh
go install ./cmd/kubeestimate
```

This builds the CLI and places the binary in Go's bin directory:

| System | Location |
|---|---|
| Linux, macOS, WSL | `~/go/bin/kubeestimate` |
| Windows | `%USERPROFILE%\go\bin\kubeestimate.exe` |

To see where it went on your machine, run `go env GOPATH`. The binary is in the `bin` folder inside that path.

> Install from a clone as shown above. `go install github.com/...@latest` does not work for this repository.

## 3. Add Go's bin directory to your PATH

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
cd /mnt/c/Users/<you>/path/to/kube-cost-workspace
go install ./cmd/kubeestimate
```

A binary built in WSL works only in WSL. A Windows build reads the Windows kubeconfig (`%USERPROFILE%\.kube\config`), not the one in WSL.

## 4. Check the installation

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
| `kubeestimate: command not found` or `is not recognized` | Go's bin directory is not on your PATH. Follow [step 3](#3-add-gos-bin-directory-to-your-path) and open a new terminal. |
| `go: command not found` | Go is not installed, or not in this environment. On Windows, check whether Go is installed in WSL or in Windows. |
| `the kubeconfig has no current context` | Pass `--context`. It takes the number, short name or full name shown by `kubeestimate contexts`, or a unique part of the name. |
| `context "<name>" was not found` | Nothing in your kubeconfig matches. Check `kubeestimate contexts`, or pass `--kubeconfig <path>` if the context is in another file. |
| `"<name>" matches 2 contexts` | Two contexts share that short name, for example the ARN and eksctl contexts of one EKS cluster. Use the number from `kubeestimate contexts` or the full name. |
| `this cluster cannot be priced` | The cluster runs locally (kind, minikube, k3s) or in a region missing from the price catalog. `cluster` still works, but `cost` and `simulate` need nodes on AWS, Google Cloud or Azure. For a local cluster, use `manifest` to estimate workloads at cloud prices instead. |
| EKS: `getting credentials: exec: executable aws not found` | Install AWS CLI v2 in the same environment as the CLI (for example inside WSL). |
| EKS: expired token or `AccessDenied` | Log in again, for example `aws sso login --profile <profile>`. Your IAM principal needs Kubernetes access to the cluster. |
| Timeout while collecting the cluster | The API server is unreachable. Check your VPN or network access to private EKS endpoints, and that the cluster still exists. |
| Usage figures are missing from the cost report | Install metrics-server on the cluster. Costs are still calculated from requests without it. |

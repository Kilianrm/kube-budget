#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
CLUSTER_CONFIG="$SCRIPT_DIR/cluster/eks-showcase.yaml"
WORKLOADS_DIR="$SCRIPT_DIR/workloads"
SCENARIOS_DIR="$SCRIPT_DIR/scenarios"
CLUSTER_NAME="kube-budget-showcase"
REGION="us-east-1"
PROFILE="default"
ASSUME_YES=false
METRICS_SERVER_MANIFEST="https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml"

usage() {
  cat <<EOF
Usage: $(basename "$0") <command> [arguments] [options]

Commands:
  create                        Create the cluster, install metrics-server and deploy the showcase workloads.
  status                        Show nodes, workloads, volumes and whether usage metrics are available.
  scenario <name> <on|off>      Switch a demo scenario on or off (see below).
  destroy                       Delete the workloads, their EBS volumes and the whole cluster.

Scenarios:
  missing-requests              Deploy a workload without resource requests ("Fix first" in Optimizations).
  traffic-spike                 Scale checkout-api from 3 to 6 replicas ("What changed?" in Budget & forecast).
  cordoned-node                 Cordon one general node ("Recover or remove nodes" in Optimizations).

Options:
  --region REGION               AWS region (default: $REGION)
  --profile PROFILE             AWS CLI profile (default: $PROFILE)
  --yes                         Skip the destroy confirmation prompt
  -h, --help                    Show this help

Examples:
  ./showcase.sh create
  ./showcase.sh scenario missing-requests on
  ./showcase.sh scenario traffic-spike off
  ./showcase.sh destroy
EOF
}

fail() {
  printf 'Error: %s\n' "$1" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

parse_args() {
  [[ $# -gt 0 ]] || { usage; exit 1; }
  COMMAND=$1
  shift
  POSITIONAL=()

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --region)
        [[ $# -ge 2 ]] || fail "--region requires a value"
        REGION=$2
        shift 2
        ;;
      --profile)
        [[ $# -ge 2 ]] || fail "--profile requires a value"
        PROFILE=$2
        shift 2
        ;;
      --yes)
        ASSUME_YES=true
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      -*)
        fail "unknown option: $1"
        ;;
      *)
        POSITIONAL+=("$1")
        shift
        ;;
    esac
  done

  case "$COMMAND" in
    create|status|destroy) [[ ${#POSITIONAL[@]} -eq 0 ]] || fail "$COMMAND takes no arguments" ;;
    scenario) [[ ${#POSITIONAL[@]} -eq 2 ]] || fail "usage: scenario <name> <on|off>" ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown command: $COMMAND" ;;
  esac
}

aws_eks() {
  aws eks "$@" --region "$REGION" --profile "$PROFILE"
}

cluster_exists() {
  aws_eks describe-cluster --name "$CLUSTER_NAME" --query 'cluster.name' --output text >/dev/null 2>&1
}

# Every kubectl call names this cluster's context, so the script never acts
# on whatever cluster happens to be current.
resolve_context() {
  CONTEXT=$(aws_eks describe-cluster --name "$CLUSTER_NAME" --query 'cluster.arn' --output text)
  [[ -n "$CONTEXT" && "$CONTEXT" != "None" ]] || fail "could not resolve the cluster ARN"
  if ! kubectl config get-contexts "$CONTEXT" >/dev/null 2>&1; then
    aws_eks update-kubeconfig --name "$CLUSTER_NAME" >/dev/null
  fi
}

k() {
  kubectl --context "$CONTEXT" "$@"
}

require_cluster() {
  require_command aws
  require_command kubectl
  cluster_exists || fail "cluster $CLUSTER_NAME does not exist in $REGION; run ./showcase.sh create"
  resolve_context
}

ensure_metrics_server() {
  if k get apiservice v1beta1.metrics.k8s.io >/dev/null 2>&1; then
    printf 'metrics-server already installed.\n'
    return
  fi
  printf 'Installing metrics-server...\n'
  k apply -f "$METRICS_SERVER_MANIFEST"
  k rollout status deployment/metrics-server -n kube-system --timeout=180s
}

create() {
  require_command aws
  require_command eksctl
  require_command kubectl
  aws sts get-caller-identity --profile "$PROFILE" >/dev/null || fail "AWS authentication failed for profile '$PROFILE'"

  if cluster_exists; then
    printf 'Cluster %s already exists; reusing it.\n' "$CLUSTER_NAME"
  else
    printf 'Creating %s in %s (about 15-20 minutes)...\n' "$CLUSTER_NAME" "$REGION"
    eksctl create cluster --config-file "$CLUSTER_CONFIG" --region "$REGION" --profile "$PROFILE"
  fi
  aws_eks update-kubeconfig --name "$CLUSTER_NAME" >/dev/null
  resolve_context

  ensure_metrics_server
  k apply -k "$WORKLOADS_DIR"
  k rollout status deployment/checkout-api -n shop --timeout=300s
  k rollout status deployment/recommendation-engine -n shop --timeout=300s
  k rollout status statefulset/orders-db -n shop --timeout=300s
  k rollout status deployment/queue-worker -n batch --timeout=300s
  k rollout status daemonset/node-agent -n platform --timeout=300s
  k wait --for=condition=complete job/write-old-backup -n shop --timeout=300s

  status
  cat <<EOF

Showcase is ready. Wait a minute or two for metrics-server to sample the pods,
then connect KubeBudget with:
  Connection source: Amazon EKS
  Cluster name:      $CLUSTER_NAME
  AWS region:        $REGION
  AWS profile:       $PROFILE

See README.md for the demo walkthrough.
EOF
}

status() {
  if [[ -z "${CONTEXT:-}" ]]; then
    require_cluster
  fi
  printf 'Cluster: %s (%s)\n\n' "$(aws_eks describe-cluster --name "$CLUSTER_NAME" --query 'cluster.status' --output text)" "$CONTEXT"
  k get nodes -L eks.amazonaws.com/nodegroup,node.kubernetes.io/instance-type,eks.amazonaws.com/capacityType
  printf '\n'
  k get pods -n shop -o wide
  k get pods -n batch -o wide
  k get pods -n platform -o wide
  printf '\n'
  k get pvc -A
  printf '\n'
  if k top pods -n shop >/dev/null 2>&1; then
    printf 'Usage metrics: available\n'
  else
    printf 'Usage metrics: not available yet (metrics-server needs a minute after pods start)\n'
  fi
}

scenario() {
  require_cluster
  local name=$1 state=$2
  [[ "$state" == on || "$state" == off ]] || fail "scenario state must be on or off"

  case "$name" in
    missing-requests)
      if [[ "$state" == on ]]; then
        k apply -f "$SCENARIOS_DIR/missing-requests/"
      else
        k delete -f "$SCENARIOS_DIR/missing-requests/" --ignore-not-found=true --wait=true
      fi
      ;;
    traffic-spike)
      local replicas=3
      [[ "$state" == on ]] && replicas=6
      k scale deployment/checkout-api -n shop --replicas="$replicas"
      k rollout status deployment/checkout-api -n shop --timeout=300s
      ;;
    cordoned-node)
      if [[ "$state" == on ]]; then
        local node
        node=$(k get nodes -l workload-tier=general -o jsonpath='{.items[0].metadata.name}')
        [[ -n "$node" ]] || fail "no general node found"
        k cordon "$node"
      else
        k uncordon -l workload-tier=general
      fi
      ;;
    *)
      fail "unknown scenario: $name (missing-requests, traffic-spike, cordoned-node)"
      ;;
  esac
  printf '\nScenario %s is %s. Refresh the cost report in KubeBudget.\n' "$name" "$state"
}

destroy() {
  require_command aws
  require_command eksctl
  require_command kubectl

  if ! cluster_exists; then
    printf 'Cluster %s does not exist in %s.\n' "$CLUSTER_NAME" "$REGION"
    return
  fi
  resolve_context

  if [[ "$ASSUME_YES" != true ]]; then
    printf 'This deletes the %s cluster, its workloads and their EBS volumes.\n' "$CLUSTER_NAME"
    printf 'Type %s to continue: ' "$CLUSTER_NAME"
    read -r confirmation
    [[ "$confirmation" == "$CLUSTER_NAME" ]] || fail "destroy cancelled"
  fi

  # Dynamically provisioned EBS volumes outlive the cluster unless their claims
  # are deleted first, and they keep billing.
  printf 'Deleting workloads and volume claims...\n'
  k delete -f "$SCENARIOS_DIR/missing-requests/" --ignore-not-found=true >/dev/null 2>&1 || true
  k delete -k "$WORKLOADS_DIR" --ignore-not-found=true --wait=true || true
  k delete pvc --all -n shop --ignore-not-found=true --wait=true || true
  for _ in $(seq 1 30); do
    if [[ -z "$(k get pv -o name 2>/dev/null)" ]]; then
      break
    fi
    sleep 10
  done
  if [[ -n "$(k get pv -o name 2>/dev/null)" ]]; then
    printf 'Warning: some persistent volumes still exist; check the EC2 console for leftover EBS volumes.\n'
  fi

  eksctl delete cluster --config-file "$CLUSTER_CONFIG" --region "$REGION" --profile "$PROFILE" --wait
  printf 'Cluster deleted.\n'
}

parse_args "$@"
case "$COMMAND" in
  create) create ;;
  status) status ;;
  scenario) scenario "${POSITIONAL[0]}" "${POSITIONAL[1]}" ;;
  destroy) destroy ;;
esac

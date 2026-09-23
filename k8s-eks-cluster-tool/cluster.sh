#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
CLUSTER_CONFIG="$SCRIPT_DIR/eks-cluster.yaml"
RESOURCES_MANIFEST="$SCRIPT_DIR/cluster-resources.yaml"
CLUSTER_NAME="cluster-to-be-monitorized"
REGION="us-east-1"
PROFILE="default"
ASSUME_YES=false

usage() {
  cat <<EOF
Usage: $(basename "$0") <create|destroy|status> [options]

Commands:
  create                 Create or reuse the EKS cluster and deploy demo resources.
  destroy                Delete demo resources and the entire EKS cluster.
  status                 Show cluster, node, add-on, and workload status.

Options:
  --region REGION       AWS region (default: $REGION)
  --profile PROFILE     AWS CLI profile (default: $PROFILE)
  --yes                 Skip the destroy confirmation prompt
  -h, --help            Show this help

Examples:
  ./cluster.sh create
  ./cluster.sh create --profile default --region us-east-1
  ./cluster.sh status
  ./cluster.sh destroy
  ./cluster.sh destroy --yes
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
      *)
        fail "unknown option: $1"
        ;;
    esac
  done

  case "$COMMAND" in
    create|destroy|status) ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown command: $COMMAND" ;;
  esac
}

require_create_tools() {
  require_command aws
  require_command eksctl
  require_command kubectl
  [[ -f "$CLUSTER_CONFIG" ]] || fail "cluster config not found: $CLUSTER_CONFIG"
  [[ -f "$RESOURCES_MANIFEST" ]] || fail "resources manifest not found: $RESOURCES_MANIFEST"
}

cluster_exists() {
  aws eks describe-cluster \
    --name "$CLUSTER_NAME" \
    --region "$REGION" \
    --profile "$PROFILE" \
    --query 'cluster.name' \
    --output text >/dev/null 2>&1
}

update_kubeconfig() {
  aws eks update-kubeconfig \
    --name "$CLUSTER_NAME" \
    --region "$REGION" \
    --profile "$PROFILE"
}

create() {
  require_create_tools
  aws sts get-caller-identity --profile "$PROFILE" >/dev/null || fail "AWS authentication failed for profile '$PROFILE'"

  if cluster_exists; then
    printf 'EKS cluster already exists; reusing %s.\n' "$CLUSTER_NAME"
  else
    printf 'Creating EKS cluster %s in %s...\n' "$CLUSTER_NAME" "$REGION"
    eksctl create cluster \
      --config-file "$CLUSTER_CONFIG" \
      --region "$REGION" \
      --profile "$PROFILE"
  fi

  update_kubeconfig
  kubectl config use-context "$(kubectl config current-context)" >/dev/null
  kubectl apply -f "$RESOURCES_MANIFEST"
  kubectl rollout status deployment/demo-api -n monitoring-demo --timeout=180s
  kubectl get nodes
  kubectl get all -n monitoring-demo
  kubectl get pvc -n monitoring-demo
  printf '\nCluster is ready.\n'
}

destroy() {
  require_command aws
  require_command eksctl
  require_command kubectl

  if [[ "$ASSUME_YES" != true ]]; then
    printf 'This will delete the EKS cluster and all demo resources in AWS.\n'
    printf 'Type %s to continue: ' "$CLUSTER_NAME"
    read -r confirmation
    [[ "$confirmation" == "$CLUSTER_NAME" ]] || fail "destroy cancelled"
  fi

  if cluster_exists; then
    kubectl delete -f "$RESOURCES_MANIFEST" --ignore-not-found=true >/dev/null 2>&1 || true
    eksctl delete cluster \
      --config-file "$CLUSTER_CONFIG" \
      --region "$REGION" \
      --profile "$PROFILE" \
      --wait
    printf 'EKS cluster deleted.\n'
  else
    printf 'EKS cluster %s does not exist.\n' "$CLUSTER_NAME"
  fi
}

status() {
  require_command aws
  require_command kubectl

  if ! cluster_exists; then
    printf 'EKS cluster %s does not exist in %s.\n' "$CLUSTER_NAME" "$REGION"
    exit 0
  fi

  aws eks describe-cluster \
    --name "$CLUSTER_NAME" \
    --region "$REGION" \
    --profile "$PROFILE" \
    --query 'cluster.status' \
    --output text
  kubectl get nodes
  kubectl get pods -n monitoring-demo
  aws eks describe-addon \
    --cluster-name "$CLUSTER_NAME" \
    --addon-name aws-ebs-csi-driver \
    --region "$REGION" \
    --profile "$PROFILE" \
    --query 'addon.status' \
    --output text 2>/dev/null || printf 'EBS CSI add-on status unavailable.\n'
}

parse_args "$@"
case "$COMMAND" in
  create) create ;;
  destroy) destroy ;;
  status) status ;;
esac

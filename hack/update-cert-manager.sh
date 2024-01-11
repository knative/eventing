#!/usr/bin/env bash

set -euo pipefail

function update_cert_manager() {
  cert_manager_version="$1"
  trust_manager_version="$2"
  echo "Updating Cert Manager to version $cert_manager_version"
  echo "Updating Trust Manager to version $trust_manager_version"

  rm -rf third_party/cert-manager
  mkdir -p third_party/cert-manager

  ensure_helm_available

  helm repo add jetstack https://charts.jetstack.io --force-update
  kubectl create namespace --dry-run=client cert-manager -oyaml > third_party/cert-manager/00-namespace.yaml
  helm template -n cert-manager cert-manager jetstack/cert-manager --create-namespace --version "${cert_manager_version}" --set installCRDs=true > third_party/cert-manager/01-cert-manager.yaml
  helm template -n cert-manager cert-manager jetstack/trust-manager --create-namespace --version "${trust_manager_version}" --set installCRDs=true > third_party/cert-manager/02-trust-manager.yaml
}

function ensure_helm_available() {
  helm_bin="$(which helm 2>/dev/null || echo "")"

  if [ -n "$helm_bin" ]; then
    echo "Helm already installed on system"
  else
    echo "Helm binary not found. Installing into tmp dir..."

    curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
    chmod 700 get_helm.sh

    export HELM_INSTALL_DIR="$(mktemp -dt helm-XXXXXX)"
    export PATH=$PATH:$HELM_INSTALL_DIR

    ./get_helm.sh --no-sudo

    rm -f ./get_helm.sh
  fi
}

update_cert_manager "v1.13.3" "v0.7.1"

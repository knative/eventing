#!/usr/bin/env bash

set -euo pipefail

function update_cert_manager() {
  cert_manager_version="$1"
  trust_manager_version="$2"
  echo "Updating Cert Manager to version $cert_manager_version"
  echo "Updating Trust Manager to version $trust_manager_version"

  rm -rf third_party/cert-manager
  mkdir -p third_party/cert-manager

  helm repo add jetstack https://charts.jetstack.io --force-update

  cat > third_party/cert-manager/00-namespace.yaml <<EOF
apiVersion: v1
kind: Namespace
metadata:
  name: cert-manager
EOF

  helm template -n cert-manager cert-manager jetstack/cert-manager --create-namespace --version "${cert_manager_version}" --set installCRDs=true > third_party/cert-manager/01-cert-manager.yaml
  helm template -n cert-manager cert-manager jetstack/trust-manager --create-namespace --version "${trust_manager_version}" --set crds.enabled=true > third_party/cert-manager/02-trust-manager.yaml
}

update_cert_manager "v1.16.3" "v0.12.0"

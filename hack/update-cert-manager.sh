#!/usr/bin/env bash

set -euo pipefail

function update_cert_manager() {
  version="$1"
  echo "Updating Cert Manager to version $version"
  rm -rf third_party/cert-manager
  mkdir -p third_party/cert-manager
  curl -L https://github.com/cert-manager/cert-manager/releases/download/$version/cert-manager.yaml >third_party/cert-manager/01-cert-manager.crds.yaml
  curl -L https://github.com/cert-manager/cert-manager/releases/download/$version/cert-manager.yaml >third_party/cert-manager/02-cert-manager.yaml
}

update_cert_manager "v1.11.1"

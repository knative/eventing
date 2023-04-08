#!/usr/bin/env bash

set -euo pipefail

function update_certmanager() {
  echo "Updating Cert Manager"
  kubectl apply -f "$(curl -s https://api.github.com/repos/cert-manager/cert-manager/releases/latest | jq '.assets[] | select(.name|match("cert-manager.yaml$")) | .browser_download_url')"
}

update_certmanager 
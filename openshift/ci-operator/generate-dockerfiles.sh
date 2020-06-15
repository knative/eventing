#!/bin/bash

set -e

function generate_dockefiles() {
  local target_dir=$1; shift
  # Remove old images and re-generate, avoid stale images hanging around.
  for img in $@; do
    local image_base=$(basename $img)
    mkdir -p $target_dir/$image_base
    bin=$image_base envsubst < openshift/ci-operator/Dockerfile.in > $target_dir/$image_base/Dockerfile
  done
}

generate_dockefiles $@

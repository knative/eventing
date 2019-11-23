#!/bin/bash

# Copyright 2019 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# performance-tests.sh is added to manage all clusters that run the performance
# benchmarks in eventing repo, it is ONLY intended to be run by Prow, users
# should NOT run it manually.

# Setup env vars to override the default settings
export PROJECT_NAME="knative-eventing-performance"
export BENCHMARK_ROOT_PATH="$GOPATH/src/knative.dev/eventing/test/performance/benchmarks"

source vendor/knative.dev/test-infra/scripts/performance-tests.sh

# Vars used in this script
export TEST_CONFIG_VARIANT="continuous"
export TEST_NAMESPACE="default"

function update_knative() {
  echo ">> Update eventing core"
  ko apply --selector knative.dev/crd-install=true \
    -f config/ || abort "Failed to apply eventing CRDs"

  ko apply \
    -f config/ || abort "Failed to apply eventing resources"

  echo ">> Update InMemoryChannel"
  ko apply --selector knative.dev/crd-install=true \
    -f config/channels/in-memory-channel/ || abort "Failed to apply InMemoryChannel CRDs"

  ko apply \
    -f config/channels/in-memory-channel/ || abort "Failed to apply InMemoryChannel resources"
}

function update_benchmark() {
  local benchmark_path="${BENCHMARK_ROOT_PATH}/$1"
  # TODO(chizhg): add update_environment function in test-infra/scripts/performance-tests.sh and move the below code there
  echo ">> Updating configmap"
  kubectl delete configmap config-mako -n "${TEST_NAMESPACE}" --ignore-not-found=true
  kubectl create configmap config-mako -n "${TEST_NAMESPACE}" --from-file="${benchmark_path}/prod.config" || abort "failed to create config-mako configmap"
  kubectl patch configmap config-mako -n "${TEST_NAMESPACE}" -p '{"data":{"environment":"prod"}}' || abort "failed to patch config-mako configmap"

  echo ">> Updating benchmark $1"
  ko delete -f "${benchmark_path}"/${TEST_CONFIG_VARIANT} --ignore-not-found=true
  ko apply -f "${benchmark_path}"/${TEST_CONFIG_VARIANT} || abort "failed to apply benchmark $1"
}

main $@

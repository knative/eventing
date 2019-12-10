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

# This script runs the end-to-end tests against eventing built from source.

# If you already have the *_OVERRIDE environment variables set, call
# this script with the --run-tests arguments and it will use the cluster
# and run the tests.

# Calling this script without arguments will create a new cluster in
# project $PROJECT_ID, start Knative eventing system, run the tests and
# delete the cluster.

source $(dirname $0)/e2e-common.sh

# Override functions to install multitenant controllers

# Multi-tenant In-memory channel CRD config.
readonly MULTI_TENANT_IN_MEMORY_CHANNEL_CRD_CONFIG_DIR="config/channels/in-memory-channel-multitenant"

function install_channel_crds() {
  echo "Installing Multi-Tenant In-Memory Channel CRD"
  ko apply -f ${MULTI_TENANT_IN_MEMORY_CHANNEL_CRD_CONFIG_DIR} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the Multi-Tenant In-Memory Channel CRD"
}

function uninstall_channel_crds() {
  echo "Uninstalling Multi-Tenant In-Memory Channel CRD"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${MULTI_TENANT_IN_MEMORY_CHANNEL_CRD_CONFIG_DIR}
}

# Script entry point.

initialize $@ --skip-istio-addon

go_test_e2e -timeout=30m -parallel=12 ./test/e2e ./test/conformance || fail_test

success

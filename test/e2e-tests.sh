#!/bin/bash

# Copyright 2018 The Knative Authors
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
# project $PROJECT_ID, start Knative serving and the eventing system, run
# the tests and delete the cluster.

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/e2e-tests.sh

# Helper functions.

# Setup the Knative environment for running tests
function knative_setup() {
  # Install the latest stable Knative/serving in the current cluster
  start_latest_knative_serving || return 1

  # Install the latest Knative/eventing in the current cluster
  echo ">> Starting Knative Eventing"
  echo "Installing Knative Eventing"
  ko apply -f config/ || return 1
  wait_until_pods_running knative-eventing || fail_test "Knative Eventing did not come up"

  echo "Installing In-Memory ClusterChannelProvisioner"
  ko apply -f config/provisioners/in-memory-channel/in-memory-channel.yaml || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the In-Memory ClusterChannelProvisioner"
}

function knative_teardown() {
  echo ">> Stopping Knative Eventing"
  echo "Uninstalling Knative Eventing"
  ko delete --ignore-not-found=true --now --timeout 60s -f config/

  wait_until_object_does_not_exist namespaces knative-eventing

  wait_until_object_does_not_exist customresourcedefinitions subscriptions.eventing.knative.dev
  wait_until_object_does_not_exist customresourcedefinitions channels.eventing.knative.dev
}

# Setup resources common to all eventing tests
function test_setup() {
  # Publish test images
  echo ">> Publishing test images"
  $(dirname $0)/upload-test-images.sh e2e || fail_test "Error uploading test images"
}

function dump_extra_cluster_state() {
  # Collecting logs from all knative's eventing pods
  echo "============================================================"
  for namespace in "knative-eventing" "e2etestfn3"; do
    for pod in $(kubectl get pod -n $namespace | grep Running | awk '{print $1}' ); do
      for container in $(kubectl get pod "${pod}" -n $namespace -ojsonpath='{.spec.containers[*].name}'); do
        echo "Namespace, Pod, Container: ${namespace}, ${pod}, ${container}"
        kubectl logs -n $namespace "${pod}" -c "${container}" || true
        echo "----------------------------------------------------------"
        echo "Namespace, Pod, Container (Previous instance): ${namespace}, ${pod}, ${container}"
        kubectl logs -p -n $namespace "${pod}" -c "${container}" || true
        echo "============================================================"
      done
    done
  done
}

# Script entry point.

initialize $@

go_test_e2e -timeout=20m ./test/e2e -run ^TestMain$ -runFromMain=true -clusterChannelProvisioners=in-memory-channel || fail_test

success

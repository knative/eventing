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

# Names of the Resources used in the tests.
# Currently this namespace must be the same as the namespace specified in
# test/e2e/e2e.go.
readonly E2E_TEST_NAMESPACE=e2etest-knative-eventing

# Helper functions.

function teardown() {
  teardown_events_test_resources
  ko delete --ignore-not-found=true -f config/

  wait_until_object_does_not_exist namespaces knative-eventing

  wait_until_object_does_not_exist customresourcedefinitions subscriptions.eventing.knative.dev
  wait_until_object_does_not_exist customresourcedefinitions channels.eventing.knative.dev
}

function setup_events_test_resources() {
  kubectl create namespace ${E2E_TEST_NAMESPACE}
}

function teardown_events_test_resources() {
  # Delete the test namespace
  echo "Deleting namespace $E2E_TEST_NAMESPACE"
  kubectl --ignore-not-found=true delete namespace ${E2E_TEST_NAMESPACE}
  wait_until_object_does_not_exist namespaces ${E2E_TEST_NAMESPACE} || return 1
}

# Script entry point.

initialize $@

header "Setting up environment"

# Install Knative Serving if not using an existing cluster
if (( ! USING_EXISTING_CLUSTER )); then
  start_latest_knative_serving || fail_test "Serving did not come up"
fi

# Clean up anything that might still be around
teardown_events_test_resources || fail_test "Error cleaning up test resources"

ko apply -f config/
wait_until_pods_running knative-eventing || fail_test "Eventing did not come up (1)"

subheader "Standing up In-Memory ClusterChannelProvisioner"
ko apply -f config/provisioners/in-memory-channel/in-memory-channel.yaml
wait_until_pods_running knative-eventing || fail_test "Eventing did not come up (2)"

# Publish test images
$(dirname $0)/upload-test-images.sh e2e || fail_test "Error uploading test images"

# Setup resources common to all eventing tests
setup_events_test_resources|| fail_test "Error setting up test resources"

go_test_e2e ./test/e2e 
exit_result=$?
if [ ${exit_result} -ne 0 ]; then
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
  fail_test
fi

success

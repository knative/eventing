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

# This script runs the end-to-end tests against the eventing
# built from source.

# If you already have the *_OVERRIDE environment variables set, call
# this script with the --run-tests arguments and it will use the cluster
# and run the tests.

# Calling this script without arguments will create a new cluster in
# project $PROJECT_ID, start Knative serving and the eventing system, run
# the tests and delete the cluster.

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/e2e-tests.sh

# Using the most recent good release of eventing-sources to unblock tests. This
# should be replaced with the commented line below when eventing-sources nightly
# is known good again.
readonly KNATIVE_EVENTING_SOURCES_RELEASE=https://knative-nightly.storage.googleapis.com/eventing-sources/previous/v20181205-fbac942/release.yaml
#readonly KNATIVE_EVENTING_SOURCES_RELEASE=https://knative-nightly.storage.googleapis.com/eventing-sources/latest/release.yaml

# Names of the Resources used in the tests.
readonly E2E_TEST_NAMESPACE=e2etest
readonly E2E_TEST_FUNCTION_NAMESPACE=e2etestfn3

# Helper functions.

# Install the latest stable Knative/serving in the current cluster.
function start_latest_eventing_sources() {
  header "Starting Knative Eventing Sources"
  subheader "Installing Knative Eventing Sources"
  kubectl apply -f ${KNATIVE_EVENTING_SOURCES_RELEASE} || return 1
  wait_until_pods_running knative-sources || return 1
}


function teardown() {
  teardown_events_test_resources
#  ko delete --ignore-not-found=true -f config/provisioners/in-memory-channel/in-memory-channel.yaml
  ko delete --ignore-not-found=true -f config/
  ko delete --ignore-not-found=true -f ${KNATIVE_EVENTING_SOURCES_RELEASE}

  wait_until_object_does_not_exist namespaces knative-eventing
  wait_until_object_does_not_exist namespaces knative-sources

  wait_until_object_does_not_exist customresourcedefinitions subscriptions.eventing.knative.dev
  wait_until_object_does_not_exist customresourcedefinitions channels.eventing.knative.dev
}

function setup_events_test_resources() {
  kubectl create namespace $E2E_TEST_NAMESPACE
  kubectl create namespace $E2E_TEST_FUNCTION_NAMESPACE
}

function teardown_events_test_resources() {
  # Delete the function namespace
  echo "Deleting namespace $E2E_TEST_FUNCTION_NAMESPACE"
  kubectl --ignore-not-found=true delete namespace $E2E_TEST_FUNCTION_NAMESPACE
  wait_until_object_does_not_exist namespaces $E2E_TEST_FUNCTION_NAMESPACE || return 1

  # Delete the test namespace
  echo "Deleting namespace $E2E_TEST_NAMESPACE"
  kubectl --ignore-not-found=true delete namespace $E2E_TEST_NAMESPACE
  wait_until_object_does_not_exist namespaces $E2E_TEST_NAMESPACE
}

# Script entry point.

initialize $@

# Install Knative Serving if not using an existing cluster
if (( ! USING_EXISTING_CLUSTER )); then
  start_latest_knative_serving || fail_test
fi

# Install Knative Eventing Sources
start_latest_eventing_sources || fail_test

# Clean up anything that might still be around
teardown_events_test_resources

# Fail fast during setup.
set -o errexit
set -o pipefail

header "Standing up Knative Eventing"
export KO_DOCKER_REPO=${DOCKER_REPO_OVERRIDE}
ko resolve -f config/
ko apply -f config/
wait_until_pods_running knative-eventing

header "Standing up In-Memory ClusterChannelProvisioner"
ko resolve -f config/provisioners/in-memory-channel/in-memory-channel.yaml
ko apply -f config/provisioners/in-memory-channel/in-memory-channel.yaml
wait_until_pods_running knative-eventing

# Publish test images
$(dirname $0)/upload-test-images.sh e2e

# Handle test failures ourselves, so we can dump useful info.
set +o errexit
set +o pipefail

# Setup resources common to all eventing tests
setup_events_test_resources

go_test_e2e ./test/e2e || fail_test

success

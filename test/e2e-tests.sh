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

# Load github.com/knative/test-infra/images/prow-tests/scripts/e2e-tests.sh
[ -f /workspace/e2e-tests.sh ] \
  && source /workspace/e2e-tests.sh \
  || eval "$(docker run --entrypoint sh gcr.io/knative-tests/test-infra/prow-tests -c 'cat e2e-tests.sh')"
[ -v KNATIVE_TEST_INFRA ] || exit 1

# Names of the Resources used in the tests.
readonly E2E_TEST_NAMESPACE=e2etest
readonly E2E_TEST_FUNCTION_NAMESPACE=e2etestfn
readonly E2E_TEST_FUNCTION=e2e-k8s-events-function

# Helper functions.

function teardown() {
  ko delete --ignore-not-found=true -f config/
}

function wait_until_feed_ready() {
  local NAMESPACE=$1
  local NAME=$2

  echo -n "Waiting until feed $NAMESPACE/$NAME is ready"
  for i in {1..150}; do  # timeout after 5 minutes
    local reason="$(kubectl get -n $NAMESPACE feeds $NAME -o 'jsonpath={.status.conditions[0].reason}')"
    local status="$(kubectl get -n $NAMESPACE feeds $NAME -o 'jsonpath={.status.conditions[0].status}')"

    if [ "$reason" = "FeedSuccess" ]; then
       if [ "$status" = "True" ]; then
          return 0
       fi
    fi
    echo -n "."
    sleep 2
  done
  echo -e "\n\nERROR: timeout waiting for feed $NAMESPACE/$NAME to be ready"
  kubectl get -n $NAMESPACE feeds $NAME -oyaml
  kubectl get -n $NAMESPACE jobs $NAME-start -oyaml
  return 1
}

function validate_function_logs() {
  local NAMESPACE=$1
  local podname="$(kubectl -n $NAMESPACE get pods --no-headers -oname | grep e2e-k8s-events-function)"
  local logs="$(kubectl -n $NAMESPACE logs $podname user-container)"
  echo "${logs}" | grep "Started container" || return 1
  echo "${logs}" | grep "Created container" || return 1
  return 0
}

function teardown_k8s_events_test_resources() {
  echo "Deleting any previously existing feed"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/feed-channel.yaml
  wait_until_object_does_not_exist feed $E2E_TEST_FUNCTION_NAMESPACE receiveevent

  # Delete the function resources and namespace
  echo "Deleting function and test namespace"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/function.yaml
  wait_until_object_does_not_exist route $E2E_TEST_FUNCTION_NAMESPACE $E2E_TEST_FUNCTION

  echo "Deleting k8s events event source"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/k8sevents.yaml
  wait_until_object_does_not_exist eventsources $E2E_TEST_FUNCTION_NAMESPACE k8sevents || return 1
  wait_until_object_does_not_exist eventtypes $E2E_TEST_FUNCTION_NAMESPACE receiveevent || return 1

  # Delete the pod from the test namespace
  echo "Deleting test pod"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/pod.yaml

  # Delete the channel and subscription
  echo "Deleting subscription"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/subscription.yaml
  echo "Deleting channel"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/channel.yaml

  # Delete the service account and role binding
  echo "Deleting cluster role binding"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/serviceaccountbinding.yaml
  echo "Deleting service account"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/serviceaccount.yaml

  # Delete the function namespace
  echo "Deleting namespace $E2E_TEST_FUNCTION_NAMESPACE"
  kubectl --ignore-not-found=true delete namespace $E2E_TEST_FUNCTION_NAMESPACE
  wait_until_object_does_not_exist namespaces $E2E_TEST_FUNCTION_NAMESPACE || return 1

  # Delete the test namespace
  echo "Deleting namespace $E2E_TEST_NAMESPACE"
  kubectl --ignore-not-found=true delete namespace $E2E_TEST_NAMESPACE
  wait_until_object_does_not_exist namespaces $E2E_TEST_NAMESPACE || return 1
}

# Tests

function run_k8s_events_test() {
  header "Running 'k8s events' test"
  echo "Creating namespace $E2E_TEST_FUNCTION_NAMESPACE"
  ko apply -f test/e2e/k8sevents/e2etestnamespace.yaml || return 1
  echo "Creating namespace $E2E_TEST_NAMESPACE"
  ko apply -f test/e2e/k8sevents/e2etestfnnamespace.yaml || return 1

  # Install service account and role binding
  echo "Installing service account"
  ko apply -f test/e2e/k8sevents/serviceaccount.yaml || return 1

  echo "Installing role binding"
  ko apply -f test/e2e/k8sevents/serviceaccountbinding.yaml || return 1

  # Install stub bus
  echo "Installing stub bus"
  ko apply -f test/e2e/k8sevents/stub.yaml || return 1

  # Install k8s events as an event source
  echo "Installing k8s events as an event source"
  ko apply -f test/e2e/k8sevents/k8sevents.yaml || return 1

  # Launch the function
  echo "Installing the receiving function"
  ko apply -f test/e2e/k8sevents/function.yaml || return 1
  wait_until_pods_running $E2E_TEST_FUNCTION_NAMESPACE || return 1

  # create a channel and subscription
  echo "Creating a channel"
  ko apply -f test/e2e/k8sevents/channel.yaml || return 1
  echo "Creating a subscription"
  ko apply -f test/e2e/k8sevents/subscription.yaml || return 1

  # Install feed
  echo "Creating a feed"
  ko apply -f test/e2e/k8sevents/feed-channel.yaml || return 1
  wait_until_feed_ready $E2E_TEST_FUNCTION_NAMESPACE e2e-k8s-events-example || return 1

  # Work around for: https://github.com/knative/eventing/issues/125
  # and the fact that even after pods are up, due to Istio slowdown, there's
  # about 5-6 seconds that traffic won't be passed through.
  echo "Waiting until receive_adapter up"
  wait_until_pods_running $E2E_TEST_FUNCTION_NAMESPACE || return 1
  sleep 10

  # Launch the pod into the test namespace
  echo "Creating a pod in the test namespace"
  ko apply -f test/e2e/k8sevents/pod.yaml || return 1
  wait_until_pods_running $E2E_TEST_NAMESPACE || return 1

  # Check the logs to make sure messages made to our function
  echo "Validating that the function received the expected events"
  validate_function_logs $E2E_TEST_FUNCTION_NAMESPACE || return 1
}

# Script entry point.

initialize $@

# Install Knative Serving if not using an existing cluster
if (( ! USING_EXISTING_CLUSTER )); then
  start_latest_knative_serving || fail_test
fi

# Clean up anything that might still be around
teardown_k8s_events_test_resources

if (( USING_EXISTING_CLUSTER )); then
  subheader "Deleting any previous eventing instance"
  ko delete --ignore-not-found=true -f config/
  wait_until_object_does_not_exist namespaces knative-eventing
  wait_until_object_does_not_exist customresourcedefinitions feeds.feeds.knative.dev
fi

# Fail fast during setup.
set -o errexit
set -o pipefail

header "Standing up Knative Eventing"
export KO_DOCKER_REPO=${DOCKER_REPO_OVERRIDE}
ko resolve -f config/
ko apply -f config/
wait_until_pods_running knative-eventing

# Handle test failures ourselves, so we can dump useful info.
set +o errexit
set +o pipefail

run_k8s_events_test || fail_test

success

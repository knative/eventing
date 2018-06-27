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
# $KO_DOCKER_REPO must point to a valid writable docker repo.

source "$(dirname $(readlink -f ${BASH_SOURCE}))/library.sh"

# Test cluster parameters and location of test files
readonly E2E_CLUSTER_NAME=eventing-e2e-cluster${BUILD_NUMBER}
readonly E2E_NETWORK_NAME=eventing-e2e-net${BUILD_NUMBER}
readonly E2E_CLUSTER_ZONE=us-central1-a
readonly E2E_CLUSTER_NODES=2
readonly E2E_CLUSTER_MACHINE=n1-standard-2
readonly TEST_RESULT_FILE=/tmp/eventing-e2e-result
readonly ISTIO_YAML=https://storage.googleapis.com/knative-releases/latest/istio.yaml
readonly SERVING_RELEASE=https://storage.googleapis.com/knative-releases/latest/release.yaml
readonly E2E_TEST_NAMESPACE=e2etest
readonly E2E_TEST_FUNCTION_NAMESPACE=e2etestfn
readonly E2E_TEST_FUNCTION=e2e-k8s-events-function

# This script.
readonly SCRIPT_CANONICAL_PATH="$(readlink -f ${BASH_SOURCE})"

# Helper functions.

function teardown() {
  header "Tearing down test environment"
  # Free resources in GCP project.
  if (( ! USING_EXISTING_CLUSTER )); then
    ko delete --ignore-not-found=true -f config/
  fi

  # Delete images when using prow.
  if (( IS_PROW )); then
    echo "Images in ${KO_DOCKER_REPO}:"
    gcloud container images list --repository=${KO_DOCKER_REPO}
    delete_gcr_images ${KO_DOCKER_REPO}
  else
    restore_override_vars
  fi
}

function exit_if_test_failed() {
  [[ $? -eq 0 ]] && return 0
  [[ -n $1 ]] && echo "ERROR: $1"
  echo "***************************************"
  echo "***           TEST FAILED           ***"
  echo "***    Start of information dump    ***"
  echo "***************************************"
  if (( IS_PROW )) || [[ $PROJECT_ID != "" ]]; then
    echo ">>> Project info:"
    gcloud compute project-info describe
  fi
  echo ">>> All resources:"
  kubectl get all --all-namespaces
  echo "***************************************"
  echo "***           TEST FAILED           ***"
  echo "***     End of information dump     ***"
  echo "***************************************"
  exit 1
}

# Tests
function teardown_k8s_events_test_resources() {
  echo "Deleting any previously existing bind"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/bind-channel.yaml
  wait_until_object_does_not_exist bind $E2E_TEST_FUNCTION_NAMESPACE receiveevent

  # Delete the function resources and namespace
  echo "Deleting function and test namespace"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/function.yaml
  wait_until_object_does_not_exist route $E2E_TEST_FUNCTION_NAMESPACE $E2E_TEST_FUNCTION

  echo "Deleting k8s events event source"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/k8sevents.yaml
  wait_until_object_does_not_exist eventsources $E2E_TEST_FUNCTION_NAMESPACE k8sevents
  exit_if_test_failed
  wait_until_object_does_not_exist eventtypes $E2E_TEST_FUNCTION_NAMESPACE receiveevent
  exit_if_test_failed

  # Delete the pod from the test namespace
  echo "Deleting test pod"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/pod.yaml

  # Delete the channel and subscription
  echo "Deleting subscription"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/subscription.yaml
  echo "Deleting channel"
  ko delete -f test/e2e/k8sevents/channel.yaml

  # Delete the service account and role binding
  echo "Deleting cluster role binding"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/serviceaccountbinding.yaml
  echo "Deleting service account"
  ko delete --ignore-not-found=true -f test/e2e/k8sevents/serviceaccount.yaml

  # Delete the function namespace
  echo "Deleting namespace $E2E_TEST_FUNCTION_NAMESPACE"
  kubectl --ignore-not-found=true delete namespace $E2E_TEST_FUNCTION_NAMESPACE
  wait_until_namespace_does_not_exist $E2E_TEST_FUNCTION_NAMESPACE
  exit_if_test_failed

  # Delete the test namespace
  echo "Deleting namespace $E2E_TEST_NAMESPACE"
  kubectl --ignore-not-found=true delete namespace $E2E_TEST_NAMESPACE
  wait_until_namespace_does_not_exist $E2E_TEST_NAMESPACE
  exit_if_test_failed
}

function run_k8s_events_test() {
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

  # launch the function
  echo "Installing the receiving function"
  ko apply -f test/e2e/k8sevents/function.yaml || return 1
  wait_until_pods_running $E2E_TEST_FUNCTION_NAMESPACE
  exit_if_test_failed

  # create a channel and subscription
  echo "Creating a channel"
  ko apply -f test/e2e/k8sevents/channel.yaml || return 1
  echo "Creating a subscription"
  ko apply -f test/e2e/k8sevents/subscription.yaml || return 1


  # Install binding
  echo "Creating a binding"
  ko apply -f test/e2e/k8sevents/bind-channel.yaml || return 1
  wait_until_bind_ready $E2E_TEST_FUNCTION_NAMESPACE e2e-k8s-events-example
  exit_if_test_failed

  # Work around for: https://github.com/knative/eventing/issues/125
  # and the fact that even after pods are up, due to Istio slowdown, there's
  # about 5-6 seconds that traffic won't be passed through.
  echo "Waiting until receive_adapter up"
  wait_until_pods_running $E2E_TEST_FUNCTION_NAMESPACE
  sleep 10

  # Launch the pod into the test namespace
  echo "Creating a pod in the test namespace"
  ko apply -f test/e2e/k8sevents/pod.yaml || return 1
  wait_until_pods_running $E2E_TEST_NAMESPACE
  exit_if_test_failed

  # Check the logs to make sure messages made to our function
  echo "Validating that the function received the expected events"
  validate_function_logs $E2E_TEST_FUNCTION_NAMESPACE
  exit_if_test_failed
}

# Script entry point.

cd ${EVENTING_ROOT_DIR}

# Show help if bad arguments are passed.
if [[ -n $1 && $1 != "--run-tests" ]]; then
  echo "usage: $0 [--run-tests]"
  exit 1
fi

# No argument provided, create the test cluster.

if [[ -z $1 ]]; then
  header "Creating test cluster"
  # Smallest cluster required to run the end-to-end-tests
  CLUSTER_CREATION_ARGS=(
    --gke-create-args="--enable-autoscaling --min-nodes=1 --max-nodes=${E2E_CLUSTER_NODES} --scopes=cloud-platform"
    --gke-shape={\"default\":{\"Nodes\":${E2E_CLUSTER_NODES}\,\"MachineType\":\"${E2E_CLUSTER_MACHINE}\"}}
    --provider=gke
    --deployment=gke
    --gcp-node-image=cos
    --cluster="${E2E_CLUSTER_NAME}"
    --gcp-zone="${E2E_CLUSTER_ZONE}"
    --gcp-network="${E2E_NETWORK_NAME}"
    --gke-environment=prod
  )
  if (( ! IS_PROW )); then
    CLUSTER_CREATION_ARGS+=(--gcp-project=${PROJECT_ID:?"PROJECT_ID must be set to the GCP project where the tests are run."})
  fi
  # SSH keys are not used, but kubetest checks for their existence.
  # Touch them so if they don't exist, empty files are create to satisfy the check.
  touch $HOME/.ssh/google_compute_engine.pub
  touch $HOME/.ssh/google_compute_engine
  # Clear user and cluster variables, so they'll be set to the test cluster.
  # KO_DOCKER_REPO is not touched because when running locally it must
  # be a writeable docker repo.
  export K8S_USER_OVERRIDE=
  export K8S_CLUSTER_OVERRIDE=
  # Assume test failed (see more details at the end of this script).
  echo -n "1"> ${TEST_RESULT_FILE}
  kubetest "${CLUSTER_CREATION_ARGS[@]}" \
    --up \
    --down \
    --extract "v${EVENTING_GKE_VERSION}" \
    --test-cmd "${SCRIPT_CANONICAL_PATH}" \
    --test-cmd-args --run-tests
  result="$(cat ${TEST_RESULT_FILE})"
  echo "Test result code is $result"
  exit $result
fi

# --run-tests passed as first argument, run the tests.

# Set the required variables if necessary.

if [[ -z ${K8S_USER_OVERRIDE} ]]; then
  export K8S_USER_OVERRIDE=$(gcloud config get-value core/account)
fi

USING_EXISTING_CLUSTER=1
if [[ -z ${K8S_CLUSTER_OVERRIDE} ]]; then
  USING_EXISTING_CLUSTER=0
  export K8S_CLUSTER_OVERRIDE=$(kubectl config current-context)
  acquire_cluster_admin_role ${K8S_USER_OVERRIDE} ${E2E_CLUSTER_NAME} ${E2E_CLUSTER_ZONE}
  # Make sure we're in the default namespace
  kubectl config set-context $K8S_CLUSTER_OVERRIDE --namespace=default
fi
readonly USING_EXISTING_CLUSTER

if [[ -z ${KO_DOCKER_REPO} ]]; then
  export KO_DOCKER_REPO=gcr.io/$(gcloud config get-value project)/eventing-e2e-img
fi

echo "- Cluster is ${K8S_CLUSTER_OVERRIDE}"
echo "- User is ${K8S_USER_OVERRIDE}"
echo "- Docker is ${KO_DOCKER_REPO}"

trap teardown EXIT

install_ko

if (( ! USING_EXISTING_CLUSTER )); then
  # Start Knative Serving.

  header "Starting Knative Serving"

  subheader "Installing istio"
  kubectl apply -f ${ISTIO_YAML}
  # This seems racy:
  # https://github.com/knative/eventing/issues/92
  wait_until_pods_running istio-system
  kubectl label namespace default istio-injection=enabled

  subheader "Installing Knative Serving"
  kubectl apply -f ${SERVING_RELEASE}
  exit_if_test_failed "could not install Knative Serving"

  wait_until_pods_running knative-serving-system
  wait_until_pods_running build-system
else
  header "Using existing Knative Serving"
fi


(( IS_PROW )) && gcr_auth

# Clean up anything that might still be around...
teardown_k8s_events_test_resources

if (( USING_EXISTING_CLUSTER )); then
  header "Deleting previous eventing instance if it exists"
  ko delete --ignore-not-found=true -f config/
  wait_until_namespace_does_not_exist knative-eventing
  exit_if_test_failed
  wait_until_crd_does_not_exist binds.feeds.knative.dev
  exit_if_test_failed
fi

header "Standing up Knative Eventing"
ko resolve -f config/
ko apply -f config/
exit_if_test_failed
wait_until_pods_running knative-eventing
exit_if_test_failed

header "Running 'k8s events' test"
run_k8s_events_test
exit_if_test_failed

# kubetest teardown might fail and thus incorrectly report failure of the
# script, even if the tests pass.
# We store the real test result to return it later, ignoring any teardown
# failure in kubetest.
# TODO(adrcunha): Get rid of this workaround.
echo -n "0"> ${TEST_RESULT_FILE}
echo "**************************************"
echo "***        ALL TESTS PASSED        ***"
echo "**************************************"
exit 0

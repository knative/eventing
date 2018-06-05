#!/bin/bash

# Copyright 2018 Google, Inc. All rights reserved.
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
# project $PROJECT_ID, start elafros and the eventing system, run the
# tests and delete the cluster.
# $KO_DOCKER_REPO must point to a valid writable docker repo.

source "$(dirname $(readlink -f ${BASH_SOURCE}))/library.sh"

# Test cluster parameters and location of test files
readonly E2E_CLUSTER_NAME=eventing-e2e-cluster${BUILD_NUMBER}
readonly E2E_NETWORK_NAME=eventing-e2e-net${BUILD_NUMBER}
readonly E2E_CLUSTER_ZONE=us-central1-a
readonly E2E_CLUSTER_NODES=2
readonly E2E_CLUSTER_MACHINE=n1-standard-2
readonly TEST_RESULT_FILE=/tmp/eventing-e2e-result
readonly ISTIO_VERSION=0.6.0
readonly ELAFROS_RELEASE=https://storage.googleapis.com/elafros-releases/latest/release.yaml
export ISTIO_VERSION

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
  else
    # On prow, set bogus SSH keys for kubetest, we're not using them.
    touch $HOME/.ssh/google_compute_engine.pub
    touch $HOME/.ssh/google_compute_engine
  fi
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

# Start Elafros.

header "Starting Elafros"

echo "- Cluster is ${K8S_CLUSTER_OVERRIDE}"
echo "- User is ${K8S_USER_OVERRIDE}"
echo "- Docker is ${KO_DOCKER_REPO}"

trap teardown EXIT

install_ko

subheader "Fetching istio ${ISTIO_VERSION}"
rm -fr istio-${ISTIO_VERSION}
curl -L https://git.io/getLatestIstio | sh -

pushd istio-${ISTIO_VERSION}/install/kubernetes

subheader "Installing istio"
kubectl apply -f istio.yaml
wait_until_pods_running istio-system

subheader "Enabling automatic sidecar injection in Istio"
./webhook-create-signed-cert.sh \
  --service istio-sidecar-injector \
  --namespace istio-system \
  --secret sidecar-injector-certs
kubectl apply -f istio-sidecar-injector-configmap-release.yaml
cat ./istio-sidecar-injector.yaml | \
  ./webhook-patch-ca-bundle.sh > istio-sidecar-injector-with-ca-bundle.yaml
kubectl apply -f istio-sidecar-injector-with-ca-bundle.yaml
rm ./istio-sidecar-injector-with-ca-bundle.yaml
kubectl label namespace default istio-injection=enabled
wait_until_pods_running istio-system

popd

subheader "Installing Elafros"
# Install might fail before succeding, so we retry a few times.
# For details, see https://github.com/elafros/install/issues/13
installed=0
for i in {1..10}; do
  kubectl apply -f ${ELAFROS_RELEASE} && installed=1 && break
  sleep 30
done
(( installed ))
exit_if_test_failed "could not install Elafros"

wait_until_pods_running ela-system
wait_until_pods_running build-system

(( IS_PROW )) && gcr_auth

if (( USING_EXISTING_CLUSTER )); then
  echo "Deleting any previous eventing instance"
  ko delete --ignore-not-found=true -f config/
fi

header "Standing up Elafros Binding"
ko apply -f config/
exit_if_test_failed
wait_until_pods_running knative-eventing-system
exit_if_test_failed

# TODO: Add tests.

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

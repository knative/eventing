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

readonly EVENTING_CONFIG="config/"

readonly IN_MEMORY_CHANNEL_CONFIG="config/provisioners/in-memory-channel/in-memory-channel.yaml"

# GCP PubSub config template.
readonly GCP_PUBSUB_CONFIG_TEMPLATE="contrib/gcppubsub/config/gcppubsub.yaml"
# Real GCP PubSub config, generated from the template.
readonly GCP_PUBSUB_CONFIG="$(mktemp)"

# TODO(Fredy-Z): delete this flag after https://github.com/knative/test-infra/pull/692 is merged and updated
E2E_PROJECT_ID=""

# Constants used for creating ServiceAccount for GCP PubSub provisioner setup if it's not running on Prow.
readonly PUBSUB_SERVICE_ACCOUNT="eventing_pubsub_test"
readonly PUBSUB_SERVICE_ACCOUNT_KEY="$(mktemp)"
readonly PUBSUB_SECRET_NAME="gcppubsub-channel-key"

# Setup the Knative environment for running tests.
function knative_setup() {
  # Install the latest stable Knative/serving in the current cluster.
  start_latest_knative_serving || return 1

  # Install the latest Knative/eventing in the current cluster.
  echo ">> Starting Knative Eventing"
  echo "Installing Knative Eventing"
  ko apply -f ${EVENTING_CONFIG} || return 1
  wait_until_pods_running knative-eventing || fail_test "Knative Eventing did not come up"

  echo "Installing In-Memory ClusterChannelProvisioner"
  ko apply -f ${IN_MEMORY_CHANNEL_CONFIG} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the In-Memory ClusterChannelProvisioner"

  E2E_PROJECT_ID="$(gcloud config get-value project)"
  echo "Installing GCPPubSub ClusterChannelProvisioner"
  gcppubsub_setup
  sed "s/REPLACE_WITH_GCP_PROJECT/${E2E_PROJECT_ID}/" ${GCP_PUBSUB_CONFIG_TEMPLATE} > ${GCP_PUBSUB_CONFIG}
  ko apply -f ${GCP_PUBSUB_CONFIG}
  wait_until_pods_running knative-eventing || fail_test "Failed to install the GCPPubSub ClusterChannelProvisioner"
}

# Teardown the Knative environment after tests finish.
function knative_teardown() {
  echo ">> Stopping Knative Eventing"
  echo "Uninstalling Knative Eventing"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${EVENTING_CONFIG}

  echo "Uninstalling In-Memory ClusterChannelProvisioner"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${IN_MEMORY_CHANNEL_CONFIG}

  echo "Uninstalling GCPPubSub ClusterChannelProvisioner"
  gcppubsub_teardown
  ko delete --ignore-not-found=true --now --timeout 60s -f ${GCP_PUBSUB_CONFIG}

  wait_until_object_does_not_exist namespaces knative-eventing
}

# Setup resources common to all eventing tests.
function test_setup() {
  # Publish test images.
  echo ">> Publishing test images"
  $(dirname $0)/upload-test-images.sh e2e || fail_test "Error uploading test images"
}

# Create resources required for GCP PubSub provisioner setup
function gcppubsub_setup() {
  local service_account_key="${GOOGLE_APPLICATION_CREDENTIALS}"
  # When not running on Prow we need to set up a service account for PubSub
  if (( ! IS_PROW )); then
    echo "Set up ServiceAccount for GCP PubSub provisioner"
    gcloud services enable pubsub.googleapis.com
    gcloud iam service-accounts create ${PUBSUB_SERVICE_ACCOUNT}
    gcloud projects add-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/pubsub.editor
    gcloud iam service-accounts keys create ${PUBSUB_SERVICE_ACCOUNT_KEY} \
      --iam-account=${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com
    service_account_key="${PUBSUB_SERVICE_ACCOUNT_KEY}"
  fi
  kubectl -n knative-eventing create secret generic ${PUBSUB_SECRET_NAME} --from-file=key.json=${service_account_key}
}

# Delete resources that were used for GCP PubSub provisioner setup
function gcppubsub_teardown() {
  # When not running on Prow we need to delete the service account created for PubSub
  if (( ! IS_PROW )); then
    echo "Tear down ServiceAccount for GCP PubSub provisioner"
    gcloud iam service-accounts keys delete -q ${PUBSUB_SERVICE_ACCOUNT_KEY} \
      --iam-account=${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com
    gcloud projects remove-iam-policy-binding ${E2E_PROJECT_ID} \
      --member=serviceAccount:${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com \
      --role roles/pubsub.editor
    gcloud iam service-accounts delete -q ${PUBSUB_SERVICE_ACCOUNT}@${E2E_PROJECT_ID}.iam.gserviceaccount.com
  fi
  kubectl -n knative-eventing delete secret ${PUBSUB_SECRET_NAME}
}

function dump_extra_cluster_state() {
  # Collecting logs from all knative's eventing pods.
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

go_test_e2e -timeout=20m ./test/e2e -run ^TestMain$ -runFromMain=true -clusterChannelProvisioners=in-memory-channel,in-memory,gcp-pubsub || fail_test

success

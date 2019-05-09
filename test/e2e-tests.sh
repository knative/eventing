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

# In-memory provisioner config.
readonly IN_MEMORY_CHANNEL_CONFIG="config/provisioners/in-memory-channel/in-memory-channel.yaml"

# GCP PubSub provisioner config template.
readonly GCP_PUBSUB_CONFIG_TEMPLATE="contrib/gcppubsub/config/gcppubsub.yaml"
# Real GCP PubSub provisioner config, generated from the template.
readonly GCP_PUBSUB_CONFIG="$(mktemp)"

# Constants used for creating ServiceAccount for GCP PubSub provisioner setup if it's not running on Prow.
readonly PUBSUB_SERVICE_ACCOUNT="eventing-pubsub-test"
readonly PUBSUB_SERVICE_ACCOUNT_KEY="$(mktemp)"
readonly PUBSUB_SECRET_NAME="gcppubsub-channel-key"

# NATS Streaming installation config.
readonly NATSS_INSTALLATION_CONFIG="contrib/natss/config/broker/natss.yaml"
# NATSS provisioner config.
readonly NATSS_CONFIG="contrib/natss/config/provisioner.yaml"

# Strimzi installation config template used for starting up Kafka clusters.
readonly STRIMZI_INSTALLATION_CONFIG_TEMPLATE="test/config/100-strimzi-cluster-operator-0.11.3.yaml"
# Strimzi installation config.
readonly STRIMZI_INSTALLATION_CONFIG="$(mktemp)"
# Kafka cluster CR config file.
readonly KAFKA_INSTALLATION_CONFIG="test/config/100-kafka-persistent-single-2.1.0.yaml"
# Kafka provisioner config template.
readonly KAFKA_CONFIG_TEMPLATE="contrib/kafka/config/kafka.yaml"
# Real Kafka provisioner config, generated from the template.
readonly KAFKA_CONFIG="$(mktemp)"
# Kafka cluster URL for our installation
readonly KAFKA_CLUSTER_URL="my-cluster-kafka-bootstrap.kafka:9092"

# Setup the Knative environment for running tests.
function knative_setup() {
  # Install the latest Knative/eventing in the current cluster.
  echo ">> Starting Knative Eventing"
  echo "Installing Knative Eventing"
  ko apply -f ${EVENTING_CONFIG} || return 1
  wait_until_pods_running knative-eventing || fail_test "Knative Eventing did not come up"
}

# Teardown the Knative environment after tests finish.
function knative_teardown() {
  echo ">> Stopping Knative Eventing"
  echo "Uninstalling Knative Eventing"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${EVENTING_CONFIG}
}

# Setup resources common to all eventing tests.
function test_setup() {
  # Install provisioners used by the tests.
  echo "Installing In-Memory ClusterChannelProvisioner"
  ko apply -f ${IN_MEMORY_CHANNEL_CONFIG} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the In-Memory ClusterChannelProvisioner"

  echo "Installing GCPPubSub ClusterChannelProvisioner"
  gcppubsub_setup || return 1
  sed "s/REPLACE_WITH_GCP_PROJECT/${E2E_PROJECT_ID}/" ${GCP_PUBSUB_CONFIG_TEMPLATE} > ${GCP_PUBSUB_CONFIG}
  ko apply -f ${GCP_PUBSUB_CONFIG} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the GCPPubSub ClusterChannelProvisioner"

  echo "Installing NATSS ClusterChannelProvisioner"
  natss_setup || return 1
  ko apply -f ${NATSS_CONFIG} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the NATSS ClusterChannelProvisioner"

  echo "Installing Kafka ClusterChannelProvisioner"
  kafka_setup || return 1
  sed "s/REPLACE_WITH_CLUSTER_URL/${KAFKA_CLUSTER_URL}/" ${KAFKA_CONFIG_TEMPLATE} > ${KAFKA_CONFIG}
  ko apply -f ${KAFKA_CONFIG} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the Kafka ClusterChannelProvisioner"

  # Publish test images.
  echo ">> Publishing test images"
  $(dirname $0)/upload-test-images.sh e2e || fail_test "Error uploading test images"
}

# Tear down resources used in the eventing tests.
function test_teardown() {
  # Uninstall provisioners used by the tests.
  echo "Uninstalling In-Memory ClusterChannelProvisioner"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${IN_MEMORY_CHANNEL_CONFIG}

  echo "Uninstalling GCPPubSub ClusterChannelProvisioner"
  gcppubsub_teardown
  ko delete --ignore-not-found=true --now --timeout 60s -f ${GCP_PUBSUB_CONFIG}

  echo "Uninstalling NATSS ClusterChannelProvisioner"
  natss_teardown
  ko delete --ignore-not-found=true --now --timeout 60s -f ${NATSS_CONFIG}

  echo "Uninstalling Kafka ClusterChannelProvisioner"
  kafka_teardown
  ko delete --ignore-not-found=true --now --timeout 60s -f ${KAFKA_CONFIG}

  wait_until_object_does_not_exist namespaces knative-eventing
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

# Create resources required for NATSS provisioner setup
function natss_setup() {
  echo "Installing NATS Streaming"
  kubectl create namespace natss || return 1
  kubectl apply -n natss -f ${NATSS_INSTALLATION_CONFIG} || return 1
  wait_until_pods_running natss || fail_test "Failed to start up a NATSS cluster"
}

# Delete resources used for NATSS provisioner setup
function natss_teardown() {
  echo "Uninstalling NATS Streaming"
  kubectl delete -f ${NATSS_INSTALLATION_CONFIG}
  kubectl delete namespace natss
}

function kafka_setup() {
  echo "Installing Kafka cluster"
  kubectl create namespace kafka || return 1
  sed 's/namespace: .*/namespace: kafka/' ${STRIMZI_INSTALLATION_CONFIG_TEMPLATE} > ${STRIMZI_INSTALLATION_CONFIG}
  kubectl apply -f ${STRIMZI_INSTALLATION_CONFIG} -n kafka
  kubectl apply -f ${KAFKA_INSTALLATION_CONFIG} -n kafka
  wait_until_pods_running kafka || fail_test "Failed to start up a Kafka cluster"
}

function kafka_teardown() {
  echo "Uninstalling Kafka cluster"
  kubectl delete -f ${STRIMZI_INSTALLATION_CONFIG} -n kafka
  kubectl delete -f ${KAFKA_INSTALLATION_CONFIG} -n kafka
  kubectl delete namespace kafka
}

function dump_extra_cluster_state() {
  # Collecting logs from all knative's eventing pods.
  echo "============================================================"
  local namespace="knative-eventing"
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
}

# Script entry point.

initialize $@ --skip-istio

go_test_e2e -timeout=20m ./test/e2e -run ^TestMain$ -runFromMain=true -clusterChannelProvisioners=in-memory-channel,in-memory,natss,kafka || fail_test

success

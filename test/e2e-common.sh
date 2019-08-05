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

# This script includes common functions for testing setup and teardown.

source $(dirname $0)/../vendor/knative.dev/test-infra/scripts/e2e-tests.sh

# If gcloud is not available make it a no-op, not an error.
which gcloud &> /dev/null || gcloud() { echo "[ignore-gcloud $*]" 1>&2; }

# Let sed work on mac.
if [ "$(uname)" == "Darwin" ]; then
  sed=gsed
fi

# Eventing main config.
readonly EVENTING_CONFIG="config/"

# In-memory channel CRD config.
readonly IN_MEMORY_CHANNEL_CRD_CONFIG_DIR="config/channels/in-memory-channel"

# NATS Streaming installation config.
readonly NATSS_INSTALLATION_CONFIG="contrib/natss/config/broker/natss.yaml"
# NATSS channel CRD config directory.
readonly NATSS_CRD_CONFIG_DIR="contrib/natss/config"

# Strimzi installation config template used for starting up Kafka clusters.
readonly STRIMZI_VERSION="0.11.4"
readonly STRIMZI_INSTALLATION_CONFIG_TEMPLATE="test/config/100-strimzi-cluster-operator-${STRIMZI_VERSION}.yaml"
# Strimzi installation config.
readonly STRIMZI_INSTALLATION_CONFIG="$(mktemp)"
# Kafka cluster CR config file.
readonly KAFKA_INSTALLATION_CONFIG="test/config/100-kafka-persistent-single-2.1.0.yaml"
# Kafka cluster URL for our installation
readonly KAFKA_CLUSTER_URL="my-cluster-kafka-bootstrap.kafka:9092"
# Kafka channel CRD config template directory.
readonly KAFKA_CRD_CONFIG_TEMPLATE_DIR="contrib/kafka/config"
# Kafka channel CRD config template file. It needs to be modified to be the real config file.
readonly KAFKA_CRD_CONFIG_TEMPLATE="400-kafka-config.yaml"
# Real Kafka channel CRD config , generated from the template directory and modified template file.
readonly KAFKA_CRD_CONFIG_DIR="$(mktemp -d)"

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
  wait_until_object_does_not_exist namespaces knative-eventing
}

# Setup resources common to all eventing tests.
function test_setup() {
  natss_setup || return 1
  kafka_setup || return 1

  install_test_resources || return 1

  # Publish test images.
  echo ">> Publishing test images"
  $(dirname $0)/upload-test-images.sh e2e || fail_test "Error uploading test images"
}

# Tear down resources used in the eventing tests.
function test_teardown() {
  natss_teardown
  kafka_teardown

  uninstall_test_resources
}

function install_test_resources() {
  install_channel_crds || return 1
}

function uninstall_test_resources() {
  uninstall_channel_crds
}

function install_channel_crds() {
  echo "Installing In-Memory Channel CRD"
  ko apply -f ${IN_MEMORY_CHANNEL_CRD_CONFIG_DIR} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the In-Memory Channel CRD"

  echo "Installing NATSS Channel CRD"
  ko apply -f ${NATSS_CRD_CONFIG_DIR} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the NATSS Channel CRD"

  echo "Installing Kafka Channel CRD"
  cp ${KAFKA_CRD_CONFIG_TEMPLATE_DIR}/*yaml ${KAFKA_CRD_CONFIG_DIR}
  sed -i "s/REPLACE_WITH_CLUSTER_URL/${KAFKA_CLUSTER_URL}/" ${KAFKA_CRD_CONFIG_DIR}/${KAFKA_CRD_CONFIG_TEMPLATE}
  ko apply -f ${KAFKA_CRD_CONFIG_DIR} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the Kafka Channel CRD"
}

function uninstall_channel_crds() {
  echo "Uninstalling In-Memory Channel CRD"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${IN_MEMORY_CHANNEL_CRD_CONFIG_DIR}

  echo "Uninstalling NATSS Channel CRD"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${NATSS_CRD_CONFIG_DIR}

  echo "Uninstalling Kafka Channel CRD"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${KAFKA_CRD_CONFIG_DIR}
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

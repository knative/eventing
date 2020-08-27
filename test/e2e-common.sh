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

export GO111MODULE=on

source $(dirname $0)/../vendor/knative.dev/test-infra/scripts/e2e-tests.sh

# If gcloud is not available make it a no-op, not an error.
which gcloud &>/dev/null || gcloud() { echo "[ignore-gcloud $*]" 1>&2; }

# Use GNU tools on macOS. Requires the 'grep' and 'gnu-sed' Homebrew formulae.
if [ "$(uname)" == "Darwin" ]; then
  sed=gsed
  grep=ggrep
fi

# Eventing main config.
readonly EVENTING_CONFIG="config/"

# In-memory channel CRD config.
readonly IN_MEMORY_CHANNEL_CRD_CONFIG_DIR="config/channels/in-memory-channel"

# MT Channel Based Broker config.
readonly MT_CHANNEL_BASED_BROKER_CONFIG_DIR="config/brokers/mt-channel-broker"
# MT Channel Based Broker config.
readonly MT_CHANNEL_BASED_BROKER_DEFAULT_CONFIG="config/core/configmaps/default-broker.yaml"

# Sugar Controller config. For label/annotation magic.
readonly SUGAR_CONTROLLER_CONFIG_DIR="config/sugar"
readonly SUGAR_CONTROLLER_CONFIG="config/sugar/500-controller.yaml"

# Config tracing config.
readonly CONFIG_TRACING_CONFIG="test/config/config-tracing.yaml"

# PreInstall script for v0.18
readonly PRE_INSTALL_V018="config/pre-install/v0.18.0"

# The number of controlplane replicas to run.
readonly REPLICAS=3

# Should deploy a Knative Monitoring as well
readonly DEPLOY_KNATIVE_MONITORING="${DEPLOY_KNATIVE_MONITORING:-1}"

TMP_DIR=$(mktemp -d -t ci-$(date +%Y-%m-%d-%H-%M-%S)-XXXXXXXXXX)
readonly TMP_DIR
readonly KNATIVE_DEFAULT_NAMESPACE="knative-eventing"

# This the namespace used to install and test Knative Eventing.
export TEST_EVENTING_NAMESPACE
TEST_EVENTING_NAMESPACE="${TEST_EVENTING_NAMESPACE:-"knative-eventing-"$(cat /dev/urandom \
  | tr -dc 'a-z0-9' | fold -w 10 | head -n 1)}"

latest_version() {
  local semver=$(git describe --match "v[0-9]*" --abbrev=0)
  local major_minor=$(echo "$semver" | cut -d. -f1-2)

  # Get the latest patch release for the major minor
  git tag -l "${major_minor}*" | sort -r --version-sort | head -n1
}

# Latest release. If user does not supply this as a flag, the latest
# tagged release on the current branch will be used.
readonly LATEST_RELEASE_VERSION=$(latest_version)

UNINSTALL_LIST=()

# Setup the Knative environment for running tests.
function knative_setup() {
  install_knative_eventing

  install_mt_broker || fail_test "Could not install MT Channel Based Broker"

  install_sugar || fail_test "Could not install Sugar Controller"

  unleash_duck || fail_test "Could not unleash the chaos duck"
}

function scale_controlplane() {
  for deployment in "$@"; do
    # Make sure all pods run in leader-elected mode.
    kubectl -n "${TEST_EVENTING_NAMESPACE}" scale deployment "$deployment" --replicas=0 || failed=1
    # Give it time to kill the pods.
    sleep 5
    # Scale up components for HA tests
    kubectl -n "${TEST_EVENTING_NAMESPACE}" scale deployment "$deployment" --replicas="${REPLICAS}" || failed=1
  done
}

# This installs everything from the config dir but then removes the Channel Based Broker.
# TODO: This should only install the core.
# Args:
#  - $1 - if passed, it will be used as eventing config directory
function install_knative_eventing() {
  echo ">> Creating ${TEST_EVENTING_NAMESPACE} namespace if it does not exist"
  kubectl get ns ${TEST_EVENTING_NAMESPACE} || kubectl create namespace ${TEST_EVENTING_NAMESPACE}
  local kne_config
  kne_config="${1:-${EVENTING_CONFIG}}"
  # Install Knative Eventing in the current cluster.
  echo "Installing Knative Eventing from: ${kne_config}"
  if [ -d "${kne_config}" ]; then
    local TMP_CONFIG_DIR=${TMP_DIR}/config
    mkdir -p ${TMP_CONFIG_DIR}
    cp -r ${kne_config}/* ${TMP_CONFIG_DIR}
    find ${TMP_CONFIG_DIR} -type f -name "*.yaml" -exec sed -i "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${TEST_EVENTING_NAMESPACE}/g" {} +
    ko apply --strict -f "${TMP_CONFIG_DIR}" || return $?
  else
    local EVENTING_RELEASE_YAML=${TMP_DIR}/"eventing-${LATEST_RELEASE_VERSION}.yaml"
    # Download the latest release of Knative Eventing.
    wget "${kne_config}" -O "${EVENTING_RELEASE_YAML}" \
      || fail_test "Unable to download latest knative/eventing file."

    # Replace the default system namespace with the test's system namespace.
    sed -i "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${TEST_EVENTING_NAMESPACE}/g" ${EVENTING_RELEASE_YAML}

    echo "Knative EVENTING YAML: ${EVENTING_RELEASE_YAML}"

    kubectl apply -f "${EVENTING_RELEASE_YAML}" || return $?
    UNINSTALL_LIST+=( "${EVENTING_RELEASE_YAML}" )
  fi

  # Setup config tracing for tracing tests
  local TMP_CONFIG_TRACING_CONFIG=${TMP_DIR}/${CONFIG_TRACING_CONFIG##*/}
  sed "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${TEST_EVENTING_NAMESPACE}/g" ${CONFIG_TRACING_CONFIG} > ${TMP_CONFIG_TRACING_CONFIG}
  kubectl replace -f ${TMP_CONFIG_TRACING_CONFIG}

  scale_controlplane eventing-webhook eventing-controller

  wait_until_pods_running ${TEST_EVENTING_NAMESPACE} || fail_test "Knative Eventing did not come up"

  echo "check the config map"
  kubectl get configmaps -n ${TEST_EVENTING_NAMESPACE}
  if ! (( DEPLOY_KNATIVE_MONITORING )); then return 0; fi

  # Ensure knative monitoring is installed only once
  kubectl get ns knative-monitoring|| kubectl create namespace knative-monitoring
  knative_monitoring_pods=$(kubectl get pods -n knative-monitoring \
    --field-selector status.phase=Running 2> /dev/null | tail -n +2 | wc -l)
  if ! [[ ${knative_monitoring_pods} -gt 0 ]]; then
    echo ">> Installing Knative Monitoring"
    start_knative_monitoring "${KNATIVE_MONITORING_RELEASE}" || fail_test "Knative Monitoring did not come up"
    UNINSTALL_LIST+=( "${KNATIVE_MONITORING_RELEASE}" )
  else
    echo ">> Knative Monitoring seems to be running, pods running: ${knative_monitoring_pods}."
  fi
}

function install_head {
  # Install Knative Eventing from HEAD in the current cluster.
  echo ">> Installing Knative Eventing from HEAD"
  install_knative_eventing || \
    fail_test "Knative HEAD installation failed"
}

function install_latest_release() {
  header ">> Installing Knative Eventing latest public release"
  local url="https://github.com/knative/eventing/releases/download/${LATEST_RELEASE_VERSION}"
  local yaml="eventing.yaml"

  install_knative_eventing \
    "${url}/${yaml}" || \
    fail_test "Knative latest release installation failed"
}

function run_preinstall_V018() {
  local TMP_PRE_INSTALL_V018=${TMP_DIR}/pre_install
  mkdir -p ${TMP_PRE_INSTALL_V018}
  cp -r ${PRE_INSTALL_V018}/* ${TMP_PRE_INSTALL_V018}
  find ${TMP_PRE_INSTALL_V018} -type f -name "*.yaml" -exec sed -i "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${TEST_EVENTING_NAMESPACE}/g" {} +
  ko apply --strict -f "${TMP_PRE_INSTALL_V018}" || return 1
  wait_until_batch_job_complete ${TEST_EVENTING_NAMESPACE} || return 1
}

function install_mt_broker() {
  local TMP_MT_CHANNEL_BASED_BROKER_DEFAULT_CONFIG=${TMP_DIR}/${MT_CHANNEL_BASED_BROKER_DEFAULT_CONFIG##*/}
  sed "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${TEST_EVENTING_NAMESPACE}/g" ${MT_CHANNEL_BASED_BROKER_DEFAULT_CONFIG} > ${TMP_MT_CHANNEL_BASED_BROKER_DEFAULT_CONFIG}
  ko apply --strict -f ${TMP_MT_CHANNEL_BASED_BROKER_DEFAULT_CONFIG} || return 1

  local TMP_MT_CHANNEL_BASED_BROKER_CONFIG_DIR=${TMP_DIR}/channel_based_config
  mkdir -p ${TMP_MT_CHANNEL_BASED_BROKER_CONFIG_DIR}
  cp -r ${MT_CHANNEL_BASED_BROKER_CONFIG_DIR}/* ${TMP_MT_CHANNEL_BASED_BROKER_CONFIG_DIR}
  find ${TMP_MT_CHANNEL_BASED_BROKER_CONFIG_DIR} -type f -name "*.yaml" -exec sed -i "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${TEST_EVENTING_NAMESPACE}/g" {} +
  ko apply --strict -f ${TMP_MT_CHANNEL_BASED_BROKER_CONFIG_DIR} || return 1

  scale_controlplane mt-broker-controller

  wait_until_pods_running ${TEST_EVENTING_NAMESPACE} || fail_test "Knative Eventing with MT Broker did not come up"
}

function install_sugar() {
  local TMP_SUGAR_CONTROLLER_CONFIG_DIR=${TMP_DIR}/${SUGAR_CONTROLLER_CONFIG_DIR}
  mkdir -p ${TMP_SUGAR_CONTROLLER_CONFIG_DIR}
  cp -r ${SUGAR_CONTROLLER_CONFIG_DIR}/* ${TMP_SUGAR_CONTROLLER_CONFIG_DIR}
  find ${TMP_SUGAR_CONTROLLER_CONFIG_DIR} -type f -name "*.yaml" -exec sed -i "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${TEST_EVENTING_NAMESPACE}/g" {} +
  ko apply --strict -f ${TMP_SUGAR_CONTROLLER_CONFIG_DIR} || return 1
  kubectl -n ${TEST_EVENTING_NAMESPACE} set env deployment/sugar-controller BROKER_INJECTION_DEFAULT=true || return 1

  scale_controlplane sugar-controller

  wait_until_pods_running ${TEST_EVENTING_NAMESPACE} || fail_test "Knative Eventing Sugar Controller did not come up"
}

function unleash_duck() {
  echo "enable debug logging"
  cat test/config/config-logging.yaml | \
    sed "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${TEST_EVENTING_NAMESPACE}/g" | \
    ko apply --strict -f - || return $?

  echo "unleash the duck"
  cat test/config/chaosduck.yaml | \
    sed "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${TEST_EVENTING_NAMESPACE}/g" | \
    ko apply --strict -f - || return $?
}

# Teardown the Knative environment after tests finish.
function knative_teardown() {
  echo ">> Stopping Knative Eventing"
  echo "Uninstalling Knative Eventing"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${EVENTING_CONFIG}
  wait_until_object_does_not_exist namespaces ${TEST_EVENTING_NAMESPACE}

  echo ">> Uninstalling dependencies"
  for i in ${!UNINSTALL_LIST[@]}; do
    # We uninstall elements in the reverse of the order they were installed.
    local YAML="${UNINSTALL_LIST[$(( ${#array[@]} - $i ))]}"
    echo ">> Bringing down YAML: ${YAML}"
    kubectl delete --ignore-not-found=true -f "${YAML}" || return 1
  done
}

# Add function call to trap
# Parameters: $1 - Function to call
#             $2...$n - Signals for trap
function add_trap() {
  local cmd=$1
  shift
  for trap_signal in $@; do
    local current_trap="$(trap -p $trap_signal | cut -d\' -f2)"
    local new_cmd="($cmd)"
    [[ -n "${current_trap}" ]] && new_cmd="${current_trap};${new_cmd}"
    trap -- "${new_cmd}" $trap_signal
  done
}

# Setup resources common to all eventing tests.
function test_setup() {
  echo ">> Setting up logging..."

  # Install kail if needed.
  if ! which kail >/dev/null; then
    bash <(curl -sfL https://raw.githubusercontent.com/boz/kail/master/godownloader.sh) -b "$GOPATH/bin"
  fi

  # Capture all logs.
  kail >${ARTIFACTS}/k8s.log.txt &
  local kail_pid=$!
  # Clean up kail so it doesn't interfere with job shutting down
  add_trap "kill $kail_pid || true" EXIT

  install_test_resources || return 1

  echo ">> Publish test images"
  "$(dirname "$0")/upload-test-images.sh" e2e || fail_test "Error uploading test images"
}

# Tear down resources used in the eventing tests.
function test_teardown() {
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
  local TMP_IN_MEMORY_CHANNEL_CONFIG_DIR=${TMP_DIR}/in_memory_channel_config
  mkdir -p ${TMP_IN_MEMORY_CHANNEL_CONFIG_DIR}
  cp -r ${IN_MEMORY_CHANNEL_CRD_CONFIG_DIR}/* ${TMP_IN_MEMORY_CHANNEL_CONFIG_DIR}
  find ${TMP_IN_MEMORY_CHANNEL_CONFIG_DIR} -type f -name "*.yaml" -exec sed -i "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${TEST_EVENTING_NAMESPACE}/g" {} +
  ko apply --strict -f ${TMP_IN_MEMORY_CHANNEL_CONFIG_DIR} || return 1

  # TODO(https://github.com/knative/eventing/issues/3590): Enable once IMC chaos issues are fixed.
  # scale_controlplane imc-controller imc-dispatcher

  wait_until_pods_running ${TEST_EVENTING_NAMESPACE} || fail_test "Failed to install the In-Memory Channel CRD"
}

function uninstall_channel_crds() {
  echo "Uninstalling In-Memory Channel CRD"
  local TMP_IN_MEMORY_CHANNEL_CONFIG_DIR=${TMP_DIR}/in_memory_channel_config
  ko delete --ignore-not-found=true --now --timeout 60s -f ${TMP_IN_MEMORY_CHANNEL_CONFIG_DIR}
}

function dump_extra_cluster_state() {
  # Collecting logs from all knative's eventing pods.
  echo "============================================================"
  local namespace=${TEST_EVENTING_NAMESPACE}
  for pod in $(kubectl get pod -n $namespace | grep Running | awk '{print $1}'); do
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

function wait_for_file() {
  local file timeout waits
  file="$1"
  waits=300
  timeout=$waits

  echo "Waiting for existance of file: ${file}"

  while [ ! -f "${file}" ]; do
    # When the timeout is equal to zero, show an error and leave the loop.
    if [ "${timeout}" == 0 ]; then
      echo "ERROR: Timeout (${waits}s) while waiting for the file ${file}."
      return 1
    fi

    sleep 1

    # Decrease the timeout of one
    ((timeout--))
  done
  return 0
}

#!/usr/bin/env bash

# Copyright 2020 The Knative Authors
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

# shellcheck disable=SC1090

export GO111MODULE=on

source "$(dirname "${BASH_SOURCE[0]}")/../vendor/knative.dev/hack/e2e-tests.sh"

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

# Installs Zipkin for tracing tests.
readonly KNATIVE_EVENTING_MONITORING_YAML="test/config/monitoring.yaml"

# The number of controlplane replicas to run.
readonly REPLICAS=3

# Should deploy a Knative Monitoring as well
readonly DEPLOY_KNATIVE_MONITORING="${DEPLOY_KNATIVE_MONITORING:-1}"

TMP_DIR=$(mktemp -d -t "ci-$(date +%Y-%m-%d-%H-%M-%S)-XXXXXXXXXX")
readonly TMP_DIR
readonly KNATIVE_DEFAULT_NAMESPACE="knative-eventing"

# This the namespace used to install and test Knative Eventing.
export SYSTEM_NAMESPACE
SYSTEM_NAMESPACE="${SYSTEM_NAMESPACE:-"knative-eventing-"$(head -c 128 < \
  /dev/urandom | LC_CTYPE=C tr -dc 'a-z0-9' | fold -w 10 | head -n 1)}"

# Latest release. If user does not supply this as a flag, the latest
# tagged release on the current branch will be used.
readonly LATEST_RELEASE_VERSION=$(latest_version)

UNINSTALL_LIST=()

# Setup the Knative environment for running tests.
function knative_setup() {
  install_knative_eventing "HEAD"

  install_mt_broker || fail_test "Could not install MT Channel Based Broker"

  install_sugar || fail_test "Could not install Sugar Controller"

  unleash_duck || fail_test "Could not unleash the chaos duck"
}

function scale_controlplane() {
  for deployment in "$@"; do
    # Make sure all pods run in leader-elected mode.
    kubectl -n "${SYSTEM_NAMESPACE}" scale deployment "$deployment" --replicas=0 || failed=1
    # Give it time to kill the pods.
    sleep 5
    # Scale up components for HA tests
    kubectl -n "${SYSTEM_NAMESPACE}" scale deployment "$deployment" --replicas="${REPLICAS}" || failed=1
  done
}

# Install Knative Monitoring in the current cluster.
# Parameters: $1 - Knative Monitoring manifest.
# This is a lightly modified verion of start_knative_monitoring() from test-infra's library.sh.
function start_knative_eventing_monitoring() {
  header "Starting Knative Eventing Monitoring"
  subheader "Installing Knative Eventing Monitoring"
  echo "Installing Monitoring from $1"
  kubectl apply -f "$1" || return 1
  wait_until_pods_running knative-eventing || return 1
}

# Create all manifests required to install Knative Eventing.
# This will build everything from the current source.
# All generated YAMLs will be available and pointed by the corresponding
# environment variables as set in /hack/generate-yamls.sh.
function build_knative_from_source() {
  local FULL_OUTPUT YAML_LIST LOG_OUTPUT ENV_OUTPUT
  YAML_LIST="$(mktemp)"

  # Generate manifests, capture environment variables pointing to the YAML files.
  FULL_OUTPUT="$( \
      source "$(dirname "${BASH_SOURCE[0]}")/../hack/generate-yamls.sh" "${REPO_ROOT_DIR}" "${YAML_LIST}" ; \
      set | grep _YAML=/)"
  LOG_OUTPUT="$(echo "${FULL_OUTPUT}" | grep -v _YAML=/)"
  ENV_OUTPUT="$(echo "${FULL_OUTPUT}" | grep '^[_0-9A-Z]\+_YAML=/')"
  [[ -z "${LOG_OUTPUT}" || -z "${ENV_OUTPUT}" ]] && fail_test "Error generating manifests"
  # Only import the environment variables pointing to the YAML files.
  echo "${LOG_OUTPUT}"
  echo -e "Generated manifests:\n${ENV_OUTPUT}"
  eval "${ENV_OUTPUT}"
}

# TODO: This should only install the core.
# Install Knative Eventing in current cluster
# Args:
# Parameters: $1 - Knative Eventing version "HEAD" or "latest-release".
function install_knative_eventing() {
  echo ">> Creating ${SYSTEM_NAMESPACE} namespace if it does not exist"
  kubectl get ns ${SYSTEM_NAMESPACE} || kubectl create namespace ${SYSTEM_NAMESPACE}
  # Install Knative Eventing in the current cluster.
  echo "Installing Knative Eventing from: ${1}"
  if [[ "$1" == "HEAD" ]]; then
    build_knative_from_source
    local EVENTING_CORE_NAME=${TMP_DIR}/${EVENTING_CORE_YAML##*/}
    sed "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${SYSTEM_NAMESPACE}/g" ${EVENTING_CORE_YAML} > ${EVENTING_CORE_NAME}
    kubectl apply \
      -f "${EVENTING_CORE_NAME}" || return 1
    UNINSTALL_LIST+=( "${EVENTING_CORE_NAME}" )

    kubectl patch horizontalpodautoscalers.autoscaling -n ${SYSTEM_NAMESPACE} eventing-webhook -p '{"spec": {"minReplicas": '${REPLICAS}'}}' || return 1

  else
    local EVENTING_RELEASE_YAML=${TMP_DIR}/"eventing-${LATEST_RELEASE_VERSION}.yaml"
    # Download the latest release of Knative Eventing.
    local url="https://github.com/knative/eventing/releases/download/${LATEST_RELEASE_VERSION}"
    wget "${url}/eventing.yaml" -O "${EVENTING_RELEASE_YAML}" \
      || fail_test "Unable to download latest knative/eventing file."

    # Replace the default system namespace with the test's system namespace.
    sed -i "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${SYSTEM_NAMESPACE}/g" ${EVENTING_RELEASE_YAML}

    echo "Knative EVENTING YAML: ${EVENTING_RELEASE_YAML}"

    kubectl apply -f "${EVENTING_RELEASE_YAML}" || return $?
    UNINSTALL_LIST+=( "${EVENTING_RELEASE_YAML}" )
  fi

  # Setup config tracing for tracing tests
  local TMP_CONFIG_TRACING_CONFIG=${TMP_DIR}/${CONFIG_TRACING_CONFIG##*/}
  sed "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${SYSTEM_NAMESPACE}/g" ${CONFIG_TRACING_CONFIG} > ${TMP_CONFIG_TRACING_CONFIG}
  kubectl replace -f ${TMP_CONFIG_TRACING_CONFIG}

  scale_controlplane eventing-webhook eventing-controller

  wait_until_pods_running ${SYSTEM_NAMESPACE} || fail_test "Knative Eventing did not come up"

  echo "check the config map"
  kubectl get configmaps -n ${SYSTEM_NAMESPACE}
  if ! (( DEPLOY_KNATIVE_MONITORING )); then return 0; fi

  # Ensure knative monitoring is installed only once
  kubectl get ns knative-monitoring|| kubectl create namespace knative-monitoring
  knative_monitoring_pods=$(kubectl get pods -n knative-monitoring \
    --field-selector status.phase=Running 2> /dev/null | tail -n +2 | wc -l)
  if ! [[ ${knative_monitoring_pods} -gt 0 ]]; then
    echo ">> Installing Knative Monitoring"
    start_knative_eventing_monitoring "${KNATIVE_EVENTING_MONITORING_YAML}" || fail_test "Knative Monitoring did not come up"
    UNINSTALL_LIST+=( "${KNATIVE_EVENTING_MONITORING_YAML}" )
  else
    echo ">> Knative Monitoring seems to be running, pods running: ${knative_monitoring_pods}."
  fi
}

function install_head {
  # Install Knative Eventing from HEAD in the current cluster.
  echo ">> Installing Knative Eventing from HEAD"
  install_knative_eventing "HEAD" || \
    fail_test "Knative HEAD installation failed"
}

function install_latest_release() {
  header ">> Installing Knative Eventing latest public release"

  install_knative_eventing \
    "latest-release" || \
    fail_test "Knative latest release installation failed"
}

function install_mt_broker() {
  if [[ -z "${EVENTING_MT_CHANNEL_BROKER_YAML:-}" ]]; then
    build_knative_from_source
  else
    echo "use exist EVENTING_MT_CHANNEL_BROKER_YAML"
  fi
  local EVENTING_MT_CHANNEL_BROKER_NAME=${TMP_DIR}/${EVENTING_MT_CHANNEL_BROKER_YAML##*/}
  sed "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${SYSTEM_NAMESPACE}/g" ${EVENTING_MT_CHANNEL_BROKER_YAML} > ${EVENTING_MT_CHANNEL_BROKER_NAME}
  kubectl apply \
    -f "${EVENTING_MT_CHANNEL_BROKER_NAME}" || return 1
  UNINSTALL_LIST+=( "${EVENTING_MT_CHANNEL_BROKER_NAME}" )
  scale_controlplane mt-broker-controller

  wait_until_pods_running ${SYSTEM_NAMESPACE} || fail_test "Knative Eventing with MT Broker did not come up"
}

function install_sugar() {
  if [[ -z "${EVENTING_SUGAR_CONTROLLER_YAML:-}" ]]; then
    build_knative_from_source
  else
    echo "use exist EVENTING_SUGAR_CONTROLLER_YAML"
  fi
  local EVENTING_SUGAR_CONTROLLER_NAME=${TMP_DIR}/${EVENTING_SUGAR_CONTROLLER_YAML##*/}
  sed "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${SYSTEM_NAMESPACE}/g" ${EVENTING_SUGAR_CONTROLLER_YAML} > ${EVENTING_SUGAR_CONTROLLER_NAME}
  kubectl apply \
    -f "${EVENTING_SUGAR_CONTROLLER_NAME}" || return 1
  UNINSTALL_LIST+=( "${EVENTING_SUGAR_CONTROLLER_NAME}" )

  scale_controlplane sugar-controller

  wait_until_pods_running ${SYSTEM_NAMESPACE} || fail_test "Knative Eventing Sugar Controller did not come up"
}

function unleash_duck() {
  echo "enable debug logging"
  cat test/config/config-logging.yaml | \
    sed "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${SYSTEM_NAMESPACE}/g" | \
    ko apply --strict -f - || return $?

  echo "unleash the duck"
  cat test/config/chaosduck.yaml | \
    sed "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${SYSTEM_NAMESPACE}/g" | \
    ko apply --strict -f - || return $?
}

# Teardown the Knative environment after tests finish.
function knative_teardown() {
  echo ">> Stopping Knative Eventing"
  echo "Uninstalling Knative Eventing"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${EVENTING_CONFIG}
  wait_until_object_does_not_exist namespaces ${SYSTEM_NAMESPACE}

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
  "$(dirname "${BASH_SOURCE[0]}")/upload-test-images.sh" e2e || fail_test "Error uploading test images"
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
  if [[ -z "${EVENTING_IN_MEMORY_CHANNEL_YAML:-}" ]]; then
    build_knative_from_source
  else
    echo "use exist EVENTING_SUGAR_CONTROLLER_YAML"
  fi
  local EVENTING_IN_MEMORY_CHANNEL_NAME=${TMP_DIR}/${EVENTING_IN_MEMORY_CHANNEL_YAML##*/}
  sed "s/namespace: ${KNATIVE_DEFAULT_NAMESPACE}/namespace: ${SYSTEM_NAMESPACE}/g" ${EVENTING_IN_MEMORY_CHANNEL_YAML} > ${EVENTING_IN_MEMORY_CHANNEL_NAME}
  kubectl apply \
    -f "${EVENTING_IN_MEMORY_CHANNEL_NAME}" || return 1
  UNINSTALL_LIST+=( "${EVENTING_IN_MEMORY_CHANNEL_NAME}" )

  # TODO(https://github.com/knative/eventing/issues/3590): Enable once IMC chaos issues are fixed.
  # scale_controlplane imc-controller imc-dispatcher

  wait_until_pods_running ${SYSTEM_NAMESPACE} || fail_test "Failed to install the In-Memory Channel CRD"
}

function uninstall_channel_crds() {
  echo "Uninstalling In-Memory Channel CRD"
  local EVENTING_IN_MEMORY_CHANNEL_NAME=${TMP_DIR}/${EVENTING_IN_MEMORY_CHANNEL_YAML##*/}
  kubectl delete --ignore-not-found=true -f "${EVENTING_IN_MEMORY_CHANNEL_NAME}" || return 1
}

function dump_extra_cluster_state() {
  # Collecting logs from all knative's eventing pods.
  echo "============================================================"
  local namespace=${SYSTEM_NAMESPACE}
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

  echo "Waiting for existence of file: ${file}"

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

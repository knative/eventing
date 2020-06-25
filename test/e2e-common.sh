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

# Channel Based Broker Controller.
readonly CHANNEL_BASED_BROKER_CONTROLLER="config/brokers/channel-broker"
# Channel Based Broker config.
readonly CHANNEL_BASED_BROKER_DEFAULT_CONFIG="test/config/st-channel-broker.yaml"

# PreInstall script for v0.16
readonly PRE_INSTALL_V016="config/pre-install/v0.16.0"

# Should deploy a Knative Monitoring as well
readonly DEPLOY_KNATIVE_MONITORING="${DEPLOY_KNATIVE_MONITORING:-1}"

# Latest release. If user does not supply this as a flag, the latest
# tagged release on the current branch will be used.
readonly LATEST_RELEASE_VERSION=$(git describe --match "v[0-9]*" --abbrev=0)

UNINSTALL_LIST=()

# Setup the Knative environment for running tests.
function knative_setup() {
  install_knative_eventing
}

# This installs everything from the config dir but then removes the Channel Based Broker.
# TODO: This should only install the core.
# Args:
#  - $1 - if passed, it will be used as eventing config directory
function install_knative_eventing() {
  local kne_config
  kne_config="${1:-${EVENTING_CONFIG}}"
  # Install Knative Eventing in the current cluster.
  echo "Installing Knative Eventing from: ${kne_config}"
  if [ -f "${kne_config}" ] || [ -d "${kne_config}" ]; then
    ko apply --strict -f "${kne_config}" || return $?
  else
    kubectl apply -f "${kne_config}" || return $?
    UNINSTALL_LIST+=( "${kne_config}" )
  fi

  wait_until_pods_running knative-eventing || fail_test "Knative Eventing did not come up"

  if ! (( DEPLOY_KNATIVE_MONITORING )); then return 0; fi

  # Ensure knative monitoring is installed only once
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

function run_preinstall_V016() {
  ko apply --strict -f ${PRE_INSTALL_V016} || return 1
  wait_until_batch_job_complete knative-eventing || return 1
}

function install_mt_broker() {
  ko apply --strict -f ${MT_CHANNEL_BASED_BROKER_DEFAULT_CONFIG} || return 1
  ko apply --strict -f ${MT_CHANNEL_BASED_BROKER_CONFIG_DIR} || return 1
  wait_until_pods_running knative-eventing || return 1
  kubectl -n knative-eventing set env deployment/mt-broker-controller BROKER_INJECTION_DEFAULT=true || return 1
  wait_until_pods_running knative-eventing || fail_test "Knative Eventing with MT Broker did not come up"
}

# Teardown the Knative environment after tests finish.
function knative_teardown() {
  echo ">> Stopping Knative Eventing"
  echo "Uninstalling Knative Eventing"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${EVENTING_CONFIG}
  wait_until_object_does_not_exist namespaces knative-eventing

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
  ko apply --strict -f ${IN_MEMORY_CHANNEL_CRD_CONFIG_DIR} || return 1
  wait_until_pods_running knative-eventing || fail_test "Failed to install the In-Memory Channel CRD"
}

function uninstall_channel_crds() {
  echo "Uninstalling In-Memory Channel CRD"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${IN_MEMORY_CHANNEL_CRD_CONFIG_DIR}
}

function dump_extra_cluster_state() {
  # Collecting logs from all knative's eventing pods.
  echo "============================================================"
  local namespace="knative-eventing"
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

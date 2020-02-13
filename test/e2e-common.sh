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

# Latest release. If user does not supply this as a flag, the latest
# tagged release on the current branch will be used.
readonly LATEST_RELEASE_VERSION=$(git describe --match "v[0-9]*" --abbrev=0)

UNINSTALL_LIST=()

# Install Knative Eventing in the current cluster, and waits for it to be ready.
# If no parameters are passed, installs the current source-based build.
# Parameters: $1 - Knative Eventing YAML file
#             $2 - Knative Monitoring YAML file (optional)
function install_knative_eventing {
  local knative_networking_pods
  local INSTALL_RELEASE_YAML=$1
  echo ">> Installing Knative Eventing"
  if [[ -z "$1" ]]; then
    install_head
  else
    kubectl apply -f "${INSTALL_RELEASE_YAML}" || return $?
  fi
  wait_until_pods_running knative-eventing || fail_test "Knative Eventing did not come up"

  # Ensure knative monitoring is installed only once
  knative_networking_pods=$(kubectl get pods -n knative-monitoring \
    --field-selector status.phase=Running 2> /dev/null | tail -n +2 | wc -l)
  if ! [[ ${knative_networking_pods} -gt 0 ]]; then
    echo ">> Installing Knative Monitoring"
    start_knative_monitoring "${KNATIVE_MONITORING_RELEASE}" || fail_test "Knative Monitoring did not come up"
    UNINSTALL_LIST+=( "${KNATIVE_MONITORING_RELEASE}" )
  else
    echo ">> Knative Monitoring seems to be running, pods running: ${knative_networking_pods}."
  fi
}

function install_head {
  ko apply -f ${EVENTING_CONFIG} || return $?
  wait_until_pods_running knative-eventing || fail_test "Knative Eventing did not come up"
}

function install_latest_release {
  header "Installing Knative latest public release"
  local url="https://github.com/knative/eventing/releases/download/${LATEST_RELEASE_VERSION}"
  local yaml="release.yaml"

  install_knative_eventing \
    "${url}/${yaml}" \
    || fail_test "Knative latest release installation failed"
  wait_until_pods_running knative-eventing || fail_test "Knative Eventing did not come up"
}

function knative_setup {
  install_knative_eventing
}

# Teardown the Knative environment after tests finish.
function knative_teardown() {
  echo ">> Stopping Knative Eventing"
  echo "Uninstalling Knative Eventing"
  ko delete --ignore-not-found=true --now --timeout 60s -f ${EVENTING_CONFIG}
  wait_until_object_does_not_exist namespaces knative-eventing

  echo ">> Uninstalling dependencies"
  # shellcheck disable=SC2068
  for i in ${!UNINSTALL_LIST[@]}; do
    # We uninstall elements in the reverse of the order they were installed.
    local YAML="${UNINSTALL_LIST[$(( ${#array[@]} - $i ))]}"
    echo ">> Bringing down YAML: ${YAML}"
    kubectl delete --ignore-not-found=true -f "${YAML}" || return 1
  done
}

# Setup resources common to all eventing tests.
function test_setup() {
  install_test_resources || return 1

  # Publish test images.
  echo ">> Publishing test images"
  "$(dirname $0)/upload-test-images.sh" e2e || fail_test "Error uploading test images"
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
  ko apply -f ${IN_MEMORY_CHANNEL_CRD_CONFIG_DIR} || return 1
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

function install_istio {
  if [[ -z "${ISTIO_VERSION}" ]]; then
    readonly ISTIO_VERSION="latest"
  fi
  echo ">> Installing Istio: ${ISTIO_VERSION}"

  local istio_base="./third_party/istio-${ISTIO_VERSION}"
  INSTALL_ISTIO_CRD_YAML="${istio_base}/istio-crds.yaml"
  INSTALL_ISTIO_YAML="${istio_base}/istio-minimal.yaml"

  echo "Istio CRD YAML: ${INSTALL_ISTIO_CRD_YAML}"
  echo "Istio YAML: ${INSTALL_ISTIO_YAML}"

  echo ">> Bringing up Istio"
  echo ">> Running Istio CRD installer"
  kubectl apply -f "${INSTALL_ISTIO_CRD_YAML}" || return 1
  wait_until_batch_job_complete istio-system || return 1
  UNINSTALL_LIST+=( "${INSTALL_ISTIO_CRD_YAML}" )

  echo ">> Running Istio"
  kubectl apply -f "${INSTALL_ISTIO_YAML}" || return 1
  UNINSTALL_LIST+=( "${INSTALL_ISTIO_YAML}" )
}

# Installs Knative Serving in the current cluster, and waits for it to be ready.
function install_knative_serving {
  install_istio
  echo ">> Installing Knative serving"
  local SERVING_VERSION
  SERVING_VERSION=$(go run test/scripts/resolve_matching_release.go \
    'knative/serving' "${LATEST_RELEASE_VERSION}") \
    || fail_test 'Could not resolve knative serving version'
  readonly SERVING_YAML="https://github.com/knative/serving/releases/download/${SERVING_VERSION}/serving.yaml"

  echo "Knative serving YAML: ${SERVING_YAML}"
  kubectl apply -f "${SERVING_YAML}" || return 1
  UNINSTALL_LIST+=( "${SERVING_YAML}" )

  wait_until_pods_running knative-serving || fail_test "Knative Serving did not come up"
}

function wait_for_file {
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

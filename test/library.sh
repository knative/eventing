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

# This is a collection of useful bash functions and constants, intended
# to be used in test scripts and the like. It doesn't do anything when
# called from command line.

# Default GKE version to be used with eventing
readonly EVENTING_GKE_VERSION=1.9.6-gke.1

# Useful environment variables
[[ -n "${PROW_JOB_ID}" ]] && IS_PROW=1 || IS_PROW=0
readonly IS_PROW
readonly EVENTING_ROOT_DIR="$(dirname $(readlink -f ${BASH_SOURCE}))/.."
readonly OUTPUT_GOBIN="${EVENTING_ROOT_DIR}/_output/bin"

# Copy of *_OVERRIDE variables
readonly OG_DOCKER_REPO="${DOCKER_REPO_OVERRIDE}"
readonly OG_K8S_CLUSTER="${K8S_CLUSTER_OVERRIDE}"
readonly OG_K8S_USER="${K8S_USER_OVERRIDE}"
readonly OG_KO_DOCKER_REPO="${KO_DOCKER_REPO}"

# Simple header for logging purposes.
function header() {
  echo "================================================="
  echo ${1^^}
  echo "================================================="
}

# Simple subheader for logging purposes.
function subheader() {
  echo "-------------------------------------------------"
  echo $1
  echo "-------------------------------------------------"
}

# Restores the *_OVERRIDE variables to their original value.
function restore_override_vars() {
  export DOCKER_REPO_OVERRIDE="${OG_DOCKER_REPO}"
  export K8S_CLUSTER_OVERRIDE="${OG_K8S_CLUSTER}"
  export K8S_USER_OVERRIDE="${OG_K8S_CLUSTER}"
  export KO_DOCKER_REPO="${OG_KO_DOCKER_REPO}"
}

# Waits until all pods are running in the given namespace or Completed.
# Parameters: $1 - namespace.
function wait_until_pods_running() {
  echo -n "Waiting until all pods in namespace $1 are up"
  for i in {1..150}; do  # timeout after 5 minutes
    local pods="$(kubectl get pods -n $1 | grep -v NAME)"
    local not_running=$(echo "${pods}" | grep -v Running | grep -v Completed | wc -l)
    if [[ -n "${pods}" && ${not_running} == 0 ]]; then
      echo -e "\nAll pods are up:"
      kubectl get pods -n $1
      return 0
    fi
    echo -n "."
    sleep 2
  done
  echo -e "\n\nERROR: timeout waiting for pods to come up"
  kubectl get pods -n $1
  return 1
}

# Sets the given user as cluster admin.
# Parameters: $1 - user
#             $2 - cluster name
#             $3 - cluster zone
function acquire_cluster_admin_role() {
  # Get the password of the admin and use it, as the service account (or the user)
  # might not have the necessary permission.
  local password=$(gcloud --format="value(masterAuth.password)" \
      container clusters describe $2 --zone=$3)
  kubectl --username=admin --password=$password \
      create clusterrolebinding cluster-admin-binding \
      --clusterrole=cluster-admin \
      --user=$1
}

# Waits until a namespace no longer exists
# Parameters: $1 - namespace.
function wait_until_namespace_does_not_exist() {
  echo -n "Waiting until namespace $1 does not exist"
  for i in {1..150}; do  # timeout after 5 minutes
    kubectl get namespaces $1 2>&1 > /dev/null || return 0
    echo -n "."
    sleep 2
  done
  echo -e "\n\nERROR: timeout waiting for namespace to not exist"
  kubectl get namespaces $1
  return 1
}

# Waits until a CRD no longer exists
# Parameters: $1 - crd.
function wait_until_crd_does_not_exist() {
  echo -n "Waiting until CRD $1 does not exist"
  for i in {1..150}; do  # timeout after 5 minutes
    kubectl get customresourcedefinitions $1 2>&1 > /dev/null || return 0
    echo -n "."
    sleep 2
  done
  echo -e "\n\nERROR: timeout waiting for CRD to not exist"
  kubectl get customresourcedefinitions $1
  return 1
}

function wait_until_object_does_not_exist() {
  local KIND=$1
  local NAMESPACE=$2
  local NAME=$3

  echo -n "Waiting until $KIND $NAMESPACE/$NAME does not exist"
  for i in {1..150}; do  # timeout after 5 minutes
    kubectl get -n $NAMESPACE $KIND $NAME 2>&1 > /dev/null || return 0
    echo -n "."
    sleep 2
  done
  echo -e "\n\nERROR: timeout waiting for $KIND $NAMESPACE/$NAME not to exist"
  kubectl get -n $NAMESPACE $KIND $NAME  $1
  return 1
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
  kubectl get -n $NAMESPACE $KIND $NAME  $1
  return 1
}

function validate_function_logs() {
  local NAMESPACE=$1
  local podname="$(kubectl -n $NAMESPACE get pods --no-headers -oname | grep e2e-k8s-events-)"
  echo "$podname"
  local logs="$(kubectl -n $NAMESPACE logs $podname user-container)"
  echo "${logs}" | grep "Started container" || return 1
  echo "${logs}" | grep "Created container" || return 1
  return 0
}

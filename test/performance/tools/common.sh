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

# Setup env vars
export PROJECT_NAME="knative-eventing-performance"
export USER_NAME="mako-job@knative-eventing-performance.iam.gserviceaccount.com"
export TEST_ROOT_PATH="$GOPATH/src/knative.dev/eventing/test/performance"
export KO_DOCKER_REPO="gcr.io/knative-eventing-performance"

function header() {
  echo "***** $1 *****"
}

# Exit script, dumping current state info.
# Parameters: $1 - error message (optional).
function abort() {
  [[ -n $1 ]] && echo "SCRIPT ERROR: $1"
  exit 1
}

# Creates a new cluster.
# $1 -> name, $2 -> zone/region, $3 -> num_nodes
function create_cluster() {
  header "Creating cluster $1 with $3 nodes in $2"
  gcloud beta container clusters create ${1} \
    --addons=HorizontalPodAutoscaling,HttpLoadBalancing \
    --machine-type=n1-standard-4 \
    --cluster-version=latest --region=${2} \
    --enable-stackdriver-kubernetes --enable-ip-alias \
    --num-nodes=${3} \
    --enable-autorepair \
    --scopes cloud-platform
}

# Create serice account secret on the cluster.
# $1 -> cluster_name, $2 -> cluster_zone
function create_secret() {
  echo "Create service account on cluster $1 in zone $2"
  gcloud container clusters get-credentials $1 --zone=$2 --project=${PROJECT_NAME} || abort "Failed to get cluster creds"
  kubectl create secret generic service-account --from-file=robot.json=${PERF_TEST_GOOGLE_APPLICATION_CREDENTIALS}
}

# Set up the user credentials for cluster operations.
function setup_user() {
  header "Setup User"

  gcloud config set core/account ${USER_NAME}
  gcloud auth activate-service-account ${USER_NAME} --key-file=${PERF_TEST_GOOGLE_APPLICATION_CREDENTIALS}
  gcloud config set core/project ${PROJECT_NAME}

  echo "gcloud user is $(gcloud config get-value core/account)"
  echo "Using secret defined in ${PERF_TEST_GOOGLE_APPLICATION_CREDENTIALS}"
}

# Get cluster credentials for GKE cluster
function get_gke_credentials() {
  echo "Updating cluster with name ${name} in zone ${zone}"
  gcloud container clusters get-credentials ${name} --zone=${zone} --project=${PROJECT_NAME} || abort "Failed to get cluster creds"
}

# Create a new cluster and install serving components and apply benchmark yamls.
# $1 -> cluster_name, $2 -> cluster_zone, $3 -> node_count
function create_new_cluster() {
  # create a new cluster
  create_cluster $1 $2 $3 || abort "Failed to create the new cluster $1"

  # create the secret on the new cluster
  create_secret $1 $2 || abort "Failed to create secrets on the new cluster"

  # Setup user credentials to run on GKE for continous runs.
  get_gke_credentials

  # update components on the cluster, e.g. serving and istio
  update_cluster $1 $2 || abort "Failed to update the cluster"
}

# Update resources installed on the cluster with the up-to-date code.
# $1 -> cluster_name, $2 -> cluster_zone
function update_cluster() {
  name=$1
  zone=$2
  local version="v0.8.0"  

  echo ">> Delete all existing jobs and test resources"
  kubectl delete job --all
  ko delete -f "${TEST_ROOT_PATH}/$1"

  pushd .
  cd ${GOPATH}/src/knative.dev

  echo ">> Update eventing"
  kubectl apply --selector knative.dev/crd-install=true \
  -f eventing/releases/download/$version/release.yaml || abort "Failed to apply eventing CRD's"

  kubectl apply -f eventing/releases/download/$version/release.yaml || abort "Failed to apply eventing sources"

  popd

  # Patch resources

  echo ">> Setting up config-mako"
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-mako
data:
  # This should only be used by our performance automation.
  environment: prod
EOF

  echo ">> Applying all the yamls"
  # install the service and cronjob to run the benchmark
  # NOTE: this assumes we have a benchmark with the same name as the cluster
  # If service creation takes long time, we will have some intially unreachable errors in the test
  cd $TEST_ROOT_PATH
  echo "Using ko version $(ko version)"
  ko apply -f $1 || abort "Failed to apply benchmarks yaml"
}

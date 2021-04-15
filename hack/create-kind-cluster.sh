#!/usr/bin/env bash

# Copyright 2021 The Knative Authors
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

set -e
set -o errexit
set -o nounset
set -o pipefail

CLUSTER_SUFFIX=${CLUSTER_SUFFIX:-"cluster.local"}
NODE_VERSION=${NODE_VERSION:-"v1.20.0"}
NODE_SHA=${NODE_SHA:-"sha256:b40ecf8bcb188f6a0d0f5d406089c48588b75edc112c6f635d26be5de1c89040"}

cat <<EOF | kind create cluster --config=-
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster

# This is needed in order to support projected volumes with service account tokens.
# See: https://kubernetes.slack.com/archives/CEKK1KTN2/p1600268272383600
kubeadmConfigPatches:
  - |
    apiVersion: kubeadm.k8s.io/v1beta2
    kind: ClusterConfiguration
    metadata:
      name: config
    apiServer:
      extraArgs:
        "service-account-issuer": "kubernetes.default.svc"
        "service-account-signing-key-file": "/etc/kubernetes/pki/sa.key"
    networking:
      dnsDomain: "${CLUSTER_SUFFIX}"
nodes:
- role: control-plane
  image: kindest/node:${NODE_VERSION}@${NODE_SHA}
- role: worker
  image: kindest/node:${NODE_VERSION}@${NODE_SHA}
EOF

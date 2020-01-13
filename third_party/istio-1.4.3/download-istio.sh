#!/usr/bin/env bash

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

# Download and unpack Istio
ISTIO_VERSION=1.4.3
DOWNLOAD_URL=https://github.com/istio/istio/releases/download/${ISTIO_VERSION}/istio-${ISTIO_VERSION}-linux.tar.gz

wget --no-check-certificate $DOWNLOAD_URL
if [ $? != 0 ]; then
  echo "Failed to download istio package"
  exit 1
fi
tar xzf istio-${ISTIO_VERSION}-linux.tar.gz

( # subshell in downloaded directory
cd istio-${ISTIO_VERSION} || exit

# Create CRDs template
helm template --namespace=istio-system \
  install/kubernetes/helm/istio-init \
  `# Removing trailing whitespaces to make automation happy` \
  | sed 's/[ \t]*$//' \
  > ../istio-crds.yaml

# An even lighter template, with just pilot/gateway and small resource requests.
# Based on install/kubernetes/helm/istio/values-istio-minimal.yaml
helm template --namespace=istio-system install/kubernetes/helm/istio --values ../values-local.yaml \
  `# Removing trailing whitespaces to make automation happy` \
  | sed 's/[ \t]*$//' \
  > ../istio-minimal.yaml
)

# Clean up.
rm -rf istio-${ISTIO_VERSION}
rm istio-${ISTIO_VERSION}-linux.tar.gz

# Add in the `istio-system` namespace to reduce number of commands.
patch istio-crds.yaml namespace.yaml.patch
patch istio-minimal.yaml namespace.yaml.patch

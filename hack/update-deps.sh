#!/usr/bin/env bash

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

readonly ROOT_DIR=$(dirname $0)/..
source ${ROOT_DIR}/vendor/knative.dev/test-infra/scripts/library.sh

set -o errexit
set -o nounset
set -o pipefail

cd ${ROOT_DIR}

# Ensure we have everything we need under vendor/
dep ensure

rm -rf $(find vendor/ -name 'OWNERS')
rm -rf $(find vendor/ -name 'OWNERS_ALIASES')
rm -rf $(find vendor/ -name 'BUILD')
rm -rf $(find vendor/ -name 'BUILD.bazel')

update_licenses third_party/VENDOR-LICENSE \
  $(find . -name "*.go" | grep -v vendor | xargs grep "package main" | cut -d: -f1 | xargs -n1 dirname | uniq)

# HACK HACK HACK
# The only way we found to create a consistent Trace tree without any missing Spans is to
# artificially set the SpanId. See pkg/tracing/traceparent.go for more details.
# Produced with:
# git diff origin/master HEAD -- vendor/go.opencensus.io/trace/trace.go > ./hack/set-span-id.patch
git apply ${REPO_ROOT_DIR}/hack/set-span-id.patch

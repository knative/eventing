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

# This script runs the performance tests; It is run by prow daily.
# For convenience, it can also be executed manually.

source $(dirname $0)/e2e-common.sh

# Override the default install_test_resources function by only installing Channel CRDs
function install_test_resources() {
    install_channel_crds() || return 1
}

# Override the default uninstall_test_resources function by only uninstalling Channel CRDs
function uninstall_test_resources() {
    uninstall_channel_crds()
}

initialize $@ --skip-istio-addon

# Run performance tests
go_test_e2e -tags="performance" -timeout=30m ./test/performance || fail_test

success

#!/usr/bin/env bash

# Copyright 2023 The Knative Authors
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

export GO111MODULE=on

source $(dirname "$0")/../test/e2e-common.sh

test_name="$1"
test_dir="$2"

if [[ -z "${test_name}" ]]; then
	fail_test "No testname provided"
fi

if [[ -z "${test_dir}" ]]; then
	fail_test "No testdir provided"
fi

header "Waiting for Knative eventing to come up"

wait_until_pods_running knative-eventing || fail_test "Pods in knative-eventing didn't come up"

header "Running tests"

go test -tags=e2e -v -timeout=30m -parallel=12 -run="${test_name}" "${test_dir}" || fail_test "Test(s) failed"

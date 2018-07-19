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

# This script runs the presubmit tests; it is started by prow for each PR.
# For convenience, it can also be executed manually.
# Running the script without parameters, or with the --all-tests
# flag, causes all tests to be executed, in the right order.
# Use the flags --build-tests, --unit-tests and --integration-tests
# to run a specific set of tests.

# Load github.com/knative/test-infra/images/prow-tests/scripts/presubmit-tests.sh
[ -f /workspace/presubmit-tests.sh ] \
  && source /workspace/presubmit-tests.sh \
  || eval "$(docker run --entrypoint sh gcr.io/knative-tests/test-infra/prow-tests -c 'cat presubmit-tests.sh')"
[ -v KNATIVE_TEST_INFRA ] || exit 1

# Directories with code to build/test
readonly CODE_PACKAGES=(cmd pkg sample)

# Convert list of packages into list of dirs for go
CODE_PACKAGES_STR="${CODE_PACKAGES[*]}"
CODE_PACKAGES_STR="./${CODE_PACKAGES_STR// /\/... .\/}/..."
readonly CODE_PACKAGES_STR

function build_tests() {
  header "Running build tests"
  local result=0
  go build -v ${CODE_PACKAGES_STR} || result=1
  subheader "Checking autogenerated code is up-to-date"
  ./hack/verify-codegen.sh || result=1
  return ${result}
}

function unit_tests() {
  header "Running unit tests"
  report_go_test ${CODE_PACKAGES_STR}
}

function integration_tests() {
  ./test/e2e-tests.sh
}

main $@

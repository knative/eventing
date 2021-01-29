#!/usr/bin/env bash

# Copyright 2020 The Knative Authors
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

# Docs -> file://./upgrade/README.md

# Script entry point.

export GO111MODULE=on

# shellcheck disable=SC1090
source "$(dirname "${BASH_SOURCE[0]}")/e2e-common.sh"

# Overrides

function knative_setup {
  # Nothing to do at setup
  true
}

function install_test_resources {
  # Nothing to install before tests
  true
}

function uninstall_test_resources {
  # Nothing to uninstall after tests
  true
}

initialize "$@" --skip-istio-addon

TIMEOUT=${TIMEOUT:-60m}

export GO_TEST_VERBOSITY="${GO_TEST_VERBOSITY:-standard-verbose}"

go_test_e2e \
  -tags=upgrade \
  -timeout="${TIMEOUT}" \
  ./test/upgrade \
  || fail_test

success

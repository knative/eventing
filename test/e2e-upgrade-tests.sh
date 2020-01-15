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

# Script entry point.

# shellcheck disable=SC1090
source "$(dirname "$0")/e2e-common.sh"

# Overrides

function knative_setup {
  install_knative_serving
  install_latest_release
}

function install_test_resources {
  # Nothing to install before tests
  true
}

function uninstall_test_resources {
  # Nothing to uninstall after tests
  true
}

# shellcheck disable=SC2068
initialize $@ --skip-istio-addon

TIMEOUT=${TIMEOUT:-30m}

header "Running preupgrade tests"

go_test_e2e -tags=preupgrade -timeout="${TIMEOUT}" ./test/upgrade || fail_test

header "Starting prober test"

go_test_e2e -tags=probe -timeout="${TIMEOUT}" ./test/upgrade &
PROBER_PID=$!
echo "Prober PID is ${PROBER_PID}"

wait_for_file /tmp/prober-ready || fail_test

header "Performing upgrade to HEAD"
install_head || return $?
install_channel_crds || return $?

header "Running postupgrade tests"
go_test_e2e -tags=postupgrade -timeout="${TIMEOUT}" ./test/upgrade || fail_test

header "Performing downgrade to latest release"
install_latest_release

header "Running postdowngrade tests"
go_test_e2e -tags=postdowngrade -timeout="${TIMEOUT}" ./test/upgrade || fail_test

# The prober is blocking on /tmp/prober-signal to know when it should exit.
echo "done" > /tmp/prober-signal

header "Waiting for prober test"
wait ${PROBER_PID} || fail_test "Prober failed"

success

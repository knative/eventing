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

source "$(dirname "$0")/e2e-common.sh"

# Overrides

function knative_setup {
  install_latest_release || fail_test 'Installing latest release of Knative Eventing failed'
}

function install_test_resources {
  # Nothing to install before tests
  true
}

function uninstall_test_resources {
  # Nothing to uninstall after tests
  true
}

initialize $@ --skip-istio-addon

TIMEOUT=${TIMEOUT:-30m}

header "Running preupgrade tests"

go_test_e2e -tags=preupgrade -timeout="${TIMEOUT}" ./test/upgrade || fail_test

header "Starting prober test"
rm -fv /tmp/prober-ready
go_test_e2e -tags=probe -timeout="${TIMEOUT}" ./test/upgrade &
PROBER_PID=$!
echo "Prober PID is ${PROBER_PID}"

wait_for_file /tmp/prober-ready || fail_test

header "Performing upgrade to HEAD"
install_head || fail_test 'Installing HEAD version of eventing failed'
install_channel_crds || fail_test 'Installing HEAD channel CRDs failed'
install_broker || fail_test 'Installing HEAD Broker failed'

run_postinstall || fail_test 'Running postinstall failed'

kubectl get brokers.v1alpha1.eventing.knative.dev --all-namespaces -oyaml
kubectl get brokers.v1beta1.eventing.knative.dev --all-namespaces -oyaml
kubectl get inmemorychannels.v1alpha1.messaging.knative.dev --all-namespaces -oyaml
kubectl get inmemorychannels.v1beta1.messaging.knative.dev --all-namespaces -oyaml
kubectl get subscriptions.v1alpha1.messaging.knative.dev --all-namespaces -oyaml
kubectl get subscriptions.v1beta1.messaging.knative.dev --all-namespaces -oyaml
kubectl get all --all-namespaces -oyaml

header "Running postupgrade tests"
go_test_e2e -tags=postupgrade -timeout="${TIMEOUT}" ./test/upgrade || fail_test

kubectl get brokers.v1alpha1.eventing.knative.dev --all-namespaces -oyaml
kubectl get brokers.v1beta1.eventing.knative.dev --all-namespaces -oyaml
kubectl get inmemorychannels.v1alpha1.messaging.knative.dev --all-namespaces -oyaml
kubectl get inmemorychannels.v1beta1.messaging.knative.dev --all-namespaces -oyaml
kubectl get subscriptions.v1alpha1.messaging.knative.dev --all-namespaces -oyaml
kubectl get subscriptions.v1beta1.messaging.knative.dev --all-namespaces -oyaml
kubectl get all --all-namespaces -oyaml

header "Performing downgrade to latest release"
install_latest_release || fail_test 'Installing latest release of Knative Eventing failed'

header "Running postdowngrade tests"
go_test_e2e -tags=postdowngrade -timeout="${TIMEOUT}" ./test/upgrade || fail_test

kubectl get brokers.v1alpha1.eventing.knative.dev --all-namespaces -oyaml
kubectl get brokers.v1beta1.eventing.knative.dev --all-namespaces -oyaml
kubectl get inmemorychannels.v1alpha1.messaging.knative.dev --all-namespaces -oyaml
kubectl get inmemorychannels.v1beta1.messaging.knative.dev --all-namespaces -oyaml
kubectl get subscriptions.v1alpha1.messaging.knative.dev --all-namespaces -oyaml
kubectl get subscriptions.v1beta1.messaging.knative.dev --all-namespaces -oyaml
kubectl get all --all-namespaces -oyaml

# The prober is blocking on /tmp/prober-signal to know when it should exit.
echo "done" > /tmp/prober-signal

header "Waiting for prober test"
wait ${PROBER_PID} || fail_test "Prober failed"

success

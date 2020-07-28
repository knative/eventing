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

# This script runs the end-to-end tests against eventing built from source.

# If you already have the *_OVERRIDE environment variables set, call
# this script with the --run-tests arguments and it will use the cluster
# and run the tests.

# Calling this script without arguments will create a new cluster in
# project $PROJECT_ID, start Knative eventing system, run the tests and
# delete the cluster.

export GO111MODULE=on

source "$(dirname "$0")/e2e-common.sh"

# Script entry point.

initialize $@ --skip-istio-addon

echo "Running tests with Multi Tenant Channel Based Broker"
go_test_e2e -timeout=30m -parallel=12 ./test/e2e -brokerclass=MTChannelBasedBroker -channels=messaging.knative.dev/v1beta1:Channel,messaging.knative.dev/v1beta1:InMemoryChannel,messaging.knative.dev/v1:Channel,messaging.knative.dev/v1:InMemoryChannel -sources=sources.knative.dev/v1alpha2:ApiServerSource,sources.knative.dev/v1alpha2:ContainerSource,sources.knative.dev/v1alpha2:PingSource || fail_test

success

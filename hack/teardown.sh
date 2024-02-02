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

# This script tears down the installed Knative Eventing components from source for local development.

set -e
set -o errexit
set -o nounset
set -o pipefail

source "$(dirname "$0")/../test/e2e-common.sh"

if [[ -e $(dirname "$0")/../tmp/uninstall_list.txt ]]; then
    read -a UNINSTALL_LIST < $(dirname "$0")/../tmp/uninstall_list.txt
else
    echo "install.sh failed to create ./tmp/uninstall_list.txt delete namespaces and crds manually if created" && exit
fi

knative_teardown || exit $?

rm -f $(dirname "$0")/../tmp/uninstall_list.txt
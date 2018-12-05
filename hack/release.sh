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

source $(dirname $0)/../vendor/github.com/knative/test-infra/scripts/release.sh

# Yaml files to generate, and the source config dir for them.
declare -A COMPONENTS
COMPONENTS=(
  ["eventing.yaml"]="config"
  ["in-memory-channel.yaml"]="config/provisioners/in-memory-channel"
  ["kafka.yaml"]="config/provisioners/kafka"
)
readonly COMPONENTS

declare -A RELEASES
RELEASES=(
  ["release.yaml"]="eventing.yaml in-memory-channel.yaml"
)
readonly RELEASES

# Script entry point.

initialize $@

set -o errexit
set -o pipefail

run_validation_tests ./test/presubmit-tests.sh

# Build the release

banner "Building the release"

# Build the components

all_yamls=()

for yaml in "${!COMPONENTS[@]}"; do
  config="${COMPONENTS[${yaml}]}"
  echo "Building Knative Eventing - ${config}"
  ko resolve ${KO_FLAGS} -f ${config}/ > ${yaml}
  tag_images_in_yaml ${yaml}
  all_yamls+=(${yaml})
done

# Assemble the release
for yaml in "${!RELEASES[@]}"; do
  echo "Assembling Knative Eventing - ${yaml}"
  echo "" > ${yaml}
  for component in ${RELEASES[${yaml}]}; do
    echo "---" >> ${yaml}
    echo "# ${component}" >> ${yaml}
    cat ${component} >> ${yaml}
  done
  tag_images_in_yaml ${yaml}
  all_yamls+=(${yaml})
done

echo "New release built successfully"

if (( ! PUBLISH_RELEASE )); then
 exit 0
fi

# Publish the release

for yaml in ${all_yamls[@]}; do
  publish_yaml ${yaml}
done

branch_release "Knative Eventing" "${all_yamls[*]}"

echo "New release published successfully"

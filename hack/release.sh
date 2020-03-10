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

# Documentation about this script and how to use it can be found
# at https://github.com/knative/test-infra/tree/master/ci

source $(dirname $0)/../vendor/knative.dev/test-infra/scripts/release.sh

# Yaml files to generate, and the source config dir for them.
declare -A COMPONENTS
COMPONENTS=(
  ["eventing-core.yaml"]="config/core"
  ["eventing-crds.yaml"]="config/core/resources"
  ["channel-broker.yaml"]="config/brokers/channel-broker"
  ["in-memory-channel.yaml"]="config/channels/in-memory-channel"
)
readonly COMPONENTS

declare -A RELEASES
RELEASES=(
  ["eventing.yaml"]="eventing-core.yaml channel-broker.yaml in-memory-channel.yaml"
)
readonly RELEASES

function build_release() {
  # Update release labels if this is a tagged release
  if [[ -n "${TAG}" ]]; then
    echo "Tagged release, updating release labels to eventing.knative.dev/release: \"${TAG}\""
    LABEL_YAML_CMD=(sed -e "s|eventing.knative.dev/release: devel|eventing.knative.dev/release: \"${TAG}\"|")
  else
    echo "Untagged release, will NOT update release labels"
    LABEL_YAML_CMD=(cat)
  fi

  # Build the components
  local all_yamls=()
  for yaml in "${!COMPONENTS[@]}"; do
    local config="${COMPONENTS[${yaml}]}"
    echo "Building Knative Eventing - ${config}"
    # TODO(chizhg): reenable --strict mode after https://github.com/knative/test-infra/issues/1262 is fixed.
    ko resolve ${KO_FLAGS} -R -f ${config}/ | "${LABEL_YAML_CMD[@]}" > ${yaml}
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
    all_yamls+=(${yaml})
  done
  ARTIFACTS_TO_PUBLISH="${all_yamls[@]}"
}

main $@

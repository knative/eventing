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

readonly EVENTING_CORE_YAML="eventing-core.yaml"
readonly EVENTING_CRDS_YAML="eventing-crds.yaml"
readonly SUGAR_CONTROLLER_YAML="eventing-sugar-controller.yaml"
readonly MT_CHANNEL_BROKER_YAML="mt-channel-broker.yaml"
readonly IN_MEMORY_CHANNEL="in-memory-channel.yaml"
readonly PRE_INSTALL="eventing-pre-install-jobs.yaml"
readonly POST_INSTALL="eventing-post-install-jobs.yaml"

declare -A RELEASES
RELEASES=(
  ["eventing.yaml"]="${EVENTING_CORE_YAML} ${MT_CHANNEL_BROKER_YAML} ${IN_MEMORY_CHANNEL}"
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
  echo "Building Knative Eventing"
  # Create eventing core yaml
  ko resolve ${KO_FLAGS} -R -f config/core/ | "${LABEL_YAML_CMD[@]}" > "${EVENTING_CORE_YAML}"

  # Create eventing crds yaml
  ko resolve ${KO_FLAGS} -f config/core/resources/ | "${LABEL_YAML_CMD[@]}" > "${EVENTING_CRDS_YAML}"

  # Create sugar controller yaml
  ko resolve ${KO_FLAGS} -f config/sugar/ | "${LABEL_YAML_CMD[@]}" > "${SUGAR_CONTROLLER_YAML}"

  # Create mt channel broker yaml
  ko resolve ${KO_FLAGS} -f config/brokers/mt-channel-broker/ | "${LABEL_YAML_CMD[@]}" > "${MT_CHANNEL_BROKER_YAML}"

  # Create in memory channel yaml
  ko resolve ${KO_FLAGS} -f config/channels/in-memory-channel/ | "${LABEL_YAML_CMD[@]}" > "${IN_MEMORY_CHANNEL}"

  # Create v0.18.0 pre-install job yaml. Upgrades some resources' storage versions.
  ko resolve ${KO_FLAGS} -f config/pre-install/v0.18.0/ | "${LABEL_YAML_CMD[@]}" > "${PRE_INSTALL}"

  local all_yamls=(${EVENTING_CORE_YAML} ${EVENTING_CRDS_YAML} ${SUGAR_CONTROLLER_YAML} ${MT_CHANNEL_BROKER_YAML} ${IN_MEMORY_CHANNEL} ${PRE_INSTALL})

  # # Template for POST_INSTALL usage:
  # # Create vX.Y.Z post-install job yaml.
  # ko resolve ${KO_FLAGS} -f config/post-install/vX.Y.Z/ | "${LABEL_YAML_CMD[@]}" > "${POST_INSTALL}"
  # # If used, add  ${POST_INSTALL} to all_yamls,
  # all_yamls+=(${POST_INSTALL})

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

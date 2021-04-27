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

# This script builds all the YAMLs that Knative eventing publishes. It may be
# varied between different branches, of what it does, but the following usage
# must be observed:
#
# generate-yamls.sh  <repo-root-dir> <generated-yaml-list>
#     repo-root-dir         the root directory of the repository.
#     generated-yaml-list   an output file that will contain the list of all
#                           YAML files. The first file listed must be our
#                           manifest that contains all images to be tagged.

# Different versions of our scripts should be able to call this script with
# such assumption so that the test/publishing/tagging steps can evolve
# differently than how the YAMLs are built.

# The following environment variables affect the behavior of this script:
# * `$KO_FLAGS` Any extra flags that will be passed to ko.
# * `$YAML_OUTPUT_DIR` Where to put the generated YAML files, otherwise a
#   random temporary directory will be created. **All existing YAML files in
#   this directory will be deleted.**
# * `$KO_DOCKER_REPO` If not set, use ko.local as the registry.

set -o errexit
set -o pipefail

readonly YAML_REPO_ROOT=${1:?"First argument must be the repo root dir"}
readonly YAML_LIST_FILE=${2:?"Second argument must be the output file"}

# Set output directory
if [[ -z "${YAML_OUTPUT_DIR:-}" ]]; then
  readonly YAML_OUTPUT_DIR="$(mktemp -d)"
fi
rm -fr ${YAML_OUTPUT_DIR}/*.yaml

# Generated Knative component YAML files
readonly EVENTING_CORE_YAML=${YAML_OUTPUT_DIR}/"eventing-core.yaml"
readonly EVENTING_CRDS_YAML=${YAML_OUTPUT_DIR}/"eventing-crds.yaml"
readonly EVENTING_SUGAR_CONTROLLER_YAML=${YAML_OUTPUT_DIR}/"eventing-sugar-controller.yaml"
readonly EVENTING_MT_CHANNEL_BROKER_YAML=${YAML_OUTPUT_DIR}/"mt-channel-broker.yaml"
readonly EVENTING_IN_MEMORY_CHANNEL_YAML=${YAML_OUTPUT_DIR}/"in-memory-channel.yaml"
readonly EVENTING_YAML=${YAML_OUTPUT_DIR}"/eventing.yaml"
# Tools
readonly EVENT_DISPLAY_YAML=${YAML_OUTPUT_DIR}"/event-display.yaml"
readonly APPENDER_YAML=${YAML_OUTPUT_DIR}"/appender.yaml"
readonly WEBSOCKET_SOURCE_YAML=${YAML_OUTPUT_DIR}"/websocket-source.yaml"
declare -A RELEASES
RELEASES=(
  [${EVENTING_YAML}]="${EVENTING_CORE_YAML} ${EVENTING_MT_CHANNEL_BROKER_YAML} ${EVENTING_IN_MEMORY_CHANNEL_YAML}"
)
readonly RELEASES
# Flags for all ko commands
KO_YAML_FLAGS="-P"
KO_FLAGS="${KO_FLAGS:-}"
[[ "${KO_DOCKER_REPO}" != gcr.io/* ]] && KO_YAML_FLAGS=""

if [[ "${KO_FLAGS}" != *"--platform"* ]]; then
  KO_YAML_FLAGS="${KO_YAML_FLAGS} --platform=all"
fi

readonly KO_YAML_FLAGS="${KO_YAML_FLAGS} ${KO_FLAGS}"

if [[ -n "${TAG:-}" ]]; then
  LABEL_YAML_CMD=(sed -e "s|eventing.knative.dev/release: devel|eventing.knative.dev/release: \"${TAG}\"|")
else
  LABEL_YAML_CMD=(cat)
fi

: ${KO_DOCKER_REPO:="ko.local"}
export KO_DOCKER_REPO

cd "${YAML_REPO_ROOT}"

# Build the components
echo "Building Knative Eventing"
# Create eventing core yaml
ko resolve ${KO_YAML_FLAGS} -R -f config/core/ | "${LABEL_YAML_CMD[@]}" > "${EVENTING_CORE_YAML}"

# Create eventing crds yaml
ko resolve ${KO_YAML_FLAGS} -f config/core/resources/ | "${LABEL_YAML_CMD[@]}" > "${EVENTING_CRDS_YAML}"

# Create sugar controller yaml
ko resolve ${KO_YAML_FLAGS} -f config/sugar/ | "${LABEL_YAML_CMD[@]}" > "${EVENTING_SUGAR_CONTROLLER_YAML}"

# Create mt channel broker yaml
ko resolve ${KO_YAML_FLAGS} -f config/brokers/mt-channel-broker/ | "${LABEL_YAML_CMD[@]}" > "${EVENTING_MT_CHANNEL_BROKER_YAML}"

# Create in memory channel yaml
ko resolve ${KO_YAML_FLAGS} -f config/channels/in-memory-channel/ | "${LABEL_YAML_CMD[@]}" > "${EVENTING_IN_MEMORY_CHANNEL_YAML}"

# Create the tools
ko resolve ${KO_YAML_FLAGS} -f config/tools/appender/ | "${LABEL_YAML_CMD[@]}" > "${APPENDER_YAML}"
ko resolve ${KO_YAML_FLAGS} -f config/tools/event-display/ | "${LABEL_YAML_CMD[@]}" > "${EVENT_DISPLAY_YAML}"
ko resolve ${KO_YAML_FLAGS} -f config/tools/websocket-source/ | "${LABEL_YAML_CMD[@]}" > "${WEBSOCKET_SOURCE_YAML}"

all_yamls=(${EVENTING_CORE_YAML} ${EVENTING_CRDS_YAML} ${EVENTING_SUGAR_CONTROLLER_YAML} ${EVENTING_MT_CHANNEL_BROKER_YAML} ${EVENTING_IN_MEMORY_CHANNEL_YAML} ${EVENTING_YAML} ${APPENDER_YAML} ${EVENT_DISPLAY_YAML} ${WEBSOCKET_SOURCE_YAML})

if [ -d "${YAML_REPO_ROOT}/config/post-install" ]; then

  readonly EVENTING_POST_INSTALL_YAML=${YAML_OUTPUT_DIR}/"eventing-post-install.yaml"

  echo "Resolving post install manifests"

  ko resolve ${KO_YAML_FLAGS} -f config/post-install/ | "${LABEL_YAML_CMD[@]}" > "${EVENTING_POST_INSTALL_YAML}"
  all_yamls+=(${EVENTING_POST_INSTALL_YAML})
fi

# Assemble the release
for yaml in "${!RELEASES[@]}"; do
  echo "Assembling Knative Eventing - ${yaml}"
  echo "" > ${yaml}
  for component in ${RELEASES[${yaml}]}; do
    echo "---" >> ${yaml}
    echo "# ${component##*/}" >> ${yaml}
    cat ${component} >> ${yaml}
  done
done

echo "All manifests generated"

for yaml in "${!all_yamls[@]}"; do
  echo "${all_yamls[${yaml}]}" >> ${YAML_LIST_FILE}
done

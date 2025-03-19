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
# at https://github.com/knative/test-infra/tree/main/ci

source "$(dirname "${BASH_SOURCE[0]}")/../vendor/knative.dev/hack/release.sh"

KNATIVE_EVENTING_INTEGRATIONS_IMAGES_RELEASE="$(get_latest_knative_yaml_source "eventing-integrations" "eventing-integrations-images")"
readonly KNATIVE_EVENTING_INTEGRATIONS_IMAGES_RELEASE
KNATIVE_EVENTING_TRANSFORMATIONS_IMAGES_RELEASE="$(get_latest_knative_yaml_source "eventing-integrations" "eventing-transformations-images")"
readonly KNATIVE_EVENTING_TRANSFORMATIONS_IMAGES_RELEASE

readonly KNATIVE_EVENTING_INTEGRATIONS_IMAGES_CM="${REPO_ROOT_DIR}/third_party/eventing-integrations-latest/eventing-integrations-images.yaml"
readonly KNATIVE_EVENTING_TRANSFORMATIONS_IMAGES_CM="${REPO_ROOT_DIR}/third_party/eventing-integrations-latest/eventing-transformations-images.yaml"

function update_eventing_integrations_release_cms() {
   curl "${KNATIVE_EVENTING_INTEGRATIONS_IMAGES_RELEASE}" --create-dirs -o "${KNATIVE_EVENTING_INTEGRATIONS_IMAGES_CM}" || return $?
   curl "${KNATIVE_EVENTING_TRANSFORMATIONS_IMAGES_RELEASE}" --create-dirs -o "${KNATIVE_EVENTING_TRANSFORMATIONS_IMAGES_CM}" || return $?
}

function check_knative_nightly() {
    local files=(
       "${KNATIVE_EVENTING_INTEGRATIONS_IMAGES_CM}"
       "${KNATIVE_EVENTING_TRANSFORMATIONS_IMAGES_CM}"
       "${REPO_ROOT_DIR}/config/core/configmaps/eventing-integrations-images.yaml"
       "${REPO_ROOT_DIR}/config/core/configmaps/eventing-transformations-images.yaml"
       "${REPO_ROOT_DIR}/config/400-config-eventing-integrations-images.yaml"
       "${REPO_ROOT_DIR}/config/400-config-eventing-transformations-images.yaml"
    )

    for file in "${files[@]}"; do
        if grep -q "knative-nightly" "$file"; then
            echo "Error: Found 'knative-nightly' in $file, is eventing-integrations for this major and minor '${TAG}' version already released? https://github.com/knative-extensions/eventing-integrations/releases"
            cat "${file}"
            return 1
        fi
    done

    echo "No 'knative-nightly' occurrences found."
}

function build_release() {
  if (( PUBLISH_TO_GITHUB )); then
    # For official releases, update eventing-integrations ConfigMaps and stop the release if a nightly image is found
    # in the ConfigMaps.
    update_eventing_integrations_release_cms || return $?
    check_knative_nightly || return $?
  fi

  # Run `generate-yamls.sh`, which should be versioned with the
  # branch since the detail of building may change over time.
  local YAML_LIST="$(mktemp)"
  export TAG
  $(dirname $0)/generate-yamls.sh "${REPO_ROOT_DIR}" "${YAML_LIST}"
  ARTIFACTS_TO_PUBLISH=$(cat "${YAML_LIST}" | tr '\n' ' ')
  if (( ! PUBLISH_RELEASE )); then
    # Copy the generated YAML files to the repo root dir if not publishing.
    cp ${ARTIFACTS_TO_PUBLISH} ${REPO_ROOT_DIR}
  fi
}

main $@

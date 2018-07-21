#!/bin/bash

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

set -o errexit
set -o pipefail

source "$(dirname $(readlink -f ${BASH_SOURCE}))/../test/library.sh"

# Set default GCS/GCR
: ${EVENTING_RELEASE_GCS:="knative-releases"}
: ${EVENTING_RELEASE_GCR:="gcr.io/knative-releases"}
readonly EVENTING_RELEASE_GCS
readonly EVENTING_RELEASE_GCR

# Local generated yaml file.
readonly OUTPUT_YAML=release-eventing.yaml
readonly OUTPUT_YAML_BUS_STUB=release-eventing-bus-stub.yaml
readonly OUTPUT_YAML_CLUSTERBUS_STUB=release-eventing-clusterbus-stub.yaml
readonly OUTPUT_YAML_BUS_GCPPUBSUB=release-eventing-bus-gcppubsub.yaml
readonly OUTPUT_YAML_CLUSTERBUS_GCPPUBSUB=release-eventing-clusterbus-gcppubsub.yaml
readonly OUTPUT_YAML_BUS_KAFKA=release-eventing-bus-kafka.yaml
readonly OUTPUT_YAML_CLUSTERBUS_KAFKA=release-eventing-clusterbus-kafka.yaml
readonly OUTPUT_YAML_SOURCE_K8SEVENTS=release-eventing-source-k8sevents.yaml
readonly OUTPUT_YAML_SOURCE_GCPPUBSUB=release-eventing-source-gcppubsub.yaml
readonly OUTPUT_YAML_SOURCE_GITHUB=release-eventing-source-github.yaml

function banner() {
  echo "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@"
  echo "@@@@ $1 @@@@"
  echo "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@"
}

# Tag Knative Eventing images in the yaml file with a tag.
# Parameters: $1 - yaml file to parse for images.
#             $2 - tag to apply.
function tag_knative_images() {
  [[ -z $2 ]] && return 0
  echo "Tagging images with $2"
  for image in $(grep -o "${EVENTING_RELEASE_GCR}/[a-z\./-]\+@sha256:[0-9a-f]\+" $1); do
    gcloud -q container images add-tag ${image} ${image%%@*}:$2
  done
}

# Copy the given yaml file to the release GCS bucket.
# Parameters: $1 - yaml file to copy.
function publish_yaml() {
  gsutil cp $1 gs://${EVENTING_RELEASE_GCS}/latest/
  if (( TAG_RELEASE )); then
    gsutil cp $1 gs://${EVENTING_RELEASE_GCS}/previous/${TAG}/
  fi
}

# Convert a Bus to a ClusterBus
# Parameters: $1 - yaml file containing a Bus.
#             $2 - yaml file to create containing a ClusterBus.
function make_clusterbus() {
  local BUS_YAML=$1
  local CLUSTERBUS_YAML=$2
  sed -e 's/^kind: Bus$/kind: ClusterBus/g' $BUS_YAML > $CLUSTERBUS_YAML
}

# Script entry point.

cd ${EVENTING_ROOT_DIR}

SKIP_TESTS=0
TAG_RELEASE=0
DONT_PUBLISH=0
KO_FLAGS="-P"

for parameter in "$@"; do
  case $parameter in
    --skip-tests)
      SKIP_TESTS=1
      shift
      ;;
    --tag-release)
      TAG_RELEASE=1
      shift
      ;;
    --publish)
      DONT_PUBLISH=0
      shift
      ;;
    --nopublish)
      DONT_PUBLISH=1
      KO_FLAGS="-L"
      shift
      ;;
    *)
      echo "error: unknown option ${parameter}"
      exit 1
      ;;
  esac
done

readonly SKIP_TESTS
readonly TAG_RELEASE
readonly DONT_PUBLISH
readonly KO_FLAGS

if (( ! SKIP_TESTS )); then
  banner "RUNNING RELEASE VALIDATION TESTS"
  # Run tests.
  ./test/presubmit-tests.sh
fi

banner "    BUILDING THE RELEASE   "

# Set the repository
export KO_DOCKER_REPO=${EVENTING_RELEASE_GCR}

TAG=""
if (( TAG_RELEASE )); then
  commit=$(git describe --tags --always --dirty)
  # Like kubernetes, image tag is vYYYYMMDD-commit
  TAG="v$(date +%Y%m%d)-${commit}"
fi
readonly TAG

if (( ! DONT_PUBLISH )); then
  echo "- Destination GCR: ${EVENTING_RELEASE_GCR}"
  echo "- Destination GCS: ${EVENTING_RELEASE_GCS}"
fi

echo "Building Knative Eventing"
ko resolve ${KO_FLAGS} -f config/ >> ${OUTPUT_YAML}
tag_knative_images ${OUTPUT_YAML} ${TAG}

echo "Building Knative Eventing - Stub Bus"
ko resolve ${KO_FLAGS} -f config/buses/stub/ >> ${OUTPUT_YAML_BUS_STUB}
tag_knative_images ${OUTPUT_YAML_BUS_STUB} ${TAG}
make_clusterbus ${OUTPUT_YAML_BUS_STUB} ${OUTPUT_YAML_CLUSTERBUS_STUB}
tag_knative_images ${OUTPUT_YAML_CLUSTERBUS_STUB} ${TAG}

echo "Building Knative Eventing - GCP Cloud Pub/Sub Bus"
ko resolve ${KO_FLAGS} -f config/buses/gcppubsub/ >> ${OUTPUT_YAML_BUS_GCPPUBSUB}
tag_knative_images ${OUTPUT_YAML_BUS_GCPPUBSUB} ${TAG}
make_clusterbus ${OUTPUT_YAML_BUS_GCPPUBSUB} ${OUTPUT_YAML_CLUSTERBUS_GCPPUBSUB}
tag_knative_images ${OUTPUT_YAML_CLUSTERBUS_GCPPUBSUB} ${TAG}

echo "Building Knative Eventing - Kafka Bus"
ko resolve ${KO_FLAGS} -f config/buses/kafka/ >> ${OUTPUT_YAML_BUS_KAFKA}
tag_knative_images ${OUTPUT_YAML_BUS_KAFKA} ${TAG}
make_clusterbus ${OUTPUT_YAML_BUS_KAFKA} ${OUTPUT_YAML_CLUSTERBUS_KAFKA}
tag_knative_images ${OUTPUT_YAML_CLUSTERBUS_KAFKA} ${TAG}

echo "Building Knative Eventing - K8s Events Source"
ko resolve ${KO_FLAGS} -f pkg/sources/k8sevents/ >> ${OUTPUT_YAML_SOURCE_K8SEVENTS}
tag_knative_images ${OUTPUT_YAML_SOURCE_K8SEVENTS} ${TAG}

echo "Building Knative Eventing - GCP Cloud Pub/Sub Source"
ko resolve ${KO_FLAGS} -f pkg/sources/gcppubsub/ >> ${OUTPUT_YAML_SOURCE_GCPPUBSUB}
tag_knative_images ${OUTPUT_YAML_SOURCE_GCPPUBSUB} ${TAG}

echo "Building Knative Eventing - GitHub Source"
ko resolve ${KO_FLAGS} -f pkg/sources/github/ >> ${OUTPUT_YAML_SOURCE_GITHUB}
tag_knative_images ${OUTPUT_YAML_SOURCE_GITHUB} ${TAG}

if (( DONT_PUBLISH )); then
  echo "New release built successfully"
  exit 0
fi

echo "Publishing release.yaml"
publish_yaml ${OUTPUT_YAML}
publish_yaml ${OUTPUT_YAML_BUS_STUB}
publish_yaml ${OUTPUT_YAML_CLUSTERBUS_STUB}
publish_yaml ${OUTPUT_YAML_BUS_GCPPUBSUB}
publish_yaml ${OUTPUT_YAML_CLUSTERBUS_GCPPUBSUB}
publish_yaml ${OUTPUT_YAML_BUS_KAFKA}
publish_yaml ${OUTPUT_YAML_CLUSTERBUS_KAFKA}
publish_yaml ${OUTPUT_YAML_SOURCE_K8SEVENTS}
publish_yaml ${OUTPUT_YAML_SOURCE_GCPPUBSUB}
publish_yaml ${OUTPUT_YAML_SOURCE_GITHUB}

echo "New release published successfully"

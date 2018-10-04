#!/bin/bash
#
# Copyright 2018 The Knative Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit

: ${DOCKER_REPO_OVERRIDE:?"You must set 'DOCKER_REPO_OVERRIDE', see DEVELOPMENT.md"}

export KO_DOCKER_REPO=${DOCKER_REPO_OVERRIDE}
IMAGE_PATHS_FILE="$(dirname $0)/image_paths.txt"
DOCKER_TAG=$1

while read -r IMAGE || [[ -n "$IMAGE" ]]; do
    if [ $(echo "$IMAGE" | grep -v -e "^#") ]; then
        ko publish -P $IMAGE
        if [ -n "$DOCKER_TAG" ]; then
            IMAGE=$KO_DOCKER_REPO/$IMAGE
            DIGEST=$(gcloud container images list-tags --format='get(digest)' $IMAGE)
            echo "Tagging $IMAGE:$DIGEST with $DOCKER_TAG"
            gcloud -q container images add-tag $IMAGE@$DIGEST $IMAGE:$DOCKER_TAG
        fi
    fi
done < "$IMAGE_PATHS_FILE"
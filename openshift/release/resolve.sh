#!/usr/bin/env bash

function resolve_resources(){
  local dir=$1
  local resolved_file_name=$2
  local image_prefix=$3
  local image_tag=$4

  [[ -n $image_tag ]] && image_tag=":$image_tag"

  echo "Writing resolved yaml to $resolved_file_name"

  > $resolved_file_name

  for yaml in "$dir"/*.yaml; do
    echo "---" >> $resolved_file_name
    # 1. Prefix test image references with test-
    # 2. Rewrite image references
    # 3. Remove comment lines
    # 4. Remove empty lines
    sed -e "s+\(.* image: \)\(knative.dev\)\(.*/\)\(test/\)\(.*\)+\1\2 \3\4test-\5+g" \
        -e "s+ko://++" \
        -e "s+knative.dev/eventing/cmd/broker/ingress+${image_prefix}ingress${image_tag}+" \
        -e "s+knative.dev/eventing/cmd/broker/filter+${image_prefix}filter${image_tag}+" \
        -e "s+knative.dev/eventing/cmd/mtbroker/ingress+${image_prefix}mtbroker-ingress${image_tag}+" \
        -e "s+knative.dev/eventing/cmd/mtbroker/filter+${image_prefix}mtbroker-filter${image_tag}+" \
        -e "s+knative.dev/eventing/cmd/channel_broker+${image_prefix}channel-broker${image_tag}+" \
        -e "s+knative.dev/eventing/cmd/mtchannel_broker+${image_prefix}mtchannel-broker${image_tag}+" \
        -e "s+knative.dev/eventing/cmd/in_memory/channel_controller+${image_prefix}channel-controller${image_tag}+" \
        -e "s+knative.dev/eventing/cmd/in_memory/channel_dispatcher+${image_prefix}channel-dispatcher${image_tag}+" \
        -e "s+knative.dev/eventing/cmd/ping+${image_prefix}ping${image_tag}+" \
        -e "s+knative.dev/eventing/cmd/mtping+${image_prefix}mtping${image_tag}+" \
        -e "s+knative.dev/eventing/cmd/apiserver_receive_adapter+${image_prefix}apiserver-receive-adapter${image_tag}+" \
        -e "s+\(.* image: \)\(knative.dev\)\(.*/\)\(.*\)+\1${image_prefix}\4${image_tag}+g" \
        -e '/^[ \t]*#/d' \
        -e '/^[ \t]*$/d' \
        "$yaml" >> $resolved_file_name
  done
}

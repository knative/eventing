/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	. "github.com/knative/eventing/contrib/kafka/pkg/reconciler"
)

const (
	testNumPartitions     = 10
	testReplicationFactor = 5
)

func TestKafkaChannelDefaults(t *testing.T) {
	testCases := map[string]struct {
		initial  KafkaChannel
		expected KafkaChannel
	}{
		"nil spec": {
			initial: KafkaChannel{},
			expected: KafkaChannel{
				Spec: KafkaChannelSpec{
					ConsumerMode:      ConsumerModeMultiplexConsumerValue,
					NumPartitions:     DefaultNumPartitions,
					ReplicationFactor: DefaultReplicationFactor,
				},
			},
		},
		"consumerMode empty": {
			initial: KafkaChannel{
				Spec: KafkaChannelSpec{
					ConsumerMode:      "",
					NumPartitions:     testNumPartitions,
					ReplicationFactor: testReplicationFactor,
				},
			},
			expected: KafkaChannel{
				Spec: KafkaChannelSpec{
					ConsumerMode:      ConsumerModeMultiplexConsumerValue,
					NumPartitions:     testNumPartitions,
					ReplicationFactor: testReplicationFactor,
				},
			},
		},
		"numPartitions not set": {
			initial: KafkaChannel{
				Spec: KafkaChannelSpec{
					ConsumerMode:      ConsumerModeMultiplexConsumerValue,
					ReplicationFactor: testReplicationFactor,
				},
			},
			expected: KafkaChannel{
				Spec: KafkaChannelSpec{
					ConsumerMode:      ConsumerModeMultiplexConsumerValue,
					NumPartitions:     DefaultNumPartitions,
					ReplicationFactor: testReplicationFactor,
				},
			},
		},
		"numPartitions negative": {
			initial: KafkaChannel{
				Spec: KafkaChannelSpec{
					ConsumerMode:      ConsumerModeMultiplexConsumerValue,
					ReplicationFactor: testReplicationFactor,
					NumPartitions:     -10,
				},
			},
			expected: KafkaChannel{
				Spec: KafkaChannelSpec{
					ConsumerMode:      ConsumerModeMultiplexConsumerValue,
					NumPartitions:     DefaultNumPartitions,
					ReplicationFactor: testReplicationFactor,
				},
			},
		},
		"replicationFactor not set": {
			initial: KafkaChannel{
				Spec: KafkaChannelSpec{
					ConsumerMode:  ConsumerModeMultiplexConsumerValue,
					NumPartitions: testNumPartitions,
				},
			},
			expected: KafkaChannel{
				Spec: KafkaChannelSpec{
					ConsumerMode:      ConsumerModeMultiplexConsumerValue,
					NumPartitions:     testNumPartitions,
					ReplicationFactor: DefaultReplicationFactor,
				},
			},
		},
		"replicationFactor negative": {
			initial: KafkaChannel{
				Spec: KafkaChannelSpec{
					ConsumerMode:      ConsumerModeMultiplexConsumerValue,
					NumPartitions:     testNumPartitions,
					ReplicationFactor: -10,
				},
			},
			expected: KafkaChannel{
				Spec: KafkaChannelSpec{
					ConsumerMode:      ConsumerModeMultiplexConsumerValue,
					NumPartitions:     testNumPartitions,
					ReplicationFactor: DefaultReplicationFactor,
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.initial.SetDefaults(context.TODO())
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatalf("Unexpected defaults (-want, +got): %s", diff)
			}
		})
	}
}

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
	"github.com/knative/eventing/contrib/kafka/pkg/utils"
	"testing"

	"github.com/google/go-cmp/cmp"
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
					NumPartitions:     utils.DefaultNumPartitions,
					ReplicationFactor: utils.DefaultReplicationFactor,
				},
			},
		},
		"numPartitions not set": {
			initial: KafkaChannel{
				Spec: KafkaChannelSpec{
					ReplicationFactor: testReplicationFactor,
				},
			},
			expected: KafkaChannel{
				Spec: KafkaChannelSpec{
					NumPartitions:     utils.DefaultNumPartitions,
					ReplicationFactor: testReplicationFactor,
				},
			},
		},
		"replicationFactor not set": {
			initial: KafkaChannel{
				Spec: KafkaChannelSpec{
					NumPartitions: testNumPartitions,
				},
			},
			expected: KafkaChannel{
				Spec: KafkaChannelSpec{
					NumPartitions:     testNumPartitions,
					ReplicationFactor: utils.DefaultReplicationFactor,
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

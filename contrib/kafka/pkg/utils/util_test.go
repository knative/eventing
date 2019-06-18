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

package utils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	cluster "github.com/bsm/sarama-cluster"
	"github.com/google/go-cmp/cmp"
	_ "github.com/knative/pkg/system/testing"
)

func TestGetKafkaConfig(t *testing.T) {

	testCases := []struct {
		name     string
		data     map[string]string
		path     string
		getError string
		expected *KafkaConfig
	}{
		{
			name:     "invalid config path",
			path:     "/tmp/does_not_exist",
			getError: "error loading configuration: lstat /tmp/does_not_exist: no such file or directory",
		},
		{
			name:     "configmap with no data",
			data:     map[string]string{},
			getError: "missing configuration",
		},
		{
			name:     "configmap with no bootstrapServers key",
			data:     map[string]string{"key": "value"},
			getError: "missing key bootstrapServers in configuration",
		},
		{
			name:     "configmap with empty bootstrapServers value",
			data:     map[string]string{"bootstrapServers": ""},
			getError: "empty bootstrapServers value in configuration",
		},
		{
			name: "single bootstrapServers",
			data: map[string]string{"bootstrapServers": "kafkabroker.kafka:9092"},
			expected: &KafkaConfig{
				Brokers: []string{"kafkabroker.kafka:9092"},
			},
		},
		{
			name: "multiple bootstrapServers",
			data: map[string]string{"bootstrapServers": "kafkabroker1.kafka:9092,kafkabroker2.kafka:9092"},
			expected: &KafkaConfig{
				Brokers: []string{"kafkabroker1.kafka:9092", "kafkabroker2.kafka:9092"},
			},
		},
		{
			name: "partition consumer",
			data: map[string]string{"bootstrapServers": "kafkabroker.kafka:9092", "consumerMode": "partitions"},
			expected: &KafkaConfig{
				Brokers:      []string{"kafkabroker.kafka:9092"},
				ConsumerMode: cluster.ConsumerModePartitions,
			},
		},
		{
			name: "default multiplex",
			data: map[string]string{"bootstrapServers": "kafkabroker.kafka:9092", "consumerMode": "multiplex"},
			expected: &KafkaConfig{
				Brokers:      []string{"kafkabroker.kafka:9092"},
				ConsumerMode: cluster.ConsumerModeMultiplex,
			},
		},
		{
			name: "default multiplex from invalid consumerMode",
			data: map[string]string{"bootstrapServers": "kafkabroker.kafka:9092", "consumerMode": "foo"},
			expected: &KafkaConfig{
				Brokers:      []string{"kafkabroker.kafka:9092"},
				ConsumerMode: cluster.ConsumerModeMultiplex,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Running %s", t.Name())

			if tc.path == "" {
				dir, err := ioutil.TempDir("", "util_test")
				if err != nil {
					t.Errorf("error creating tmp directory")
				}
				defer os.RemoveAll(dir)

				for k, v := range tc.data {
					if k != "" {
						path := filepath.Join(dir, k)
						if err := ioutil.WriteFile(path, []byte(v), 0600); err != nil {
							t.Errorf("error writing file %s: %s", path, err)
						}
					}
				}

				tc.path = dir
			}
			got, err := GetKafkaConfig(tc.path)

			if tc.getError != "" {
				if err == nil {
					t.Errorf("Expected Config error: '%v'. Actual nil", tc.getError)
				} else if err.Error() != tc.getError {
					t.Errorf("Unexpected Config error. Expected '%v'. Actual '%v'", tc.getError, err)
				}
				return
			} else if err != nil {
				t.Errorf("Unexpected Config error. Expected nil. Actual '%v'", err)
			}

			if diff := cmp.Diff(tc.expected, got); diff != "" {
				t.Errorf("unexpected Config (-want, +got) = %v", diff)
			}

		})
	}

}

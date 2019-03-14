package controller

import (
	"github.com/bsm/sarama-cluster"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	_ "github.com/knative/pkg/system/testing"
)

func TestGetProvisionerConfigBrokers(t *testing.T) {

	testCases := []struct {
		name     string
		data     map[string]string
		path     string
		getError string
		expected *KafkaProvisionerConfig
	}{
		{
			name:     "invalid config path",
			path:     "/tmp/does_not_exist",
			getError: "error loading provisioner configuration: lstat /tmp/does_not_exist: no such file or directory",
		},
		{
			name:     "configmap with no data",
			data:     map[string]string{},
			getError: "missing provisioner configuration",
		},
		{
			name:     "configmap with no bootstrap_servers key",
			data:     map[string]string{"key": "value"},
			getError: "missing key bootstrap_servers in provisioner configuration",
		},
		{
			name:     "configmap with empty bootstrap_servers value",
			data:     map[string]string{"bootstrap_servers": ""},
			getError: "empty bootstrap_servers value in provisioner configuration",
		},
		{
			name: "single bootstrap_servers",
			data: map[string]string{"bootstrap_servers": "kafkabroker.kafka:9092"},
			expected: &KafkaProvisionerConfig{
				Brokers: []string{"kafkabroker.kafka:9092"},
			},
		},
		{
			name: "multiple bootstrap_servers",
			data: map[string]string{"bootstrap_servers": "kafkabroker1.kafka:9092,kafkabroker2.kafka:9092"},
			expected: &KafkaProvisionerConfig{
				Brokers: []string{"kafkabroker1.kafka:9092", "kafkabroker2.kafka:9092"},
			},
		},
		{
			name: "partition consumer",
			data: map[string]string{"bootstrap_servers": "kafkabroker.kafka:9092", "consumer_mode": "partitions"},
			expected: &KafkaProvisionerConfig{
				Brokers:      []string{"kafkabroker.kafka:9092"},
				ConsumerMode: cluster.ConsumerModePartitions,
			},
		},
		{
			name: "default multiplex",
			data: map[string]string{"bootstrap_servers": "kafkabroker.kafka:9092", "consumer_mode": "multiplex"},
			expected: &KafkaProvisionerConfig{
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
			got, err := GetProvisionerConfig(tc.path)

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

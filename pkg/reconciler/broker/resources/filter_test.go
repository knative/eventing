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

package resources

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"

	_ "knative.dev/pkg/system/testing"
)

func TestMakeFilterDeployment(t *testing.T) {
	testCases := map[string]struct {
		args FilterArgs
		want []byte
	}{
		"happy": {
			args: FilterArgs{
				Broker: &v1alpha1.Broker{
					ObjectMeta: v1.ObjectMeta{
						Name: "happy",
					},
					Spec:   v1alpha1.BrokerSpec{},
					Status: v1alpha1.BrokerStatus{},
				},
				Image:              "image-uri",
				ServiceAccountName: "service-account-name",
			},
			want: []byte(`{
  "metadata": {
    "name": "happy-broker-filter",
    "creationTimestamp": null,
    "labels": {
      "eventing.knative.dev/broker": "happy",
      "eventing.knative.dev/brokerRole": "filter"
    },
    "ownerReferences": [
      {
        "apiVersion": "eventing.knative.dev/v1alpha1",
        "kind": "Broker",
        "name": "happy",
        "uid": "",
        "controller": true,
        "blockOwnerDeletion": true
      }
    ]
  },
  "spec": {
    "selector": {
      "matchLabels": {
        "eventing.knative.dev/broker": "happy",
        "eventing.knative.dev/brokerRole": "filter"
      }
    },
    "template": {
      "metadata": {
        "creationTimestamp": null,
        "labels": {
          "eventing.knative.dev/broker": "happy",
          "eventing.knative.dev/brokerRole": "filter"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "filter",
            "image": "image-uri",
            "ports": [
              {
                "name": "http",
                "containerPort": 8080
              },
              {
                "name": "metrics",
                "containerPort": 9092
              }
            ],
            "env": [
              {
                "name": "SYSTEM_NAMESPACE",
                "value": "knative-testing"
              },
              {
                "name": "NAMESPACE",
                "valueFrom": {
                  "fieldRef": {
                    "fieldPath": "metadata.namespace"
                  }
                }
              },
              {
                "name": "POD_NAME",
                "valueFrom": {
                  "fieldRef": {
                    "fieldPath": "metadata.name"
                  }
                }
              },
              {
                "name": "CONTAINER_NAME",
                "value": "filter"
              },
              {
                "name": "BROKER",
                "value": "happy"
              },
              {
                "name": "METRICS_DOMAIN",
                "value": "knative.dev/internal/eventing"
              }
            ],
            "resources": {},
            "livenessProbe": {
              "httpGet": {
                "path": "/healthz",
                "port": 8080
              },
              "initialDelaySeconds": 5,
              "periodSeconds": 2
            },
            "readinessProbe": {
              "httpGet": {
                "path": "/readyz",
                "port": 8080
              },
              "initialDelaySeconds": 5,
              "periodSeconds": 2
            }
          }
        ],
        "serviceAccountName": "service-account-name"
      }
    },
    "strategy": {}
  },
  "status": {}
}`),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			dep := MakeFilterDeployment(&tc.args)

			got, err := json.MarshalIndent(dep, "", "  ")
			if err != nil {
				t.Errorf("failed to marshal deployment, %s", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Log(string(got))
				t.Errorf("unexpected deployment (-want, +got) = %v", diff)
			}
		})
	}
}

func TestMakeFilterService(t *testing.T) {
	testCases := map[string]struct {
		broker v1alpha1.Broker
		want   []byte
	}{
		"happy": {
			broker: v1alpha1.Broker{
				ObjectMeta: v1.ObjectMeta{
					Name: "happy",
				},
				Spec:   v1alpha1.BrokerSpec{},
				Status: v1alpha1.BrokerStatus{},
			},
			want: []byte(`{
  "metadata": {
    "name": "happy-broker-filter",
    "creationTimestamp": null,
    "labels": {
      "eventing.knative.dev/broker": "happy",
      "eventing.knative.dev/brokerRole": "filter"
    },
    "ownerReferences": [
      {
        "apiVersion": "eventing.knative.dev/v1alpha1",
        "kind": "Broker",
        "name": "happy",
        "uid": "",
        "controller": true,
        "blockOwnerDeletion": true
      }
    ]
  },
  "spec": {
    "ports": [
      {
        "name": "http",
        "port": 80,
        "targetPort": 8080
      },
      {
        "name": "http-metrics",
        "port": 9090,
        "targetPort": 0
      }
    ],
    "selector": {
      "eventing.knative.dev/broker": "happy",
      "eventing.knative.dev/brokerRole": "filter"
    }
  },
  "status": {
    "loadBalancer": {}
  }
}`),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			dep := MakeFilterService(&tc.broker)

			got, err := json.MarshalIndent(dep, "", "  ")
			if err != nil {
				t.Errorf("failed to marshal deployment, %s", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Log(string(got))
				t.Errorf("unexpected deployment (-want, +got) = %v", diff)
			}
		})
	}
}

func TestMakeFilterLabels(t *testing.T) {
	testCases := map[string]struct {
		name string
		want map[string]string
	}{
		"with name": {
			name: "brokerName",
			want: map[string]string{
				"eventing.knative.dev/broker":     "brokerName",
				"eventing.knative.dev/brokerRole": "filter",
			},
		},
		"name empty": {
			name: "",
			want: map[string]string{
				"eventing.knative.dev/broker":     "",
				"eventing.knative.dev/brokerRole": "filter",
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			got := FilterLabels(tc.name)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected labels (-want, +got) = %v", diff)
			}
		})
	}
}

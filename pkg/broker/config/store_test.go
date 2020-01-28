/*
Copyright 2020 The Knative Authors

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

package config

import (
	"fmt"
	"knative.dev/eventing/pkg/kncloudevents"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	configmaptesting "knative.dev/pkg/configmap/testing"
	logtesting "knative.dev/pkg/logging/testing"
)

// expected config corresponding to `config-broker.yaml`
var wantFromFullConfigMap = BrokerConfig{
	IngressConfig: IngressConfig{
		LivenessProbe: corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
				},
			},
			InitialDelaySeconds: 50,
			PeriodSeconds:       20,
		},
		ConnectionArgs: kncloudevents.ConnectionArgs{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
		},
		TTL:         10,
		MetricsPort: 10,
	},
	FilterConfig: FilterConfig{
		LivenessProbe: corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
				},
			},
			InitialDelaySeconds: 50,
			PeriodSeconds:       20,
		},
		ReadinessProbe: corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/readyz",
					Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
				},
			},
			InitialDelaySeconds: 50,
			PeriodSeconds:       20,
		},
		ConnectionArgs: kncloudevents.ConnectionArgs{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 1,
		},
		MetricsPort: 10,
	},
}

// expected config corresponding to `config-broker-partial.yaml`
var wantFromPartialConfigMap = BrokerConfig{
	IngressConfig: IngressConfig{
		LivenessProbe: corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
				},
			},
			InitialDelaySeconds: 50,
			PeriodSeconds:       20,
		},
		ConnectionArgs: kncloudevents.ConnectionArgs{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
		},
		TTL:         10,
		MetricsPort: 10,
	},
	FilterConfig: FilterConfig{
		LivenessProbe: corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
				},
			},
			InitialDelaySeconds: 50,
			PeriodSeconds:       20,
		},
		// ReadinessProbe not provided in configmap. Defaults should be applied.
		ReadinessProbe: corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/readyz",
					Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       2,
		},
		ConnectionArgs: kncloudevents.ConnectionArgs{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 1,
		},
		MetricsPort: 10,
	},
}

func TestGetConfig(t *testing.T) {
	tests := []struct {
		name      string
		configmap string
		want      BrokerConfig
	}{
		{
			name:      "full config provided in configmap",
			configmap: "config-broker",
			want:      wantFromFullConfigMap,
		},
		{
			name:      "readiness probe not provided in configmap",
			configmap: "config-broker-partial",
			want:      wantFromPartialConfigMap,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStore(logtesting.TestLogger(t))
			configmap := configmaptesting.ConfigMapFromTestFile(t, tt.configmap, IngressConfigKey, FilterConfigKey)
			store.OnConfigChanged(configmap)
			got := store.GetConfig()
			if dif := cmp.Diff(got, tt.want); dif != "" {
				fmt.Println("Dif:", dif)
				t.Errorf("BrokerConfig mismatch. Dif: %+v \n", dif)
			}
		})
	}
}

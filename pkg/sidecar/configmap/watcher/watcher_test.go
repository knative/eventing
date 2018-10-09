/*
Copyright 2018 The Knative Authors

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

package watcher

import (
	"context"
	"errors"
	"github.com/google/go-cmp/cmp"
	controllertesting "github.com/knative/eventing/pkg/controller/testing"
	"github.com/knative/eventing/pkg/sidecar/configmap"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

const (
	namespace = "test-namespace"
	name      = "test-name"
)

func TestReconcile(t *testing.T) {
	testCases := map[string]struct {
		config          map[string]string
		getErr          error
		updateConfigErr error
		expectedErr     bool
		expectedConfig  *multichannelfanout.Config
	}{
		"cm not found": {
			expectedErr: false,
		},
		"get cm error": {
			expectedErr: true,
			getErr:      errors.New("test-error"),
		},
		"cannot parse cm": {
			config: map[string]string{
				configmap.MultiChannelFanoutConfigKey: "invalid config",
			},
			expectedErr: true,
		},
		"configUpdated fails": {
			expectedErr: true,
			config: map[string]string{
				configmap.MultiChannelFanoutConfigKey: "",
			},
			updateConfigErr: errors.New("test-error"),
			expectedConfig:  &multichannelfanout.Config{},
		},
		"success": {
			config: map[string]string{
				configmap.MultiChannelFanoutConfigKey: `
                    channelConfigs:
                        - name: foo
                          namespace: bar
                          fanoutConfig:
                              subscriptions:
                                  - callableDomain: callable
                                    sinkableDomain: sinkable`,
			},
			expectedConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Name:      "foo",
						Namespace: "bar",
						FanoutConfig: fanout.Config{
							Subscriptions: []duckv1alpha1.ChannelSubscriberSpec{
								{
									CallableDomain: "callable",
									SinkableDomain: "sinkable",
								},
							},
						},
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			mocks := controllertesting.Mocks{
				MockGets: []controllertesting.MockGet{
					func(innerClient client.Client, ctx context.Context, key client.ObjectKey, obj runtime.Object) (controllertesting.MockHandled, error) {
						if tc.getErr != nil {
							return controllertesting.Handled, tc.getErr
						}
						if tc.config != nil {
							obj.(*corev1.ConfigMap).Data = tc.config
							return controllertesting.Handled, nil
						}
						return controllertesting.Unhandled, nil
					},
				},
			}
			c := controllertesting.NewMockClient(fake.NewFakeClient(), mocks)

			cuc := &configUpdatedChecker{
				updateConfigErr: tc.updateConfigErr,
			}

			r := reconciler{
				logger:        zap.NewNop(),
				client:        c,
				configUpdated: cuc.updateConfig,
				cmName: types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				},
			}

			result, err := r.Reconcile(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: namespace,
					Name:      name,
				},
			})
			if tc.expectedErr != (err != nil) {
				t.Errorf("Unexpected error. Expected %v. Actual %v.", tc.expectedErr, err)
			}
			if diff := cmp.Diff(reconcile.Result{}, result); diff != "" {
				t.Errorf("Unexpected result (-want +got): %v", diff)
			}

			if diff := cmp.Diff(tc.expectedConfig, cuc.config); diff != "" {
				t.Errorf("Unexpected config (-want +got): %v", diff)
			}
		})
	}
}

type configUpdatedChecker struct {
	config          *multichannelfanout.Config
	updateConfigErr error
}

func (cuc *configUpdatedChecker) updateConfig(config *multichannelfanout.Config) error {
	cuc.config = config
	return cuc.updateConfigErr
}

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

package configmappropagation

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgotesting "k8s.io/client-go/testing"

	"knative.dev/eventing/pkg/apis/configs/v1alpha1"
	"knative.dev/eventing/pkg/client/clientset/versioned/scheme"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/configmappropagation/resources"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/tracker"

	. "knative.dev/eventing/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	currentNS                = "test-current-ns"
	configMapPropagationName = "test-cmp"
	originalConfigMapName    = "test-original-cm"
	originalNS               = "knative-eventing"
	copyConfigMapName        = configMapPropagationName + "-" + originalConfigMapName
)

var (
	selector = map[string]string{
		"testings": "testing",
	}
	originalSelector = map[string]string{
		"testings":                    "testing",
		resources.PropagationLabelKey: resources.PropagationLabelValueOriginal,
	}
	copySelector = map[string]string{
		resources.PropagationLabelKey: resources.PropagationLabelValueCopy,
		resources.CopyLabelKey:        currentNS + "-" + originalConfigMapName,
	}
	originalData = map[string]string{"data": "original"}
	copyData     = map[string]string{"data": "copy"}
)

func init() {
	// Add types to scheme
	_ = v1alpha1.AddToScheme(scheme.Scheme)
}

func TestAllCase(t *testing.T) {
	testKey := currentNS + "/" + configMapPropagationName
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		}, {
			Name: "ConfigMapPropagation not found",
			Key:  testKey,
		}, {
			Name: "ConfigMapPropagation is being deleted",
			Key:  testKey,
			Objects: []runtime.Object{
				NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationDeletionTimestamp),
			},
		}, {
			Name: "Original ConfigMap no longer has required labels",
			Key:  testKey,
			Objects: []runtime.Object{
				NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
				),
				NewConfigMap(originalConfigMapName, originalNS,
					WithConfigMapLabels(map[string]string{}),
				),
				NewConfigMap(copyConfigMapName, currentNS,
					WithConfigMapLabels(copySelector),
					WithConfigMapOwnerReference(NewConfigMapPropagation(configMapPropagationName, currentNS,
						WithInitConfigMapPropagationConditions,
						WithConfigMapPropagationSelector(selector),
					)),
				),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: copyConfigMapName,
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
					WithConfigMapPropagationPropagated,
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, configMapPropagationReadinessChanged, "ConfigMapPropagation %q became ready", configMapPropagationName),
			},
		}, {
			Name: "Original ConfigMap no longer exists, delete copy configMap succeeded",
			Key:  testKey,
			Objects: []runtime.Object{
				NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
				),
				NewConfigMap(copyConfigMapName, currentNS,
					WithConfigMapLabels(copySelector),
					WithConfigMapOwnerReference(NewConfigMapPropagation(configMapPropagationName, currentNS,
						WithInitConfigMapPropagationConditions,
						WithConfigMapPropagationSelector(selector),
					)),
				),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: copyConfigMapName,
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
					WithConfigMapPropagationPropagated,
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, configMapPropagationReadinessChanged, "ConfigMapPropagation %q became ready", configMapPropagationName),
			},
		}, {
			Name: "Original ConfigMap no longer exists, delete copy configMap failed",
			Key:  testKey,
			Objects: []runtime.Object{
				NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
				),
				NewConfigMap(copyConfigMapName, currentNS,
					WithConfigMapLabels(copySelector),
					WithConfigMapOwnerReference(NewConfigMapPropagation(configMapPropagationName, currentNS,
						WithInitConfigMapPropagationConditions,
						WithConfigMapPropagationSelector(selector),
					)),
				),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("delete", "configmaps"),
			},
			WantDeletes: []clientgotesting.DeleteActionImpl{{
				Name: copyConfigMapName,
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
					WithConfigMapPropagationNotPropagated,
				),
			}},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, configMapPropagationPropagateSingleConfigMapFailed,
					"Failed to propagate ConfigMap %v: inducing failure for delete configmaps", originalConfigMapName),
				Eventf(corev1.EventTypeWarning, configMapPropagationReconcileError,
					"ConfigMapPropagation reconcile error: one or more ConfigMap propagation failed"),
			},
		}, {
			Name: "Original ConfigMap has changed, update copy ConfigMap succeeded",
			Key:  testKey,
			Objects: []runtime.Object{
				NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
				),
				NewConfigMap(originalConfigMapName, originalNS,
					WithConfigMapLabels(originalSelector),
					WithConfigMapData(originalData),
				),
				NewConfigMap(copyConfigMapName, currentNS,
					WithConfigMapLabels(copySelector),
					WithConfigMapOwnerReference(NewConfigMapPropagation(configMapPropagationName, currentNS,
						WithInitConfigMapPropagationConditions,
						WithConfigMapPropagationSelector(selector),
					)),
				),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewConfigMap(copyConfigMapName, currentNS,
					WithConfigMapLabels(copySelector),
					WithConfigMapData(originalData),
					WithConfigMapOwnerReference(NewConfigMapPropagation(configMapPropagationName, currentNS,
						WithInitConfigMapPropagationConditions,
						WithConfigMapPropagationSelector(selector),
					)),
				),
			}},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("update", "configmaps"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
					WithConfigMapPropagationNotPropagated,
				),
			}},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, configMapPropagationPropagateSingleConfigMapFailed,
					"Failed to propagate ConfigMap %v: error updating ConfigMap in current namespace: inducing failure for update configmaps", originalConfigMapName),
				Eventf(corev1.EventTypeWarning, configMapPropagationReconcileError,
					"ConfigMapPropagation reconcile error: one or more ConfigMap propagation failed"),
			},
		}, {
			Name: "Original ConfigMap has changed, update copy ConfigMap failed",
			Key:  testKey,
			Objects: []runtime.Object{
				NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
				),
				NewConfigMap(originalConfigMapName, originalNS,
					WithConfigMapLabels(originalSelector),
					WithConfigMapData(originalData),
				),
				NewConfigMap(copyConfigMapName, currentNS,
					WithConfigMapLabels(copySelector),
					WithConfigMapOwnerReference(NewConfigMapPropagation(configMapPropagationName, currentNS,
						WithInitConfigMapPropagationConditions,
						WithConfigMapPropagationSelector(selector),
					)),
				),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewConfigMap(copyConfigMapName, currentNS,
					WithConfigMapLabels(copySelector),
					WithConfigMapData(originalData),
					WithConfigMapOwnerReference(NewConfigMapPropagation(configMapPropagationName, currentNS,
						WithInitConfigMapPropagationConditions,
						WithConfigMapPropagationSelector(selector),
					)),
				),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
					WithConfigMapPropagationPropagated,
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, configMapPropagationPropagateSingleConfigMapSucceed, "Propagate ConfigMap test-original-cm succeed"),
				Eventf(corev1.EventTypeNormal, configMapPropagationReadinessChanged, "ConfigMapPropagation %q became ready", configMapPropagationName),
			},
		}, {
			Name: "Copy ConfigMap has changed",
			Key:  testKey,
			Objects: []runtime.Object{
				NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
				),
				NewConfigMap(originalConfigMapName, originalNS,
					WithConfigMapLabels(originalSelector),
					WithConfigMapData(originalData),
				),
				NewConfigMap(copyConfigMapName, currentNS,
					WithConfigMapLabels(copySelector),
					WithConfigMapData(copyData),
					WithConfigMapOwnerReference(NewConfigMapPropagation(configMapPropagationName, currentNS,
						WithInitConfigMapPropagationConditions,
						WithConfigMapPropagationSelector(selector),
					)),
				),
			},
			WantUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewConfigMap(copyConfigMapName, currentNS,
					WithConfigMapLabels(copySelector),
					WithConfigMapData(originalData),
					WithConfigMapOwnerReference(NewConfigMapPropagation(configMapPropagationName, currentNS,
						WithInitConfigMapPropagationConditions,
						WithConfigMapPropagationSelector(selector),
					)),
				),
			}},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
					WithConfigMapPropagationPropagated,
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, configMapPropagationPropagateSingleConfigMapSucceed, "Propagate ConfigMap test-original-cm succeed"),
				Eventf(corev1.EventTypeNormal, configMapPropagationReadinessChanged, "ConfigMapPropagation %q became ready", configMapPropagationName),
			},
		}, {
			Name: "Copy ConfigMap no longer has required labels",
			Key:  testKey,
			Objects: []runtime.Object{
				NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
				),
				NewConfigMap(originalConfigMapName, originalNS,
					WithConfigMapLabels(originalSelector),
				),
				NewConfigMap(copyConfigMapName, currentNS,
					WithConfigMapLabels(map[string]string{}),
					WithConfigMapOwnerReference(NewConfigMapPropagation(configMapPropagationName, currentNS,
						WithInitConfigMapPropagationConditions,
						WithConfigMapPropagationSelector(selector),
					)),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
					WithConfigMapPropagationNotPropagated,
				),
			}},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, configMapPropagationPropagateSingleConfigMapFailed,
					`Failed to propagate ConfigMap %v: unable to update ConfigMap in current namespace, ConfigMap doesn't have "knative.dev/eventing/config-propagation:copy" label`, originalConfigMapName),
				Eventf(corev1.EventTypeWarning, configMapPropagationReconcileError,
					"ConfigMapPropagation reconcile error: one or more ConfigMap propagation failed"),
			},
		}, {
			Name: "Create new ConfigMap failed",
			Key:  testKey,
			Objects: []runtime.Object{
				NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
				),
				NewConfigMap(originalConfigMapName, originalNS,
					WithConfigMapLabels(originalSelector),
				),
			},
			WantCreates: []runtime.Object{
				NewConfigMap(copyConfigMapName, currentNS,
					WithConfigMapLabels(copySelector),
					WithConfigMapOwnerReference(NewConfigMapPropagation(configMapPropagationName, currentNS,
						WithInitConfigMapPropagationConditions,
						WithConfigMapPropagationSelector(selector),
					)),
				),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				InduceFailure("create", "configmaps"),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
					WithConfigMapPropagationNotPropagated,
				),
			}},
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, configMapPropagationPropagateSingleConfigMapFailed,
					"Failed to propagate ConfigMap %v: error creating ConfigMap in current namespace: inducing failure for create configmaps", originalConfigMapName),
				Eventf(corev1.EventTypeWarning, configMapPropagationReconcileError,
					"ConfigMapPropagation reconcile error: one or more ConfigMap propagation failed"),
			},
		}, {
			Name: "Successfully reconcile, became ready",
			Key:  testKey,
			Objects: []runtime.Object{
				NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
				),
				NewConfigMap(originalConfigMapName, originalNS,
					WithConfigMapLabels(originalSelector),
				),
			},
			WantCreates: []runtime.Object{
				NewConfigMap(copyConfigMapName, currentNS,
					WithConfigMapLabels(copySelector),
					WithConfigMapOwnerReference(NewConfigMapPropagation(configMapPropagationName, currentNS,
						WithInitConfigMapPropagationConditions,
						WithConfigMapPropagationSelector(selector),
					)),
				),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewConfigMapPropagation(configMapPropagationName, currentNS,
					WithInitConfigMapPropagationConditions,
					WithConfigMapPropagationSelector(selector),
					WithConfigMapPropagationPropagated,
				),
			}},
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, configMapPropagationPropagateSingleConfigMapSucceed, "Propagate ConfigMap test-original-cm succeed"),
				Eventf(corev1.EventTypeNormal, configMapPropagationReadinessChanged, "ConfigMapPropagation %q became ready", configMapPropagationName),
			},
		},
	}
	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			Base:                       reconciler.NewBase(ctx, controllerAgentName, cmw),
			configMapPropagationLister: listers.GetConfigMapPropagationLister(),
			configMapLister:            listers.GetConfigMapLister(),
			tracker:                    tracker.New(func(types.NamespacedName) {}, 0),
		}
	},
		false,
		logger,
	))
}

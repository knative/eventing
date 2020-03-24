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

package configmappropagation

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/reconciler"

	"knative.dev/eventing/pkg/apis/configs/v1alpha1"
	cmpreconciler "knative.dev/eventing/pkg/client/injection/reconciler/configs/v1alpha1/configmappropagation"
	configslisters "knative.dev/eventing/pkg/client/listers/configs/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/configmappropagation/resources"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/tracker"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process.
	configMapPropagationReconcileError                  = "ConfigMapPropagationReconcileError"
	configMapPropagationPropagateSingleConfigMapFailed  = "ConfigMapPropagationPropagateSingleConfigMapFailed"
	configMapPropagationPropagateSingleConfigMapSucceed = "ConfigMapPropagationPropagateSingleConfigMapSucceed"

	// Name of operation to propagate single configmap.
	copyConfigMap   = "Copy"
	deleteConfigMap = "Delete"
	// stopConfigMap indicates a configmap stop propagating.
	stopConfigMap = "Stop"
)

type Reconciler struct {
	kubeClientSet kubernetes.Interface

	// Listers index properties about resources
	configMapPropagationLister configslisters.ConfigMapPropagationLister
	configMapLister            corev1listers.ConfigMapLister

	tracker tracker.Interface
}

var configMapGVK = corev1.SchemeGroupVersion.WithKind("ConfigMap")

// Check that our Reconciler implements cmpreconciler.Interface.
var _ cmpreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, cmp *v1alpha1.ConfigMapPropagation) reconciler.Event {
	logging.FromContext(ctx).Debug("Reconciling", zap.Any("ConfigMapPropagation", cmp))
	cmp.Status.InitializeConditions()
	cmp.Status.CopyConfigMaps = []v1alpha1.ConfigMapPropagationStatusCopyConfigMap{}
	cmp.Status.ObservedGeneration = cmp.Generation

	// 1. Create/update ConfigMaps from original namespace to current namespace.
	// 2. Track changes of original ConfigMaps as well as copy ConfigMaps.

	// No need to reconcile if the ConfigMapPropagation has been marked for deletion.
	if cmp.DeletionTimestamp != nil {
		return nil
	}

	// Tell tracker to reconcile this ConfigMapPropagation whenever an original ConfigMap changes.
	// Note: Temporarily set Selector to match all objects.
	// If Selector is set to match specific labels (the required labels from ConfigMapPropagation),
	// the tracker can't track changes when an original ConfigMap no longer has required labels.
	// One alternative way is to track the name of every qualified original ConfigMap,
	// but this requires to track specific Selector as well, in order to notice newly qualified original ConfigMaps.
	originalConfigMapObjRef := tracker.Reference{
		Kind:       configMapGVK.Kind,
		APIVersion: configMapGVK.GroupVersion().String(),
		Namespace:  cmp.Spec.OriginalNamespace,
		Selector:   metav1.SetAsLabelSelector(map[string]string{}),
	}
	if err := r.tracker.TrackReference(originalConfigMapObjRef, cmp); err != nil {
		return err
	}

	// Tell tracker to reconcile this ConfigMapPropagation whenever the a copy ConfigMap changes.
	// Note: Temporarily set Selector to match all objects.
	// If Selector is set to match specific labels (the required labels from creation),
	// the tracker can't track changes when a copy ConfigMap no longer has required labels.
	// One alternative way is to track the name of every qualified copy ConfigMap.
	copyConfigMapObjRef := tracker.Reference{
		Kind:       configMapGVK.Kind,
		APIVersion: configMapGVK.GroupVersion().String(),
		Namespace:  cmp.Namespace,
		Selector:   metav1.SetAsLabelSelector(map[string]string{}),
	}
	if err := r.tracker.TrackReference(copyConfigMapObjRef, cmp); err != nil {
		logging.FromContext(ctx).Error("Unable to track changes to ConfigMap", zap.Error(err))
		return err
	}

	if err := r.reconcileConfigMap(ctx, cmp); err != nil {
		cmp.Status.MarkNotPropagated()
		return err
	}

	cmp.Status.MarkPropagated()
	return nil
}

func (r *Reconciler) reconcileConfigMap(ctx context.Context, cmp *v1alpha1.ConfigMapPropagation) error {
	// List ConfigMaps in original namespace and create/update copy ConfigMap in current namespace.
	var errs error
	originalConfigMapList, err := r.configMapLister.ConfigMaps(cmp.Spec.OriginalNamespace).List(resources.ExpectedOriginalSelector(cmp.Spec.Selector))
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the ConfigMap list in original namespace", zap.Error(err))
		return err
	}
	for _, configMap := range originalConfigMapList {
		name := resources.MakeCopyConfigMapName(cmp.Name, configMap.Name)
		source := types.NamespacedName{Namespace: cmp.Spec.OriginalNamespace, Name: configMap.Name}.String()
		resourceVersion := configMap.ResourceVersion
		var expectedStatus v1alpha1.ConfigMapPropagationStatusCopyConfigMap
		// Variable "succeed" represents whether a create/update action is successful or not.
		if err, succeed := r.createOrUpdateConfigMaps(ctx, cmp, configMap); err != nil {
			logging.FromContext(ctx).Warn("Failed to propagate ConfigMap: ", zap.Error(err))
			controller.GetEventRecorder(ctx).Eventf(cmp, corev1.EventTypeWarning, configMapPropagationPropagateSingleConfigMapFailed,
				fmt.Sprintf("Failed to propagate ConfigMap %v: %v", configMap.Name, err))
			if errs == nil {
				errs = fmt.Errorf("one or more ConfigMap propagation failed")
			}
			expectedStatus.SetCopyConfigMapStatus(name, source, copyConfigMap, "False", err.Error(), resourceVersion)
		} else if !succeed {
			// If there is no error, but the create/update action is not successful,
			// this indicates the copy configmap's copy label is removed.
			logging.FromContext(ctx).Debug("Stop propagating ConfigMap " + configMap.Name)
			controller.GetEventRecorder(ctx).Eventf(cmp, corev1.EventTypeNormal, configMapPropagationPropagateSingleConfigMapSucceed,
				fmt.Sprintf("Stop propagating ConfigMap: %s", configMap.Name))
			expectedStatus.SetCopyConfigMapStatus(name, source, stopConfigMap, "True",
				`copy ConfigMap doesn't have copy label, stop propagating this ConfigMap`, resourceVersion)
		} else {
			logging.FromContext(ctx).Debug("Propagate ConfigMap " + configMap.Name + " succeed")
			controller.GetEventRecorder(ctx).Eventf(cmp, corev1.EventTypeNormal, configMapPropagationPropagateSingleConfigMapSucceed,
				fmt.Sprintf("Propagate ConfigMap %v succeed", configMap.Name))
			expectedStatus.SetCopyConfigMapStatus(name, source, copyConfigMap, "True", "", resourceVersion)
		}
		// Update current copy configmap's status.
		cmp.Status.CopyConfigMaps = append(cmp.Status.CopyConfigMaps, expectedStatus)
	}
	// List ConfigMaps in current namespace and delete copy ConfigMap if the corresponding original ConfigMap no longer exists or no longer has the required label.
	copyConfigMapList, err := r.configMapLister.ConfigMaps(cmp.Namespace).List(labels.SelectorFromSet(map[string]string{resources.PropagationLabelKey: resources.PropagationLabelValueCopy}))
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the ConfigMap list in current namespace", zap.Error(err))
		return err
	}

	for _, copyConfigMap := range copyConfigMapList {
		// Select copy ConfigMap which is controlled by current ConfigMapPropagation.
		if metav1.IsControlledBy(copyConfigMap, cmp) {
			// Get the name of original ConfigMap.
			// The name of Copy ConfigMap is followed by <ConfigMapPropagation.Name>-<originalConfigMap.Name>.
			originalConfigMapName := strings.TrimPrefix(copyConfigMap.Name, cmp.Name+"-")
			source := types.NamespacedName{Namespace: cmp.Spec.OriginalNamespace, Name: originalConfigMapName}.String()
			var expectedStatus v1alpha1.ConfigMapPropagationStatusCopyConfigMap
			// Variable "succeed" represents whether a delete action is successful or not.
			if err, succeed := r.deleteOrKeepConfigMap(ctx, cmp, copyConfigMap, originalConfigMapName, originalConfigMapList); err != nil {
				logging.FromContext(ctx).Warn("Failed to propagate ConfigMap: ", zap.Error(err))
				controller.GetEventRecorder(ctx).Eventf(cmp, corev1.EventTypeWarning, configMapPropagationPropagateSingleConfigMapFailed,
					fmt.Sprintf("Failed to propagate ConfigMap %v: %v", originalConfigMapName, err))
				if errs == nil {
					errs = fmt.Errorf("one or more ConfigMap propagation failed")
				}
				expectedStatus.SetCopyConfigMapStatus(copyConfigMap.Name, source, deleteConfigMap, "False", err.Error(), "")
			} else if succeed {
				expectedStatus.SetCopyConfigMapStatus(copyConfigMap.Name, source, deleteConfigMap, "True", "", "")
			}
			if expectedStatus.Name != "" {
				// expectedStatus with same name will be merged and updated.
				cmp.Status.CopyConfigMaps = append(cmp.Status.CopyConfigMaps, expectedStatus)
			}
		}
	}

	return errs
}

// createOrUpdateConfigMaps will return error and bool (represents whether a create/update action is successful or not).
func (r *Reconciler) createOrUpdateConfigMaps(ctx context.Context, cmp *v1alpha1.ConfigMapPropagation, configMap *corev1.ConfigMap) (error, bool) {
	expected := resources.MakeConfigMap(resources.ConfigMapArgs{
		Original:             configMap,
		ConfigMapPropagation: cmp,
	})
	current, err := r.configMapLister.ConfigMaps(cmp.Namespace).Get(expected.Name)
	if err != nil && !apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get ConfigMap: "+current.Name+" in current namespace", zap.Error(err))
		return fmt.Errorf("error getting ConfigMap in current namespace: %w", err), false
	}

	// Only update ConfigMap with knative.dev/config-propagation:copy label.
	// If the ConfigMap does not have this label, the controller must not update the ConfigMap.
	if current != nil {
		label := current.GetLabels()
		succeed := true
		if label[resources.PropagationLabelKey] != resources.PropagationLabelValueCopy {
			//  OwnerReference will be removed when the knative.dev/config-propagation:copy label is not set in copy configmap
			//  so that this copy configmap will not be deleted if cmp is deleted.
			expected = current.DeepCopy()
			// Delete the CMP owner reference, and keep all other owner references (if any).
			expected.OwnerReferences = removeOwnerReference(expected.OwnerReferences, cmp.UID)
			// It will return false for the create/update action is not successful, due to removed copy label.
			// But it is not an error for ConfigMapPropagation for not propagating successfully.
			succeed = false
		}
		if current, err = r.kubeClientSet.CoreV1().ConfigMaps(expected.Namespace).Update(expected); err != nil {
			return fmt.Errorf("error updating ConfigMap in current namespace: %w", err), false
		}
		return nil, succeed
	}

	if current, err = r.kubeClientSet.CoreV1().ConfigMaps(expected.Namespace).Create(expected); err != nil {
		return fmt.Errorf("error creating ConfigMap in current namespace: %w", err), false
	}
	return nil, true
}

// deleteOrKeepConfigMap will return error and bool (represents whether a delete action is successful or not).
func (r *Reconciler) deleteOrKeepConfigMap(ctx context.Context, cmp *v1alpha1.ConfigMapPropagation, copyConfigMap *corev1.ConfigMap, originalConfigMapName string, originalConfigMapList []*corev1.ConfigMap) (error, bool) {
	originalConfigMap, contains := contains(originalConfigMapName, originalConfigMapList)
	expectedSelector := resources.ExpectedOriginalSelector(cmp.Spec.Selector)
	if !contains || !expectedSelector.Matches(labels.Set(originalConfigMap.Labels)) {
		// If Original ConfigMap no longer exists or no longer has the required label, delete copy ConfigMap.
		logging.FromContext(ctx).Info("Original ConfigMap " + originalConfigMapName +
			` no longer exists/no longer has "knative.dev/eventing/config-propagation:original" label, delete corresponding copy ConfigMap ` + copyConfigMap.Name)
		if err := r.kubeClientSet.CoreV1().ConfigMaps(cmp.Namespace).Delete(copyConfigMap.Name, &metav1.DeleteOptions{}); err != nil {
			logging.FromContext(ctx).Error("error deleting ConfigMap in current namespace", zap.Error(err))
			return err, false
		}
		return nil, true
	}
	return nil, false
}

// contains returns a configmap object if its name is in a configmaplist.
func contains(name string, list []*corev1.ConfigMap) (*corev1.ConfigMap, bool) {
	for _, configMap := range list {
		if configMap.Name == name {
			return configMap, true
		}
	}
	return nil, false
}

// removeOwnerReference removes the target ownerReference and returns a new slice of ownerReferences.
func removeOwnerReference(ownerReferences []metav1.OwnerReference, uid types.UID) []metav1.OwnerReference {
	var expected []metav1.OwnerReference
	for _, owner := range ownerReferences {
		if owner.UID != uid {
			expected = append(expected, owner)
		}
	}
	return expected
}

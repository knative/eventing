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

package trigger

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	pkgduck "knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	triggerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta1/trigger"
	listers "knative.dev/eventing/pkg/client/listers/eventing/v1beta1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler/sugar"
	"knative.dev/eventing/pkg/reconciler/sugar/resources"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process.
	brokerCreated = "BrokerCreated"

	// Name of the CE overrides extension uniquely identifying a source
	sourceIdExtension = "knsourcetrigger"
)

type Reconciler struct {
	// eventingClientSet allows us to configure Eventing objects
	eventingClientSet clientset.Interface
	brokerLister      listers.BrokerLister
	isEnabled         sugar.LabelFilterFn

	// Dynamic tracker to track Sources. In particular, it tracks the dependency between Triggers and Sources.
	sourceTracker    duck.ListableTracker
	dynamicClientSet dynamic.Interface
}

// Check that our Reconciler implements triggerreconciler.Interface
var _ triggerreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, t *v1beta1.Trigger) reconciler.Event {
	event := r.reconcileInjection(ctx, t)
	if event != nil {
		return event
	}

	return r.reconcileDependency(ctx, t)
}

func (r *Reconciler) reconcileInjection(ctx context.Context, t *v1beta1.Trigger) reconciler.Event {
	if !r.isEnabled(t.GetAnnotations()) {
		logging.FromContext(ctx).Debug("Injection for Trigger not enabled.")
		return nil
	}

	_, err := r.brokerLister.Brokers(t.Namespace).Get(t.Spec.Broker)

	// If the resource doesn't exist, we'll create it.
	if k8serrors.IsNotFound(err) {
		_, err = r.eventingClientSet.EventingV1beta1().Brokers(t.Namespace).Create(
			ctx, resources.MakeBroker(t.Namespace, t.Spec.Broker), metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("Unable to create Broker: %w", err)
		}
		return reconciler.NewEvent(corev1.EventTypeNormal, brokerCreated,
			fmt.Sprintf("Default eventing.knative.dev Broker %q created.", t.Spec.Broker))
	} else if err != nil {
		return fmt.Errorf("Unable to list Brokers: %w", err)
	}

	return nil
}

func (r *Reconciler) reconcileDependency(ctx context.Context, t *v1beta1.Trigger) reconciler.Event {
	// Don't trust the trigger status as it might be out-of-sync
	dependencyAnnotation, ok := t.GetAnnotations()[eventingv1.DependencyAnnotation]
	if !ok {
		return nil
	}

	filterAnnotation, ok := t.GetAnnotations()[eventingv1.FilterAnnotation]
	if !ok || filterAnnotation != "true" {
		return nil
	}

	dependencyObjRef, err := eventingv1.GetObjRefFromDependencyAnnotation(dependencyAnnotation)
	if err != nil {
		// The trigger controller has marked the dependency as invalid.
		return fmt.Errorf("getting object ref from dependency annotation %q: %v", dependencyAnnotation, err)
	}

	tracker := r.sourceTracker.TrackInNamespace(ctx, t)
	err = tracker(dependencyObjRef)
	if err != nil {
		return fmt.Errorf("tracking dependency: %v", err)
	}

	sourceLister, err := r.sourceTracker.ListerFor(dependencyObjRef)
	if err != nil {
		return fmt.Errorf("listing dependency: %v", err)
	}

	obj, err := sourceLister.ByNamespace(t.Namespace).Get(dependencyObjRef.Name)
	if err != nil {
		return fmt.Errorf("getting dependency: %v", err)
	}

	src := obj.(*duckv1.Source)

	gv, err := schema.ParseGroupVersion(src.APIVersion)
	if err != nil {
		return fmt.Errorf("parsing API version: %v", err)
	}

	sourceid := gv.Group + "/" + src.Namespace + "/" + src.Name

	// Patch the source object (if needed)
	if src.Spec.CloudEventOverrides == nil || src.Spec.CloudEventOverrides.Extensions[sourceIdExtension] != sourceid {
		logging.FromContext(ctx).Info("Patching source.")

		after := src.DeepCopy()
		if after.Spec.CloudEventOverrides == nil {
			after.Spec.CloudEventOverrides = &duckv1.CloudEventOverrides{}
		}
		if after.Spec.CloudEventOverrides.Extensions == nil {
			after.Spec.CloudEventOverrides.Extensions = map[string]string{}
		}

		after.Spec.CloudEventOverrides.Extensions[sourceIdExtension] = sourceid

		patch, err := pkgduck.CreateMergePatch(src, after)
		if err != nil {
			return err
		}

		// If there is nothing to patch, we are good, just return.
		// Empty patch is {}, hence we check for that.
		if len(patch) <= 2 {
			return nil
		}

		resourceClient, err := duck.ResourceInterface(r.dynamicClientSet, t.Namespace, src.GroupVersionKind())
		if err != nil {
			logging.FromContext(ctx).Warnw("Failed to create dynamic resource client", zap.Error(err))
			return err
		}
		_, err = resourceClient.Patch(ctx, after.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			logging.FromContext(ctx).Warnw("Failed to patch the source", zap.Error(err), zap.Any("patch", patch))
			return err
		}
	}

	// Patch the trigger object (if needed)
	if t.Spec.Filter == nil || t.Spec.Filter.Attributes[sourceIdExtension] != sourceid {
		logging.FromContext(ctx).Info("Patching trigger.")

		after := t.DeepCopy()
		if after.Spec.Filter == nil {
			after.Spec.Filter = &v1beta1.TriggerFilter{}
		}
		if after.Spec.Filter.Attributes == nil {
			after.Spec.Filter.Attributes = map[string]string{}
		}
		after.Spec.Filter.Attributes[sourceIdExtension] = sourceid

		patch, err := pkgduck.CreateMergePatch(t, after)
		if err != nil {
			return err
		}

		// If there is nothing to patch, we are good, just return.
		// Empty patch is {}, hence we check for that.
		if len(patch) <= 2 {
			return nil
		}

		_, err = r.eventingClientSet.EventingV1beta1().Triggers(t.Namespace).Patch(ctx, after.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			logging.FromContext(ctx).Warnw("Failed to patch the trigger", zap.Error(err), zap.Any("patch", patch))
			return err
		}
	}

	return nil
}

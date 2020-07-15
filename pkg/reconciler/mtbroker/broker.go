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

package mtbroker

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1"
	messaginglisters "knative.dev/eventing/pkg/client/listers/messaging/v1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler/mtbroker/resources"
	"knative.dev/eventing/pkg/reconciler/names"
	"knative.dev/pkg/apis"
	duckapis "knative.dev/pkg/apis/duck"
	pkgduckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/system"
)

const (
	// Name of the corev1.Events emitted from the Broker reconciliation process.
	brokerReconciled = "BrokerReconciled"
)

type Reconciler struct {
	eventingClientSet clientset.Interface
	dynamicClientSet  dynamic.Interface
	kubeClientSet     kubernetes.Interface

	// listers index properties about resources
	endpointsLister    corev1listers.EndpointsLister
	subscriptionLister messaginglisters.SubscriptionLister
	triggerLister      eventinglisters.TriggerLister

	channelableTracker duck.ListableTracker

	// Dynamic tracker to track KResources. In particular, it tracks the dependency between Triggers and Sources.
	kresourceTracker duck.ListableTracker

	// Dynamic tracker to track AddressableTypes. In particular, it tracks Trigger subscribers.
	addressableTracker duck.ListableTracker
	uriResolver        *resolver.URIResolver

	// If specified, only reconcile brokers with these labels
	brokerClass string
}

// Check that our Reconciler implements Interface
var _ brokerreconciler.Interface = (*Reconciler)(nil)
var _ brokerreconciler.Finalizer = (*Reconciler)(nil)

var brokerGVK = eventingv1.SchemeGroupVersion.WithKind("Broker")

// ReconcilerArgs are the arguments needed to create a broker.Reconciler.
type ReconcilerArgs struct {
	IngressImage              string
	IngressServiceAccountName string
	FilterImage               string
	FilterServiceAccountName  string
}

func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, brokerReconciled, "Broker reconciled: \"%s/%s\"", namespace, name)
}

func (r *Reconciler) ReconcileKind(ctx context.Context, b *eventingv1.Broker) pkgreconciler.Event {
	triggerChan, err := r.reconcileKind(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Errorw("Problem reconciling broker", zap.Error(err))
	}

	if b.Status.IsReady() {
		// So, at this point the Broker is ready and everything should be solid
		// for the triggers to act upon, so reconcile them.
		te := r.reconcileTriggers(ctx, b, triggerChan)
		if te != nil {
			logging.FromContext(ctx).Errorw("Problem reconciling triggers", zap.Error(te))
			return fmt.Errorf("failed to reconcile triggers: %v", te)
		}
	} else {
		// Broker is not ready, but propagate it's status to my triggers.
		if te := r.propagateBrokerStatusToTriggers(ctx, b.Namespace, b.Name, &b.Status); te != nil {
			return fmt.Errorf("Trigger reconcile failed: %v", te)
		}
	}
	return err
}

func (r *Reconciler) reconcileKind(ctx context.Context, b *eventingv1.Broker) (*corev1.ObjectReference, pkgreconciler.Event) {
	logging.FromContext(ctx).Infow("Reconciling", zap.Any("Broker", b))

	// 1. Trigger Channel is created for all events. Triggers will Subscribe to this Channel.
	// 2. Check that Filter / Ingress deployment (shared within cluster are there)
	chanMan, err := r.getChannelTemplate(ctx, b)
	if err != nil {
		b.Status.MarkTriggerChannelFailed("ChannelTemplateFailed", "Error on setting up the ChannelTemplate: %s", err)
		return nil, err
	}

	logging.FromContext(ctx).Infow("Reconciling the trigger channel")
	c, err := resources.NewChannel("trigger", b, &chanMan.template, TriggerChannelLabels(b.Name))
	if err != nil {
		logging.FromContext(ctx).Errorw(fmt.Sprintf("Failed to create Trigger Channel object: %s/%s", chanMan.ref.Namespace, chanMan.ref.Name), zap.Error(err))
		return nil, err
	}

	triggerChan, err := r.reconcileChannel(ctx, chanMan.inf, chanMan.ref, c, b)
	if err != nil {
		logging.FromContext(ctx).Errorw("Problem reconciling the trigger channel", zap.Error(err))
		b.Status.MarkTriggerChannelFailed("ChannelFailure", "%v", err)
		return nil, fmt.Errorf("Failed to reconcile trigger channel: %v", err)
	}

	if triggerChan.Status.Address == nil {
		logging.FromContext(ctx).Debugw("Trigger Channel does not have an address", zap.Any("triggerChan", triggerChan))
		b.Status.MarkTriggerChannelFailed("NoAddress", "Channel does not have an address.")
		// Ok to return nil for error here, once channel address becomes available, this will get requeued.
		return &chanMan.ref, nil
	}
	if url := triggerChan.Status.Address.URL; url.Host == "" {
		logging.FromContext(ctx).Debugw("Trigger Channel does not have an address", zap.Any("triggerChan", triggerChan))
		b.Status.MarkTriggerChannelFailed("NoAddress", "Channel does not have an address.")
		// Ok to return nil for error here, once channel address becomes available, this will get requeued.
		return &chanMan.ref, nil
	}

	channelStatus := &duckv1.ChannelableStatus{AddressStatus: pkgduckv1.AddressStatus{Address: &pkgduckv1.Addressable{URL: triggerChan.Status.Address.URL}}}
	b.Status.PropagateTriggerChannelReadiness(channelStatus)

	filterEndpoints, err := r.endpointsLister.Endpoints(system.Namespace()).Get(names.BrokerFilterName)
	if err != nil {
		logging.FromContext(ctx).Errorw("Problem getting endpoints for filter", zap.String("namespace", system.Namespace()), zap.Error(err))
		b.Status.MarkFilterFailed("ServiceFailure", "%v", err)
		return nil, err
	}
	b.Status.PropagateFilterAvailability(filterEndpoints)

	ingressEndpoints, err := r.endpointsLister.Endpoints(system.Namespace()).Get(names.BrokerIngressName)
	if err != nil {
		logging.FromContext(ctx).Errorw("Problem getting endpoints for ingress", zap.String("namespace", system.Namespace()), zap.Error(err))
		b.Status.MarkIngressFailed("ServiceFailure", "%v", err)
		return nil, err
	}
	b.Status.PropagateIngressAvailability(ingressEndpoints)

	// Route everything to shared ingress, just tack on the namespace/name as path
	// so we can route there appropriately.
	b.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName("broker-ingress", system.Namespace()),
		Path:   fmt.Sprintf("/%s/%s", b.Namespace, b.Name),
	})

	// So, at this point the Broker is ready and everything should be solid
	// for the triggers to act upon.
	return &chanMan.ref, nil
}

type channelTemplate struct {
	ref      corev1.ObjectReference
	inf      dynamic.ResourceInterface
	template messagingv1.ChannelTemplateSpec
}

func (r *Reconciler) getChannelTemplate(ctx context.Context, b *eventingv1.Broker) (*channelTemplate, error) {
	triggerChannelName := resources.BrokerChannelName(b.Name, "trigger")
	ref := corev1.ObjectReference{
		Name:      triggerChannelName,
		Namespace: b.Namespace,
	}
	var template *messagingv1.ChannelTemplateSpec

	if b.Spec.Config != nil {
		if b.Spec.Config.Kind == "ConfigMap" && b.Spec.Config.APIVersion == "v1" {
			if b.Spec.Config.Namespace == "" || b.Spec.Config.Name == "" {
				logging.FromContext(ctx).Errorw("Broker.Spec.Config name and namespace are required",
					zap.String("namespace", b.Namespace), zap.String("name", b.Name))
				return nil, errors.New("Broker.Spec.Config name and namespace are required")
			}
			cm, err := r.kubeClientSet.CoreV1().ConfigMaps(b.Spec.Config.Namespace).Get(b.Spec.Config.Name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			// TODO: there are better ways to do this...

			if config, err := NewConfigFromConfigMapFunc(ctx)(cm); err != nil {
				return nil, err
			} else if config != nil {
				template = &config.DefaultChannelTemplate
			}
			logging.FromContext(ctx).Info("Using channel template = ", template)
		} else {
			return nil, errors.New("Broker.Spec.Config configuration not supported, only [kind: ConfigMap, apiVersion: v1]")
		}
	}

	if template == nil {
		return nil, errors.New("failed to find channelTemplate")
	}
	ref.APIVersion = template.APIVersion
	ref.Kind = template.Kind

	gvr, _ := meta.UnsafeGuessKindToResource(template.GetObjectKind().GroupVersionKind())

	inf := r.dynamicClientSet.Resource(gvr).Namespace(b.Namespace)
	if inf == nil {
		return nil, fmt.Errorf("unable to create dynamic client for: %+v", template)
	}

	track := r.channelableTracker.TrackInNamespace(b)

	// Start tracking the trigger channel.
	if err := track(ref); err != nil {
		return nil, fmt.Errorf("unable to track changes to the trigger Channel: %v", err)
	}
	return &channelTemplate{
		ref:      ref,
		inf:      inf,
		template: *template,
	}, nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, b *eventingv1.Broker) pkgreconciler.Event {
	if err := r.propagateBrokerStatusToTriggers(ctx, b.Namespace, b.Name, nil); err != nil {
		return fmt.Errorf("Trigger reconcile failed: %v", err)
	}
	return newReconciledNormal(b.Namespace, b.Name)
}

// reconcileChannel reconciles Broker's 'b' underlying channel.
func (r *Reconciler) reconcileChannel(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, channelObjRef corev1.ObjectReference, newChannel *unstructured.Unstructured, b *eventingv1.Broker) (*duckv1.Channelable, error) {
	lister, err := r.channelableTracker.ListerFor(channelObjRef)
	if err != nil {
		logging.FromContext(ctx).Errorw(fmt.Sprintf("Error getting lister for Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Error(err))
		return nil, err
	}
	c, err := lister.ByNamespace(channelObjRef.Namespace).Get(channelObjRef.Name)
	// If the resource doesn't exist, we'll create it
	if err != nil {
		if apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Info(fmt.Sprintf("Creating Channel Object: %+v", newChannel))
			created, err := channelResourceInterface.Create(newChannel, metav1.CreateOptions{})
			if err != nil {
				logging.FromContext(ctx).Errorw(fmt.Sprintf("Failed to create Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Error(err))
				return nil, err
			}
			logging.FromContext(ctx).Info(fmt.Sprintf("Created Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Any("NewChannel", newChannel))
			channelable := &duckv1.Channelable{}
			err = duckapis.FromUnstructured(created, channelable)
			if err != nil {
				logging.FromContext(ctx).Errorw(fmt.Sprintf("Failed to convert to Channelable Object: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Any("createdChannel", created), zap.Error(err))
				return nil, err

			}
			return channelable, nil
		}
		logging.FromContext(ctx).Errorw(fmt.Sprintf("Failed to get Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Error(err))
		return nil, err
	}
	logging.FromContext(ctx).Debugw(fmt.Sprintf("Found Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name))
	channelable, ok := c.(*duckv1.Channelable)
	if !ok {
		logging.FromContext(ctx).Errorw(fmt.Sprintf("Failed to convert to Channelable Object: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Error(err))
		return nil, err
	}
	return channelable, nil
}

// TriggerChannelLabels are all the labels placed on the Trigger Channel for the given brokerName. This
// should only be used by Broker and Trigger code.
func TriggerChannelLabels(brokerName string) map[string]string {
	return map[string]string{
		eventing.BrokerLabelKey:                 brokerName,
		"eventing.knative.dev/brokerEverything": "true",
	}
}

// reconcileTriggers reconciles the Triggers that are pointed to this broker
func (r *Reconciler) reconcileTriggers(ctx context.Context, b *eventingv1.Broker, triggerChan *corev1.ObjectReference) error {
	recorder := controller.GetEventRecorder(ctx)
	triggers, err := r.triggerLister.Triggers(b.Namespace).List(labels.Everything())
	if err != nil {
		return err
	}
	for _, t := range triggers {
		if t.Spec.Broker == b.Name {
			trigger := t.DeepCopy()
			tErr := r.reconcileTrigger(ctx, b, trigger, triggerChan)
			if tErr != nil {
				logging.FromContext(ctx).Errorw("Reconciling trigger failed:", zap.String("name", t.Name), zap.Error(err))
				recorder.Eventf(trigger, corev1.EventTypeWarning, triggerReconcileFailed, "Trigger reconcile failed: %v", tErr)
			} else {
				recorder.Event(trigger, corev1.EventTypeNormal, triggerReconciled, "Trigger reconciled")
			}
			trigger.Status.ObservedGeneration = t.Generation
			if _, updateStatusErr := r.updateTriggerStatus(ctx, trigger); updateStatusErr != nil {
				logging.FromContext(ctx).Errorw("Failed to update Trigger status", zap.Error(updateStatusErr))
				recorder.Eventf(trigger, corev1.EventTypeWarning, triggerUpdateStatusFailed, "Failed to update Trigger's status: %v", updateStatusErr)
			}
		}
	}
	return nil
}

func (r *Reconciler) propagateBrokerStatusToTriggers(ctx context.Context, namespace, name string, bs *eventingv1.BrokerStatus) error {
	recorder := controller.GetEventRecorder(ctx)
	triggers, err := r.triggerLister.Triggers(namespace).List(labels.Everything())
	if err != nil {
		return err
	}
	for _, t := range triggers {
		if t.Spec.Broker == name {
			// Don't modify informers copy
			trigger := t.DeepCopy()
			trigger.Status.InitializeConditions()
			if bs == nil {
				trigger.Status.MarkBrokerFailed("BrokerDoesNotExist", "Broker %q does not exist", name)
			} else {
				trigger.Status.PropagateBrokerCondition(bs.GetTopLevelCondition())
			}
			if _, updateStatusErr := r.updateTriggerStatus(ctx, trigger); updateStatusErr != nil {
				logging.FromContext(ctx).Errorw("Failed to update Trigger status", zap.Error(updateStatusErr))
				recorder.Eventf(trigger, corev1.EventTypeWarning, triggerUpdateStatusFailed, "Failed to update Trigger's status: %v", updateStatusErr)
				return updateStatusErr
			}
		}
	}
	return nil
}

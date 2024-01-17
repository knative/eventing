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

package broker

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/resolver"

	"knative.dev/pkg/apis"
	duckapis "knative.dev/pkg/apis/duck"
	pkgduckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/network"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/feature"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/auth"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	brokerreconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/broker"
	messaginglisters "knative.dev/eventing/pkg/client/listers/messaging/v1"
	ducklib "knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/reconciler/broker/resources"
	"knative.dev/eventing/pkg/reconciler/names"
)

const (
	ingressServerTLSSecretName = "mt-broker-ingress-server-tls" //nolint:gosec // This is not a hardcoded credential
	caCertsSecretKey           = eventingtls.SecretCACert
)

type Reconciler struct {
	eventingClientSet clientset.Interface
	dynamicClientSet  dynamic.Interface

	// listers index properties about resources
	endpointsLister    corev1listers.EndpointsLister
	subscriptionLister messaginglisters.SubscriptionLister
	configmapLister    corev1listers.ConfigMapLister
	secretLister       corev1listers.SecretLister

	channelableTracker ducklib.ListableTracker

	uriResolver *resolver.URIResolver

	// If specified, only reconcile brokers with these labels
	brokerClass string
}

// Check that our Reconciler implements Interface
var _ brokerreconciler.Interface = (*Reconciler)(nil)

// ReconcilerArgs are the arguments needed to create a broker.Reconciler.
type ReconcilerArgs struct {
	IngressImage              string
	IngressServiceAccountName string
	FilterImage               string
	FilterServiceAccountName  string
}

func (r *Reconciler) ReconcileKind(ctx context.Context, b *eventingv1.Broker) pkgreconciler.Event {
	logging.FromContext(ctx).Infow("Reconciling", zap.Any("Broker", b))

	// 1. Trigger Channel is created for all events. Triggers will Subscribe to this Channel.
	// 2. Check that Filter / Ingress deployment (shared within cluster are there)
	chanMan, err := r.getChannelTemplate(ctx, b)
	if err != nil {
		b.Status.MarkTriggerChannelFailed("ChannelTemplateFailed", "Error on setting up the ChannelTemplate: %s", err)
		return err
	}

	var tmpChannelableSpec duckv1.ChannelableSpec = duckv1.ChannelableSpec{
		Delivery: b.Spec.Delivery,
	}

	logging.FromContext(ctx).Infow("Reconciling the trigger channel")
	c, err := ducklib.NewPhysicalChannel(
		chanMan.template.TypeMeta,
		metav1.ObjectMeta{
			Name:      resources.BrokerChannelName(b.Name, "trigger"),
			Namespace: b.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(b),
			},
			Labels:      TriggerChannelLabels(b.Name),
			Annotations: map[string]string{eventing.ScopeAnnotationKey: eventing.ScopeCluster},
		},
		ducklib.WithChannelableSpec(tmpChannelableSpec),
		ducklib.WithPhysicalChannelSpec(chanMan.template.Spec),
	)
	if err != nil {
		logging.FromContext(ctx).Errorw(fmt.Sprintf("Failed to create Trigger Channel object: %s/%s", chanMan.ref.Namespace, chanMan.ref.Name), zap.Error(err))
		return err
	}

	triggerChan, err := r.reconcileChannel(ctx, chanMan.inf, chanMan.ref, c)
	if err != nil {
		logging.FromContext(ctx).Errorw("Problem reconciling the trigger channel", zap.Error(err))
		b.Status.MarkTriggerChannelFailed("ChannelFailure", "%v", err)
		return fmt.Errorf("failed to reconcile trigger channel: %v", err)
	}

	if triggerChan.Status.Address == nil {
		logging.FromContext(ctx).Debugw("Trigger Channel does not have an address", zap.Any("triggerChan", triggerChan))
		b.Status.MarkTriggerChannelFailed("NoAddress", "Channel does not have an address.")
		// Ok to return nil for error here, once channel address becomes available, this will get requeued.
		return nil
	}

	if url := triggerChan.Status.Address.URL; url == nil || url.Host == "" {
		logging.FromContext(ctx).Debugw("Trigger Channel does not have an address", zap.Any("triggerChan", triggerChan))
		b.Status.MarkTriggerChannelFailed("NoAddress", "Channel does not have an address.")
		// Ok to return nil for error here, once channel address becomes available, this will get requeued.
		return nil
	}

	// Attach the channel address as a status annotation.
	if b.Status.Annotations == nil {
		b.Status.Annotations = make(map[string]string, 1)
	}
	b.Status.Annotations[eventing.BrokerChannelAddressStatusAnnotationKey] = triggerChan.Status.Address.URL.String()
	b.Status.Annotations[eventing.BrokerChannelKindStatusAnnotationKey] = chanMan.ref.Kind
	b.Status.Annotations[eventing.BrokerChannelAPIVersionStatusAnnotationKey] = chanMan.ref.APIVersion
	b.Status.Annotations[eventing.BrokerChannelNameStatusAnnotationKey] = chanMan.ref.Name

	if caCerts := triggerChan.Status.Address.CACerts; caCerts != nil && *caCerts != "" {
		b.Status.Annotations[eventing.BrokerChannelCACertsStatusAnnotationKey] = *caCerts
	}

	if audience := triggerChan.Status.Address.Audience; audience != nil && *audience != "" {
		b.Status.Annotations[eventing.BrokerChannelAudienceStatusAnnotationKey] = *audience
	}

	channelStatus := &duckv1.ChannelableStatus{
		AddressStatus:  triggerChan.Status.AddressStatus,
		DeliveryStatus: triggerChan.Status.DeliveryStatus,
	}

	b.Status.PropagateTriggerChannelReadiness(channelStatus)

	filterEndpoints, err := r.endpointsLister.Endpoints(system.Namespace()).Get(names.BrokerFilterName)
	if err != nil {
		logging.FromContext(ctx).Errorw("Problem getting endpoints for filter", zap.String("namespace", system.Namespace()), zap.Error(err))
		b.Status.MarkFilterFailed("ServiceFailure", "%v", err)
		return err
	}
	b.Status.PropagateFilterAvailability(filterEndpoints)

	ingressEndpoints, err := r.endpointsLister.Endpoints(system.Namespace()).Get(names.BrokerIngressName)
	if err != nil {
		logging.FromContext(ctx).Errorw("Problem getting endpoints for ingress", zap.String("namespace", system.Namespace()), zap.Error(err))
		b.Status.MarkIngressFailed("ServiceFailure", "%v", err)
		return err
	}
	b.Status.PropagateIngressAvailability(ingressEndpoints)

	if b.Spec.Delivery != nil && b.Spec.Delivery.DeadLetterSink != nil {
		deadLetterSinkAddr, err := r.uriResolver.AddressableFromDestinationV1(ctx, *b.Spec.Delivery.DeadLetterSink, b)
		logging.FromContext(ctx).Errorw("broker has deliver spec set. Will use it to mark dls status", zap.Any("dls-addr", deadLetterSinkAddr), zap.Any("broker.spec.delivery", b.Spec.Delivery))
		if err != nil {
			b.Status.DeliveryStatus = duckv1.DeliveryStatus{}
			logging.FromContext(ctx).Errorw("Unable to get the dead letter sink's URI", zap.Error(err))
			b.Status.MarkDeadLetterSinkResolvedFailed("Unable to get the dead letter sink's URI", "%v", err)
			return err
		}
		ds := duckv1.NewDeliveryStatusFromAddressable(deadLetterSinkAddr)
		b.Status.MarkDeadLetterSinkResolvedSucceeded(ds)
	} else {
		b.Status.MarkDeadLetterSinkNotConfigured()
	}

	// Route everything to shared ingress, just tack on the namespace/name as path
	// so we can route there appropriately.
	featureFlags := feature.FromContext(ctx)
	if featureFlags.IsPermissiveTransportEncryption() {
		caCerts, err := r.getCaCerts()
		if err != nil {
			return err
		}

		httpAddress := r.httpAddress(b)
		httpsAddress := r.httpsAddress(caCerts, b)
		// Permissive mode:
		// - status.address http address with path-based routing
		// - status.addresses:
		//   - https address with path-based routing
		//   - http address with path-based routing
		b.Status.Addresses = []pkgduckv1.Addressable{httpsAddress, httpAddress}
		b.Status.Address = &httpAddress
	} else if featureFlags.IsStrictTransportEncryption() {
		// Strict mode: (only https addresses)
		// - status.address https address with path-based routing
		// - status.addresses:
		//   - https address with path-based routing
		caCerts, err := r.getCaCerts()
		if err != nil {
			return err
		}

		httpsAddress := r.httpsAddress(caCerts, b)
		b.Status.Addresses = []pkgduckv1.Addressable{httpsAddress}
		b.Status.Address = &httpsAddress
	} else {
		httpAddress := r.httpAddress(b)
		b.Status.Address = &httpAddress
	}

	if featureFlags.IsOIDCAuthentication() {
		audience := auth.GetAudience(eventingv1.SchemeGroupVersion.WithKind("Broker"), b.ObjectMeta)
		logging.FromContext(ctx).Debugw("Setting the brokers audience", zap.String("audience", audience))
		b.Status.Address.Audience = &audience
		for i := range b.Status.Addresses {
			b.Status.Addresses[i].Audience = &audience
		}
	} else {
		logging.FromContext(ctx).Debug("Clearing the brokers audience as OIDC is not enabled")
		b.Status.Address.Audience = nil
		for i := range b.Status.Addresses {
			b.Status.Addresses[i].Audience = nil
		}
	}

	b.GetConditionSet().Manage(b.GetStatus()).MarkTrue(eventingv1.BrokerConditionAddressable)

	// So, at this point the Broker is ready and everything should be solid
	// for the triggers to act upon.
	return nil
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
		if b.Spec.Config.Kind == "ConfigMap" {
			if b.Spec.Config.Namespace == "" || b.Spec.Config.Name == "" {
				logging.FromContext(ctx).Errorw("Broker.Spec.Config name and namespace are required",
					zap.String("namespace", b.Namespace), zap.String("name", b.Name))
				return nil, errors.New("Broker.Spec.Config name and namespace are required")
			}

			cm, err := r.configmapLister.ConfigMaps(b.Spec.Config.Namespace).Get(b.Spec.Config.Name)
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

	track := r.channelableTracker.TrackInNamespace(ctx, b)

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

// reconcileChannel reconciles Broker's 'b' underlying channel.
func (r *Reconciler) reconcileChannel(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, channelObjRef corev1.ObjectReference, newChannel *unstructured.Unstructured) (*duckv1.Channelable, error) {
	lister, err := r.channelableTracker.ListerFor(channelObjRef)
	if err != nil {
		logging.FromContext(ctx).Errorw(fmt.Sprintf("Error getting lister for Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Error(err))
		return nil, err
	}

	c, err := lister.ByNamespace(channelObjRef.Namespace).Get(channelObjRef.Name)
	switch {
	case apierrs.IsNotFound(err):
		// Create the Channel if it doesn't exists.
		logging.FromContext(ctx).Info(fmt.Sprintf("Creating Channel Object: %+v", newChannel))
		created, err := channelResourceInterface.Create(ctx, newChannel, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create channel %s/%s: %w", channelObjRef.Namespace, channelObjRef.Name, err)
		}
		logging.FromContext(ctx).Debug(fmt.Sprintf("Created Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Any("NewPhysicalChannel", created))

		channelable := &duckv1.Channelable{}
		err = duckapis.FromUnstructured(created, channelable)
		if err != nil {
			return nil, fmt.Errorf("failed to convert to Channelable Object %s/%s: %w", channelObjRef.Namespace, channelObjRef.Name, err)
		}
		return channelable, nil
	case err != nil:
		logging.FromContext(ctx).Errorw(fmt.Sprintf("Failed to get Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Error(err))
		return nil, err

	}

	// Make sure the existing Channel options match those of
	// the Broker, if not Patch.

	logging.FromContext(ctx).Debugw(fmt.Sprintf("Found Channel: %s/%s", channelObjRef.Namespace, channelObjRef.Name))
	channelable, ok := c.(*duckv1.Channelable)
	if !ok {
		logging.FromContext(ctx).Errorw(fmt.Sprintf("Failed to convert to Channelable Object: %s/%s", channelObjRef.Namespace, channelObjRef.Name), zap.Error(err))
		return nil, err
	}

	// We are only interested in comparing mutable properties, all
	// others should not change. Any mutable property added to Channel based Brokers
	// that needs to be propagated to underlying Channels should be added
	// to this comparison.
	desired := &duckv1.Channelable{}
	err = duckapis.FromUnstructured(newChannel, desired)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %s/%s into Channelable: %w", channelObjRef.Namespace, channelObjRef.Name, err)
	}

	if equality.Semantic.DeepEqual(desired.Spec.Delivery, channelable.Spec.Delivery) {
		// If propagated/mutable properties match return the Channel.
		return channelable, nil
	}

	// Create a Patch by isolating desired and existing Channel mutable
	// properties.
	jsonPatch, err := duckapis.CreatePatch(
		// Existing Channel properties
		duckv1.Channelable{
			Spec: duckv1.ChannelableSpec{
				Delivery: channelable.Spec.Delivery,
			},
		},
		// Desired Channel properties
		duckv1.Channelable{
			Spec: duckv1.ChannelableSpec{
				Delivery: desired.Spec.Delivery,
			},
		})
	if err != nil {
		return nil, fmt.Errorf("creating JSON patch for Channelable Object %s/%s: %w",
			channelObjRef.Namespace, channelObjRef.Name, err)
	}
	if len(jsonPatch) == 0 {
		return nil, fmt.Errorf("unexpected empty JSON patch for %s/%s",
			channelObjRef.Namespace, channelObjRef.Name)
	}

	patch, err := jsonPatch.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshalling JSON patch for Channelable Object %s/%s: %w",
			channelObjRef.Namespace, channelObjRef.Name, err)
	}

	patched, err := channelResourceInterface.Patch(ctx, channelObjRef.Name, types.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("patching channeable object %s/%s: %w", channelObjRef.Namespace, channelObjRef.Name, err)
	}
	logging.FromContext(ctx).Info("Patched Channel", zap.Any("patched", patched))
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

func (r *Reconciler) getCaCerts() (*string, error) {
	secret, err := r.secretLister.Secrets(system.Namespace()).Get(ingressServerTLSSecretName)
	if err != nil {
		return nil, fmt.Errorf("failed to get CA certs from %s/%s: %w", system.Namespace(), ingressServerTLSSecretName, err)
	}
	caCerts, ok := secret.Data[caCertsSecretKey]
	if !ok {
		return nil, nil
	}
	return pointer.String(string(caCerts)), nil
}

func (r *Reconciler) httpAddress(b *eventingv1.Broker) pkgduckv1.Addressable {
	// http address uses path-based routing
	httpAddress := pkgduckv1.Addressable{
		Name: pointer.String("http"),
		URL:  apis.HTTP(network.GetServiceHostname(names.BrokerIngressName, system.Namespace())),
	}
	httpAddress.URL.Path = fmt.Sprintf("/%s/%s", b.Namespace, b.Name)
	return httpAddress
}

func (r *Reconciler) httpsAddress(caCerts *string, b *eventingv1.Broker) pkgduckv1.Addressable {
	// https address uses path-based routing
	httpsAddress := pkgduckv1.Addressable{
		Name:    pointer.String("https"),
		URL:     apis.HTTPS(fmt.Sprintf("%s.%s.svc.%s", names.BrokerIngressName, system.Namespace(), network.GetClusterDomainName())),
		CACerts: caCerts,
	}
	httpsAddress.URL.Path = fmt.Sprintf("/%s/%s", b.Namespace, b.Name)
	return httpsAddress
}

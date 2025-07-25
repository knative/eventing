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

package dispatcher

import (
	"context"
	"fmt"
	"strconv"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/feature"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	eventingv1beta3 "knative.dev/eventing/pkg/client/clientset/versioned/typed/eventing/v1beta3"
	messagingv1 "knative.dev/eventing/pkg/client/clientset/versioned/typed/messaging/v1"
	reconcilerv1 "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/inmemorychannel"
	"knative.dev/eventing/pkg/client/listers/eventing/v1beta3"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/eventtype"
	"knative.dev/eventing/pkg/kncloudevents"
)

// Reconciler reconciles InMemory Channels.
type Reconciler struct {
	multiChannelEventHandler multichannelfanout.MultiChannelEventHandler
	messagingClientSet       messagingv1.MessagingV1Interface
	eventTypeLister          v1beta3.EventTypeLister
	inMemoryChannelLister    listers.InMemoryChannelLister
	eventingClient           eventingv1beta3.EventingV1beta3Interface
	featureStore             *feature.Store
	eventDispatcher          *kncloudevents.Dispatcher

	authVerifier  *auth.Verifier
	clientConfig  eventingtls.ClientConfig
	meterProvider metric.MeterProvider
	traceProvider trace.TracerProvider
}

// Check the interfaces Reconciler should implement
var (
	_ reconcilerv1.Interface         = (*Reconciler)(nil)
	_ reconcilerv1.ReadOnlyInterface = (*Reconciler)(nil)
)

// ReconcileKind implements inmemorychannel.Interface.
func (r *Reconciler) ReconcileKind(ctx context.Context, imc *v1.InMemoryChannel) reconciler.Event {
	if err := r.reconcile(ctx, imc); err != nil {
		return err
	}

	// Then patch the subscribers to reflect that they are now ready to go
	return r.patchSubscriberStatus(ctx, imc)
}

// ObserveKind implements inmemorychannel.ReadOnlyInterface.
func (r *Reconciler) ObserveKind(ctx context.Context, imc *v1.InMemoryChannel) reconciler.Event {
	return r.reconcile(ctx, imc)
}

func (r *Reconciler) reconcile(ctx context.Context, imc *v1.InMemoryChannel) reconciler.Event {
	logging.FromContext(ctx).Infow("Reconciling", zap.Any("InMemoryChannel", imc))

	if !imc.IsReady() {
		logging.FromContext(ctx).Debug("IMC is not ready, skipping")
		return nil
	}

	config, err := newConfigForInMemoryChannel(ctx, imc)
	if err != nil {
		logging.FromContext(ctx).Error("Error creating config for in memory channels", zap.Error(err))
		return err
	}
	var eventTypeAutoHandler *eventtype.EventTypeAutoHandler
	var channelRef *duckv1.KReference
	var UID *types.UID
	if ownerReferences := imc.GetOwnerReferences(); r.featureStore.IsEnabled(feature.EvenTypeAutoCreate) &&
		(len(ownerReferences) == 0 ||
			ownerReferences[0].Kind != "Broker") {
		logging.FromContext(ctx).Info("EventType autocreate is enabled, creating handler")
		eventTypeAutoHandler = &eventtype.EventTypeAutoHandler{
			EventTypeLister: r.eventTypeLister,
			EventingClient:  r.eventingClient,
			FeatureStore:    r.featureStore,
			Logger:          logging.FromContext(ctx).Desugar(),
		}

		channelRef = toKReference(imc)
		UID = &imc.UID
	}

	wc := func(ctx context.Context) context.Context {
		return r.featureStore.ToContext(ctx)
	}

	// First grab the host based MultiChannelFanoutMessage httpHandler
	httpHandler := r.multiChannelEventHandler.GetChannelHandler(config.HostName)
	if httpHandler == nil {
		// No handler yet, create one.
		fanoutHandler, err := fanout.NewFanoutEventHandler(
			logging.FromContext(ctx).Desugar(),
			config.FanoutConfig,
			eventTypeAutoHandler,
			channelRef,
			UID,
			r.eventDispatcher,
			r.meterProvider,
			r.traceProvider,
			channel.OIDCTokenVerification(r.authVerifier, audience(imc)),
			channel.ReceiverWithContextFunc(wc),
			channel.ReceiverWithGetPoliciesForFunc(r.getAppliedEventPolicyRef),
		)
		if err != nil {
			logging.FromContext(ctx).Error("Failed to create a new fanout.EventHandler", err)
			return err
		}
		r.multiChannelEventHandler.SetChannelHandler(config.HostName, fanoutHandler)
	} else {
		// Just update the config if necessary.
		haveSubs := httpHandler.GetSubscriptions(ctx)

		// Ignore the closures, we stash the values that we can tell from if the values have actually changed.
		if diff := cmp.Diff(config.FanoutConfig.Subscriptions, haveSubs, cmpopts.IgnoreFields(kncloudevents.RetryConfig{}, "Backoff", "CheckRetry")); diff != "" {
			logging.FromContext(ctx).Info("Updating fanout config: ", zap.String("Diff", diff))
			httpHandler.SetSubscriptions(ctx, config.FanoutConfig.Subscriptions)
		}
	}

	// Look for an https handler that's configured to use paths
	httpsHandler := r.multiChannelEventHandler.GetChannelHandler(config.Path)
	if httpsHandler == nil {
		// No handler yet, create one.
		fanoutHandler, err := fanout.NewFanoutEventHandler(
			logging.FromContext(ctx).Desugar(),
			config.FanoutConfig,
			eventTypeAutoHandler,
			channelRef,
			UID,
			r.eventDispatcher,
			r.meterProvider,
			r.traceProvider,
			channel.ResolveChannelFromPath(channel.ParseChannelFromPath),
			channel.OIDCTokenVerification(r.authVerifier, audience(imc)),
			channel.ReceiverWithContextFunc(wc),
			channel.ReceiverWithGetPoliciesForFunc(r.getAppliedEventPolicyRef),
		)
		if err != nil {
			logging.FromContext(ctx).Error("Failed to create a new fanout.EventHandler", err)
			return err
		}
		r.multiChannelEventHandler.SetChannelHandler(config.Path, fanoutHandler)
	} else {
		// Just update the config if necessary.
		haveSubs := httpsHandler.GetSubscriptions(ctx)

		// Ignore the closures, we stash the values that we can tell from if the values have actually changed.
		if diff := cmp.Diff(config.FanoutConfig.Subscriptions, haveSubs, cmpopts.IgnoreFields(kncloudevents.RetryConfig{}, "Backoff", "CheckRetry")); diff != "" {
			logging.FromContext(ctx).Info("Updating fanout config: ", zap.String("Diff", diff))
			httpsHandler.SetSubscriptions(ctx, config.FanoutConfig.Subscriptions)
		}
	}

	handleSubscribers(imc.Spec.Subscribers, func(addressable duckv1.Addressable) {
		kncloudevents.AddOrUpdateAddressableHandler(r.clientConfig, addressable, r.meterProvider, r.traceProvider)
	})

	return nil
}

func (r *Reconciler) patchSubscriberStatus(ctx context.Context, imc *v1.InMemoryChannel) error {
	after := imc.DeepCopy()

	after.Status.Subscribers = make([]eventingduckv1.SubscriberStatus, 0)
	for _, sub := range imc.Spec.Subscribers {
		after.Status.Subscribers = append(after.Status.Subscribers, eventingduckv1.SubscriberStatus{
			UID:                sub.UID,
			ObservedGeneration: sub.Generation,
			Ready:              corev1.ConditionTrue,
		})
	}
	jsonPatch, err := duck.CreatePatch(imc, after)
	if err != nil {
		return fmt.Errorf("creating JSON patch: %w", err)
	}
	// If there is nothing to patch, we are good, just return.
	// Empty patch is [], hence we check for that.
	if len(jsonPatch) == 0 {
		return nil
	}

	patch, err := jsonPatch.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshaling JSON patch: %w", err)
	}
	patched, err := r.messagingClientSet.InMemoryChannels(imc.Namespace).Patch(ctx, imc.Name, types.JSONPatchType, patch, metav1.PatchOptions{}, "status")
	if err != nil {
		return fmt.Errorf("Failed patching: %w", err)
	}
	logging.FromContext(ctx).Debugw("Patched resource", zap.Any("patch", patch), zap.Any("patched", patched))
	return nil
}

// newConfigForInMemoryChannel creates a new Config for a single inmemory channel.
func newConfigForInMemoryChannel(ctx context.Context, imc *v1.InMemoryChannel) (*multichannelfanout.ChannelConfig, error) {
	featureFlags := feature.FromContext(ctx)
	isOIDCEnabled := featureFlags.IsOIDCAuthentication()
	subs := make([]fanout.Subscription, len(imc.Spec.Subscribers))

	for i, sub := range imc.Spec.Subscribers {
		conf, err := fanout.SubscriberSpecToFanoutConfig(sub)
		if err != nil {
			return nil, err
		}

		conf.Namespace = imc.Namespace
		if isOIDCEnabled && sub.Auth != nil && sub.Auth.ServiceAccountName != nil {
			conf.ServiceAccount = &types.NamespacedName{
				Name:      *sub.Auth.ServiceAccountName,
				Namespace: imc.Namespace,
			}
		}

		subs[i] = *conf
	}

	async := false
	if v, ok := imc.Annotations[v1.AsyncHandlerAnnotation]; ok {
		b, err := strconv.ParseBool(v)
		if err == nil {
			async = b
		}
	}

	return &multichannelfanout.ChannelConfig{
		Namespace: imc.Namespace,
		Name:      imc.Name,
		HostName:  imc.Status.Address.URL.Host,
		Path:      fmt.Sprintf("%s/%s", imc.Namespace, imc.Name),
		FanoutConfig: fanout.Config{
			AsyncHandler:  async,
			Subscriptions: subs,
		},
	}, nil
}

func (r *Reconciler) deleteFunc(obj interface{}) {
	if obj == nil {
		return
	}
	acc, err := kmeta.DeletionHandlingAccessor(obj)
	if err != nil {
		return
	}
	imc, ok := acc.(*v1.InMemoryChannel)
	if !ok || imc == nil {
		return
	}
	if imc.Status.Address != nil && imc.Status.Address.URL != nil {
		if hostName := imc.Status.Address.URL.Host; hostName != "" {
			r.multiChannelEventHandler.DeleteChannelHandler(hostName)
		}
	}

	handleSubscribers(imc.Spec.Subscribers, kncloudevents.DeleteAddressableHandler)
}

func (r *Reconciler) getAppliedEventPolicyRef(channel channel.ChannelReference) ([]eventingduckv1.AppliedEventPolicyRef, error) {
	imc, err := r.inMemoryChannelLister.InMemoryChannels(channel.Namespace).Get(channel.Name)
	if err != nil {
		return nil, fmt.Errorf("could not get inmemory channel %s/%s: %w", channel.Namespace, channel.Name, err)
	}

	return imc.Status.Policies, nil
}

func handleSubscribers(subscribers []eventingduckv1.SubscriberSpec, handle func(duckv1.Addressable)) {
	for _, sub := range subscribers {
		handle(duckv1.Addressable{
			URL:     sub.SubscriberURI,
			CACerts: sub.SubscriberCACerts,
		})

		if sub.ReplyURI != nil {
			handle(duckv1.Addressable{
				URL:     sub.ReplyURI,
				CACerts: sub.ReplyCACerts,
			})
		}

		if sub.Delivery != nil && sub.Delivery.DeadLetterSink != nil && sub.Delivery.DeadLetterSink.URI != nil {
			handle(duckv1.Addressable{
				URL:     sub.Delivery.DeadLetterSink.URI,
				CACerts: sub.Delivery.DeadLetterSink.CACerts,
			})
		}
	}
}

func toKReference(imc *v1.InMemoryChannel) *duckv1.KReference {
	return &duckv1.KReference{
		// Need to set Kind and APIVersion manually as the TypeMeta is not currently properly set https://github.com/knative/eventing/issues/7091
		// TODO: refactor this to use imc.Kind and imc.TypeMeta once #7091 is resolved
		Kind:       "InMemoryChannel",
		APIVersion: "messaging.knative.dev/v1",
		Namespace:  imc.Namespace,
		Name:       imc.Name,
		Address:    imc.Status.Address.Name,
	}
}

func audience(imc *v1.InMemoryChannel) string {
	return auth.GetAudience(v1.SchemeGroupVersion.WithKind("InMemoryChannel"), imc.ObjectMeta)
}

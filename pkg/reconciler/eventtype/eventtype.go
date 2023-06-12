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

package eventtype

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"

	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/tracker"

	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta2"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	eventtypereconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta2/eventtype"
	listersv1 "knative.dev/eventing/pkg/client/listers/eventing/v1"
	listersv1beta2 "knative.dev/eventing/pkg/client/listers/eventing/v1beta2"
	messaginglistersv1 "knative.dev/eventing/pkg/client/listers/messaging/v1"
)

type Reconciler struct {
	// listers index properties about resources
	eventTypeLister listersv1beta2.EventTypeLister
	channelLister   messaginglistersv1.ChannelLister
	brokerLister    listersv1.BrokerLister
	tracker         tracker.Interface
}

var brokerGVK = v1.SchemeGroupVersion.WithKind("Broker")
var ChannelGVK = messagingv1.SchemeGroupVersion.WithKind("Channel")

// Check that our Reconciler implements interface
var _ eventtypereconciler.Interface = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
// 1. Verify the Broker/Reference exists.
// 2. Verify the Broker/Reference is ready.
func (r *Reconciler) ReconcileKind(ctx context.Context, et *v1beta2.EventType) pkgreconciler.Event {
	switch et.Spec.Reference.Kind {
	case "Broker":
		return r.reconcileBrokerReference(ctx, et)
	case "Channel":
		return r.reconcileChannelReference(ctx, et)
	default:
		return fmt.Errorf("Unknown reference type")
	}
}

func (r *Reconciler) reconcileBrokerReference(ctx context.Context, et *v1beta2.EventType) pkgreconciler.Event {
	b, err := r.getBrokerReference(et)
	if err != nil {
		if apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Errorw("Broker reference does not exist", zap.Error(err))
			et.Status.MarkBrokerDoesNotExist()
		} else {
			logging.FromContext(ctx).Errorw("Unable to get the Broker reference", zap.Error(err))
			et.Status.MarkBrokerExistsUnknown("BrokerGetFailed", "Failed to get broker reference: %v", err)
		}
		return err
	}
	et.Status.MarkBrokerExists()

	apiVersion, kind := brokerGVK.ToAPIVersionAndKind()
	ref := tracker.Reference{
		APIVersion: apiVersion,
		Kind:       kind,
		Namespace:  b.Namespace,
		Name:       b.Name,
	}
	// Tell tracker to reconcile this EventType whenever the Broker changes.
	if err = r.tracker.TrackReference(ref, et); err != nil {
		logging.FromContext(ctx).Errorw("Unable to track changes to Broker reference", zap.Error(err))
		return err
	}

	et.Status.PropagateBrokerStatus(&b.Status)

	return nil
}

func (r *Reconciler) reconcileChannelReference(ctx context.Context, et *v1beta2.EventType) pkgreconciler.Event {
	c, err := r.getChannelReference(et)
	if err != nil {
		if apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Errorw("Channel reference does not exist", zap.Error(err))
			et.Status.MarkBrokerDoesNotExist()
		} else {
			logging.FromContext(ctx).Errorw("Unable to get the Channel reference", zap.Error(err))
			et.Status.MarkBrokerExistsUnknown("ChannelGetFailed", "Failed to get channel reference: %v", err)
		}
		return err
	}
	et.Status.MarkChannelExists()

	apiVersion, kind := ChannelGVK.ToAPIVersionAndKind()
	ref := tracker.Reference{
		APIVersion: apiVersion,
		Kind:       kind,
		Namespace:  c.Namespace,
		Name:       c.Name,
	}
	// Tell tracker to reconcile this EventType whenever the Broker changes.
	if err = r.tracker.TrackReference(ref, et); err != nil {
		logging.FromContext(ctx).Errorw("Unable to track changes to Channel reference", zap.Error(err))
		return err
	}

	et.Status.PropagateChannelStatus(&c.Status)

	return nil
}

// getBrokerReference returns the Broker for EventType 'et' if it exists, otherwise it returns an error.
func (r *Reconciler) getBrokerReference(et *v1beta2.EventType) (*v1.Broker, error) {
	return r.brokerLister.Brokers(et.Namespace).Get(et.Spec.Reference.Name)
}

// getChannelReference returns the Channel for the EventType 'et' if it exists, otherwise it returns an error.
func (r *Reconciler) getChannelReference(et *v1beta2.EventType) (*messagingv1.Channel, error) {
	return r.channelLister.Channels(et.Namespace).Get(et.Spec.Reference.Name)
}

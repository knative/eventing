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

	"go.uber.org/zap"
	apierrs "k8s.io/apimachinery/pkg/api/errors"

	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/tracker"

	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	eventtypereconciler "knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1beta1/eventtype"
	listersv1 "knative.dev/eventing/pkg/client/listers/eventing/v1"
	listersv1beta1 "knative.dev/eventing/pkg/client/listers/eventing/v1beta1"
)

type Reconciler struct {
	// listers index properties about resources
	eventTypeLister listersv1beta1.EventTypeLister
	brokerLister    listersv1.BrokerLister
	tracker         tracker.Interface
}

var brokerGVK = v1.SchemeGroupVersion.WithKind("Broker")

// Check that our Reconciler implements interface
var _ eventtypereconciler.Interface = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
// 1. Verify the Broker exists.
// 2. Verify the Broker is ready.
// TODO remove https://github.com/knative/eventing/issues/2750
func (r *Reconciler) ReconcileKind(ctx context.Context, et *v1beta1.EventType) pkgreconciler.Event {
	b, err := r.getBroker(et)
	if err != nil {
		if apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Errorw("Broker does not exist", zap.Error(err))
			et.Status.MarkBrokerDoesNotExist()
		} else {
			logging.FromContext(ctx).Errorw("Unable to get the Broker", zap.Error(err))
			et.Status.MarkBrokerExistsUnknown("BrokerGetFailed", "Failed to get broker: %v", err)
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
		logging.FromContext(ctx).Errorw("Unable to track changes to Broker", zap.Error(err))
		return err
	}

	et.Status.PropagateBrokerStatus(&b.Status)

	return nil
}

// getBroker returns the Broker for EventType 'et' if it exists, otherwise it returns an error.
func (r *Reconciler) getBroker(et *v1beta1.EventType) (*v1.Broker, error) {
	return r.brokerLister.Brokers(et.Namespace).Get(et.Spec.Broker)
}

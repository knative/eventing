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

package channel

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/golang/glog"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName       = "channel-default-controller"
	channelReconciled         = "ChannelReconciled"
	channelUpdateStatusFailed = "ChannelUpdateStatusFailed"
)

type reconciler struct {
	client        client.Client
	restConfig    *rest.Config
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a Channel controller.
// This Channel controller is a default controller for channels of all provisioner kinds
func ProvideController(mgr manager.Manager) (controller.Controller, error) {
	// Setup a new controller to Reconcile channel
	c, err := controller.New(controllerAgentName, mgr, controller.Options{
		Reconciler: &reconciler{
			recorder: mgr.GetRecorder(controllerAgentName),
		},
	})
	if err != nil {
		return nil, err
	}

	// Watch channel events
	// This controller is no-op when Channels are deleted
	if err := c.Watch(
		&source.Kind{Type: &v1alpha1.Channel{}},
		&handler.EnqueueRequestForObject{},
		predicate.Funcs{
			DeleteFunc: func(event.DeleteEvent) bool {
				return false
			},
		}); err != nil {
		return nil, err
	}

	return c, nil
}

// Reconcile will check if the channel is being watched by provisioner's channel controller
// This will improve UX. See https://github.com/knative/eventing/issues/779
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	glog.Infof("Reconciling channel %s", request)
	ch := &v1alpha1.Channel{}

	// Controller-runtime client Get() always deep copies the object. Hence no need to again deep copy it
	err := r.client.Get(context.TODO(), request.NamespacedName, ch)

	if errors.IsNotFound(err) {
		glog.Errorf("could not find channel %s\n", request)
		return reconcile.Result{}, nil
	}

	if err != nil {
		glog.Errorf("could not fetch channel %s: %s\n", request, err)
		return reconcile.Result{}, err
	}

	err = r.reconcile(ch)

	if err != nil {
		glog.Warningf("Error reconciling channel %s: %s. Will retry.", request, err)
		r.recorder.Eventf(ch, corev1.EventTypeWarning, channelUpdateStatusFailed, "Failed to update channel status: %s", request)
		return reconcile.Result{Requeue: true}, err
	}
	glog.Infof("Successfully reconciled channel %s", request)
	r.recorder.Eventf(ch, corev1.EventTypeNormal, channelReconciled, "Channel reconciled: %s", request)
	return reconcile.Result{Requeue: false}, nil
}

func (r *reconciler) reconcile(ch *v1alpha1.Channel) error {
	// Do not Initialize() Status in channel-default-controller. It will set ChannelConditionProvisionerInstalled=True
	// Directly call GetCondition(). If the Status was never initialized then GetCondition() will return nil and
	// IsUnknown() will return true
	c := ch.Status.GetCondition(v1alpha1.ChannelConditionProvisionerInstalled)

	if c.IsUnknown() {
		ch.Status.MarkProvisionerNotInstalled(
			"Provisioner not found.",
			"Specified provisioner [Name:%s Kind:%s] is not installed or not controlling the channel.",
			ch.Spec.Provisioner.Name,
			ch.Spec.Provisioner.Kind,
		)
		err := r.client.Status().Update(context.TODO(), ch)
		return err
	}
	return nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

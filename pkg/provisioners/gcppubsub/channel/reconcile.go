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

package channel

import (
	"context"

	eventduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/controller"
	util "github.com/knative/eventing/pkg/provisioners"
	ccpcontroller "github.com/knative/eventing/pkg/provisioners/gcppubsub/clusterchannelprovisioner"
	pubsubutil "github.com/knative/eventing/pkg/provisioners/gcppubsub/util"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	finalizerName = controllerAgentName
)

type reconciler struct {
	client   client.Client
	recorder record.EventRecorder
	logger   *zap.Logger

	pubSubClientCreator pubsubutil.PubSubClientCreator

	defaultGcpProject string
	defaultSecret     v1.ObjectReference
	defaultSecretKey  string
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// TODO: use this to store the logger and set a deadline
	ctx := context.TODO()
	logger := r.logger.With(zap.Any("request", request))

	c := &eventingv1alpha1.Channel{}
	err := r.client.Get(ctx, request.NamespacedName, c)

	// The Channel may have been deleted since it was added to the workqueue. If so, there is
	// nothing to be done.
	if errors.IsNotFound(err) {
		logger.Info("Could not find Channel", zap.Error(err))
		return reconcile.Result{}, nil
	}

	// Any other error should be retried in another reconciliation.
	if err != nil {
		logger.Error("Unable to Get Channel", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Does this Controller control this Channel?
	if !r.shouldReconcile(c) {
		logger.Info("Not reconciling Channel, it is not controlled by this Controller", zap.Any("ref", c.Spec))
		return reconcile.Result{}, nil
	}
	logger.Info("Reconciling Channel")

	// Modify a copy, not the original.
	c = c.DeepCopy()

	err = r.reconcile(ctx, c)
	if err != nil {
		logger.Info("Error reconciling Channel", zap.Error(err))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	}

	if updateStatusErr := util.UpdateChannel(ctx, r.client, c); updateStatusErr != nil {
		logger.Info("Error updating Channel Status", zap.Error(updateStatusErr))
		return reconcile.Result{}, updateStatusErr
	}

	return reconcile.Result{}, err
}

// shouldReconcile determines if this Controller should control (and therefore reconcile) a given
// ClusterChannelProvisioner. This Controller only handles in-memory channels.
func (r *reconciler) shouldReconcile(c *eventingv1alpha1.Channel) bool {
	if c.Spec.Provisioner != nil {
		return ccpcontroller.IsControlled(c.Spec.Provisioner)
	}
	return false
}

func (r *reconciler) reconcile(ctx context.Context, c *eventingv1alpha1.Channel) error {
	ctx = logging.WithLogger(ctx, r.logger.With(zap.Any("channel", c)).Sugar())

	c.Status.InitializeConditions()

	// We are syncing four things:
	// 1. The K8s Service to talk to this Channel.
	// 2. The Istio VirtualService to talk to this Channel.
	// 3. The GCP PubSub Topic.
	// 4. The GCP PubSub Subscriptions.

	// Regardless of what we are going to do, we need GCP credentials to do it.
	gcpCreds, err := pubsubutil.GetCredentials(ctx, r.client, r.defaultSecret, r.defaultSecretKey)
	if err != nil {
		logging.FromContext(ctx).Info("Unable to generate GCP creds", zap.Error(err))
		return err
	}

	if c.DeletionTimestamp != nil {
		// K8s garbage collection will delete the K8s service and VirtualService for this channel.
		// We use a finalizer to ensure the GCP PubSub Topic and Subscriptions are deleted.
		err = r.deleteSubscriptions(ctx, c, gcpCreds, r.defaultGcpProject)
		if err != nil {
			return err
		}
		err = r.deleteTopic(ctx, c, gcpCreds, r.defaultGcpProject)
		if err != nil {
			logging.FromContext(ctx).Info("Unable to delete topic", zap.Error(err))
			return err
		}
		util.RemoveFinalizer(c, finalizerName)
		return nil
	}

	util.AddFinalizer(c, finalizerName)

	err = r.createK8sService(ctx, c)
	if err != nil {
		return err
	}

	err = r.createVirtualService(ctx, c)
	if err != nil {
		return err
	}

	topic, err := r.createTopic(ctx, c, gcpCreds, r.defaultGcpProject)
	if err != nil {
		return err
	}

	err = r.createSubscriptions(ctx, c, gcpCreds, r.defaultGcpProject, topic)
	if err != nil {
		return err
	}

	c.Status.MarkProvisioned()
	return nil
}

func (r *reconciler) createK8sService(ctx context.Context, c *eventingv1alpha1.Channel) error {
	svc, err := util.CreateK8sService(ctx, r.client, c)
	if err != nil {
		logging.FromContext(ctx).Info("Error creating the Channel's K8s Service", zap.Error(err))
		return err
	}

	// Check if this Channel is the owner of the K8s service.
	if !metav1.IsControlledBy(svc, c) {
		logging.FromContext(ctx).Warn("Channel's K8s Service is not owned by the Channel", zap.Any("channel", c), zap.Any("service", svc))
	}

	c.Status.SetAddress(controller.ServiceHostName(svc.Name, svc.Namespace))
	return nil
}

func (r *reconciler) createVirtualService(ctx context.Context, c *eventingv1alpha1.Channel) error {
	virtualService, err := util.CreateVirtualService(ctx, r.client, c)
	if err != nil {
		logging.FromContext(ctx).Info("Error creating the Virtual Service for the Channel", zap.Error(err))
		return err
	}

	// If the Virtual Service is not controlled by this Channel, we should log a warning, but don't
	// consider it an error.
	if !metav1.IsControlledBy(virtualService, c) {
		logging.FromContext(ctx).Warn("VirtualService not owned by Channel", zap.Any("channel", c), zap.Any("virtualService", virtualService))
	}
	return nil
}

func (r *reconciler) createTopic(ctx context.Context, c *eventingv1alpha1.Channel, gcpCreds *google.Credentials, gcpProject string) (pubsubutil.PubSubTopic, error) {
	psc, err := r.pubSubClientCreator(ctx, gcpCreds, gcpProject)
	if err != nil {
		return nil, err
	}
	topic := psc.Topic(pubsubutil.GenerateTopicName(c.Namespace, c.Name))
	exists, err := topic.Exists(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return topic, nil
	}

	return psc.CreateTopic(ctx, topic.ID())
}

func (r *reconciler) deleteTopic(ctx context.Context, c *eventingv1alpha1.Channel, gcpCreds *google.Credentials, gcpProject string) error {
	psc, err := r.pubSubClientCreator(ctx, gcpCreds, gcpProject)
	if err != nil {
		logging.FromContext(ctx).Info("Unable to create PubSubClient", zap.Error(err))
		return err
	}
	topic := psc.Topic(pubsubutil.GenerateTopicName(c.Namespace, c.Name))
	exists, err := topic.Exists(ctx)
	if err != nil {
		logging.FromContext(ctx).Info("Unable to check if Topic exists", zap.Error(err))
		return err
	}
	if !exists {
		return nil
	}
	err = topic.Delete(ctx)
	if err != nil {
		logging.FromContext(ctx).Info("Topic deletion failed", zap.Error(err))
		return err
	}
	return nil
}

func (r *reconciler) createSubscriptions(ctx context.Context, c *eventingv1alpha1.Channel, gcpCreds *google.Credentials, gcpProject string, topic pubsubutil.PubSubTopic) error {
	if c.Spec.Subscribable != nil {
		for _, sub := range c.Spec.Subscribable.Subscribers {
			_, err := r.createSubscription(ctx, gcpCreds, gcpProject, topic, &sub)
			if err != nil {
				logging.FromContext(ctx).Info("Unable to create subscribers", zap.Error(err), zap.Any("channelSubscriber", sub))
				return err
			}
		}
	}
	return nil
}

func (r *reconciler) createSubscription(ctx context.Context, gcpCreds *google.Credentials, gcpProject string, topic pubsubutil.PubSubTopic, cs *eventduck.ChannelSubscriberSpec) (pubsubutil.PubSubSubscription, error) {
	psc, err := r.pubSubClientCreator(ctx, gcpCreds, gcpProject)
	if err != nil {
		return nil, err
	}
	sub := psc.SubscriptionInProject(pubsubutil.GenerateSubName(cs), gcpProject)
	if exists, err := sub.Exists(ctx); err != nil {
		return nil, err
	} else if exists {
		logging.FromContext(ctx).Info("Reusing existing subscription.")
		return sub, nil
	}

	createdSub, err := psc.CreateSubscription(ctx, sub.ID(), topic)
	if err != nil {
		logging.FromContext(ctx).Info("Error creating new subscription", zap.Error(err))
	} else {
		logging.FromContext(ctx).Info("Created new subscription", zap.Any("subscription", createdSub))
	}
	return createdSub, err
}

func (r *reconciler) deleteSubscriptions(ctx context.Context, c *eventingv1alpha1.Channel, gcpCreds *google.Credentials, gcpProject string) error {
	if c.Spec.Subscribable != nil {
		for _, sub := range c.Spec.Subscribable.Subscribers {
			err := r.deleteSubscription(ctx, gcpCreds, gcpProject, &sub)
			if err != nil {
				logging.FromContext(ctx).Info("Unable to create subscribers", zap.Error(err), zap.Any("channelSubscriber", sub))
				return err
			}
		}
	}
	return nil
}

func (r *reconciler) deleteSubscription(ctx context.Context, gcpCreds *google.Credentials, gcpProject string, cs *eventduck.ChannelSubscriberSpec) error {
	psc, err := r.pubSubClientCreator(ctx, gcpCreds, gcpProject)
	if err != nil {
		return err
	}
	sub := psc.SubscriptionInProject(pubsubutil.GenerateSubName(cs), gcpProject)
	if exists, err := sub.Exists(ctx); err != nil {
		return err
	} else if !exists {
		return nil
	}
	return sub.Delete(ctx)
}

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
	ccpcontroller "github.com/knative/eventing/pkg/provisioners/gcppubsub/controller/clusterchannelprovisioner"
	pubsubutil "github.com/knative/eventing/pkg/provisioners/gcppubsub/util"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	finalizerName = controllerAgentName
)

// reconciler reconciles GCP-PubSub Channels by creating the K8s Service and Istio VirtualService
// allowing other processes to send data to them. It also creates the GCP PubSub Topics (one per
// Channel) and GCP PubSub Subscriptions (one per Subscriber).
type reconciler struct {
	client   client.Client
	recorder record.EventRecorder
	logger   *zap.Logger

	pubSubClientCreator pubsubutil.PubSubClientCreator

	// Note that for all the default* parameters below, these must be kept in lock-step with the
	// GCP PubSub Dispatcher's reconciler.
	// Eventually, individual Channels should be allowed to specify different projects and secrets,
	// but for now all Channels use the same project and secret.

	// defaultGcpProject is the GCP project ID where PubSub Topics and Subscriptions are created.
	defaultGcpProject string
	// defaultSecret and defaultSecretKey are the K8s Secret and key in that secret that contain a
	// JSON format GCP service account token, see
	// https://cloud.google.com/iam/docs/creating-managing-service-account-keys#iam-service-account-keys-create-gcloud
	defaultSecret    *v1.ObjectReference
	defaultSecretKey string
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	ctx = logging.WithLogger(ctx, r.logger.With(zap.Any("request", request)).Sugar())

	c := &eventingv1alpha1.Channel{}
	err := r.client.Get(ctx, request.NamespacedName, c)

	// The Channel may have been deleted since it was added to the workqueue. If so, there is
	// nothing to be done.
	if errors.IsNotFound(err) {
		logging.FromContext(ctx).Info("Could not find Channel", zap.Error(err))
		return reconcile.Result{}, nil
	}

	// Any other error should be retried in another reconciliation.
	if err != nil {
		logging.FromContext(ctx).Error("Unable to Get Channel", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Does this Controller control this Channel?
	if !r.shouldReconcile(c) {
		logging.FromContext(ctx).Info("Not reconciling Channel, it is not controlled by this Controller", zap.Any("ref", c.Spec))
		return reconcile.Result{}, nil
	}
	logging.FromContext(ctx).Info("Reconciling Channel")

	// Modify a copy, not the original.
	c = c.DeepCopy()

	ctx = logging.WithLogger(ctx, logging.FromContext(ctx).With(zap.Any("channel", c)))
	requeue, reconcileErr := r.reconcile(ctx, c)
	if reconcileErr != nil {
		logging.FromContext(ctx).Info("Error reconciling Channel", zap.Error(reconcileErr))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	}

	if err = util.UpdateChannel(ctx, r.client, c); err != nil {
		logging.FromContext(ctx).Info("Error updating Channel Status", zap.Error(err))
		return reconcile.Result{}, err
	}

	return reconcile.Result{
		Requeue: requeue,
	}, reconcileErr
}

// shouldReconcile determines if this Controller should control (and therefore reconcile) a given
// Channel. This Controller only handles gcp-pubsub channels.
func (r *reconciler) shouldReconcile(c *eventingv1alpha1.Channel) bool {
	if c.Spec.Provisioner != nil {
		return ccpcontroller.IsControlled(c.Spec.Provisioner)
	}
	return false
}

// reconcile reconciles this Channel so that the real world matches the intended state. The returned
// boolean indicates if this Channel should be immediately requeued for another reconcile loop. The
// returned error indicates an error during reconciliation.
func (r *reconciler) reconcile(ctx context.Context, c *eventingv1alpha1.Channel) (bool, error) {
	c.Status.InitializeConditions()

	// We are syncing four things:
	// 1. The K8s Service to talk to this Channel.
	// 2. The Istio VirtualService to talk to this Channel.
	// 3. The GCP PubSub Topic (one for the Channel).
	// 4. The GCP PubSub Subscriptions (one for each Subscriber of the Channel).

	// Regardless of what we are going to do, we need GCP credentials to do it.
	gcpCreds, err := pubsubutil.GetCredentials(ctx, r.client, r.defaultSecret, r.defaultSecretKey)
	if err != nil {
		logging.FromContext(ctx).Info("Unable to generate GCP creds", zap.Error(err))
		return false, err
	}

	if c.DeletionTimestamp != nil {
		// K8s garbage collection will delete the K8s service and VirtualService for this channel.
		err = r.deleteSubscriptions(ctx, c, gcpCreds, r.defaultGcpProject)
		if err != nil {
			return false, err
		}
		err = r.deleteTopic(ctx, c, gcpCreds, r.defaultGcpProject)
		if err != nil {
			return false, err
		}
		util.RemoveFinalizer(c, finalizerName)
		return false, nil
	}

	// If we are adding the finalizer for the first time, then ensure that finalizer is persisted
	// before manipulating GCP PubSub, which will not be automatically garbage collected by K8s if
	// this Channel is deleted.
	if addFinalizerResult := util.AddFinalizer(c, finalizerName); addFinalizerResult == util.FinalizerAdded {
		return true, nil
	}

	svc, err := r.createK8sService(ctx, c)
	if err != nil {
		return false, err
	}

	err = r.createVirtualService(ctx, c, svc)
	if err != nil {
		return false, err
	}

	topic, err := r.createTopic(ctx, c, gcpCreds, r.defaultGcpProject)
	if err != nil {
		return false, err
	}

	err = r.createSubscriptions(ctx, c, gcpCreds, r.defaultGcpProject, topic)
	if err != nil {
		return false, err
	}

	c.Status.MarkProvisioned()
	return false, nil
}

func (r *reconciler) createK8sService(ctx context.Context, c *eventingv1alpha1.Channel) (*v1.Service, error) {
	svc, err := util.CreateK8sService(ctx, r.client, c)
	if err != nil {
		logging.FromContext(ctx).Info("Error creating the Channel's K8s Service", zap.Error(err))
		return nil, err
	}

	c.Status.SetAddress(controller.ServiceHostName(svc.Name, svc.Namespace))
	return svc, nil
}

func (r *reconciler) createVirtualService(ctx context.Context, c *eventingv1alpha1.Channel, svc *v1.Service) error {
	_, err := util.CreateVirtualService(ctx, r.client, c, svc)
	if err != nil {
		logging.FromContext(ctx).Info("Error creating the Virtual Service for the Channel", zap.Error(err))
		return err
	}
	return nil
}

func (r *reconciler) createTopic(ctx context.Context, c *eventingv1alpha1.Channel, gcpCreds *google.Credentials, gcpProject string) (pubsubutil.PubSubTopic, error) {
	psc, err := r.pubSubClientCreator(ctx, gcpCreds, gcpProject)
	if err != nil {
		logging.FromContext(ctx).Info("Unable to create PubSub client", zap.Error(err))
		return nil, err
	}
	topic := psc.Topic(pubsubutil.GenerateTopicName(c.Namespace, c.Name))
	exists, err := topic.Exists(ctx)
	if err != nil {
		logging.FromContext(ctx).Info("Unable to check Topic existence", zap.Error(err))
		return nil, err
	}
	if exists {
		return topic, nil
	}

	createdTopic, err := psc.CreateTopic(ctx, topic.ID())
	if err != nil {
		logging.FromContext(ctx).Info("Unable to create topic", zap.Error(err))
		return nil, err
	}
	return createdTopic, nil
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
		logging.FromContext(ctx).Debug("Topic did not exist")
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
	exists, err := sub.Exists(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		logging.FromContext(ctx).Debug("Reusing existing subscription.")
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
	exists, err := sub.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return sub.Delete(ctx)
}

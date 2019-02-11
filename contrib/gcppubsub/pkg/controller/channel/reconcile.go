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
	"fmt"

	ccpcontroller "github.com/knative/eventing/contrib/gcppubsub/pkg/controller/clusterchannelprovisioner"
	pubsubutil "github.com/knative/eventing/contrib/gcppubsub/pkg/util"
	"github.com/knative/eventing/contrib/gcppubsub/pkg/util/logging"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	util "github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/eventing/pkg/reconciler/names"
	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	finalizerName = controllerAgentName
)

type persistence int

const (
	persistStatus persistence = iota
	noNeedToPersist

	// Name of the corev1.Events emitted from the reconciliation process
	channelReconciled          = "ChannelReconciled"
	channelUpdateStatusFailed  = "ChannelUpdateStatusFailed"
	channelReadStatusFailed    = "ChannelReadStatusFailed"
	gcpCredentialsReadFailed   = "GcpCredentialsReadFailed"
	gcpResourcesPlanFailed     = "GcpResourcesPlanFailed"
	gcpResourcesPersistFailed  = "GcpResourcesPersistFailed"
	virtualServiceCreateFailed = "VirtualServiceCreateFailed"
	k8sServiceCreateFailed     = "K8sServiceCreateFailed"
	topicCreateFailed          = "TopicCreateFailed"
	topicDeleteFailed          = "TopicDeleteFailed"
	subscriptionSyncFailed     = "SubscriptionSyncFailed"
	subscriptionDeleteFailed   = "SubscriptionDeleteFailed"
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
	ctx = logging.WithLogger(ctx, r.logger.With(zap.Any("request", request)))

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
	} else {
		logging.FromContext(ctx).Info("Channel reconciled")
		r.recorder.Eventf(c, v1.EventTypeNormal, channelReconciled, "Channel reconciled: %q", c.Name)
	}

	if err = util.UpdateChannel(ctx, r.client, c); err != nil {
		logging.FromContext(ctx).Info("Error updating Channel Status", zap.Error(err))
		r.recorder.Eventf(c, v1.EventTypeWarning, channelUpdateStatusFailed, "Failed to update Channel's status: %v", err)
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

	// First we will plan all the names out for steps 3 and 4 persist them to status.internal. Then, on a
	// subsequent reconcile, we manipulate all the GCP resources in steps 3 and 4.

	originalPCS, err := pubsubutil.GetInternalStatus(c)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to read the status.internal", zap.Error(err))
		r.recorder.Eventf(c, v1.EventTypeWarning, channelReadStatusFailed, "Failed to read Channel's status.internal: %v", err)
		return false, err
	}

	// Regardless of what we are going to do, we need GCP credentials to do it.
	gcpCreds, err := pubsubutil.GetCredentials(ctx, r.client, r.defaultSecret, r.defaultSecretKey)
	if err != nil {
		logging.FromContext(ctx).Info("Unable to generate GCP creds", zap.Error(err))
		r.recorder.Eventf(c, v1.EventTypeWarning, gcpCredentialsReadFailed, "Failed to generate GCP credentials: %v", err)
		return false, err
	}

	if c.DeletionTimestamp != nil {
		// K8s garbage collection will delete the K8s service and VirtualService for this channel.
		// All the subs should be deleted.
		subsToSync := &syncSubs{
			subsToDelete: originalPCS.Subscriptions,
		}
		// Topic is nil because it is only used for sub creation, not deletion.
		err = r.syncSubscriptions(ctx, originalPCS, gcpCreds, nil, subsToSync)
		if err != nil {
			r.recorder.Eventf(c, v1.EventTypeWarning, subscriptionSyncFailed, "Failed to sync Subscription for the Channel: %v", err)
			return false, err
		}
		err = r.deleteTopic(ctx, originalPCS, gcpCreds)
		if err != nil {
			r.recorder.Eventf(c, v1.EventTypeWarning, topicDeleteFailed, "Failed to delete Topic for the Channel: %v", err)
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

	// We don't want to leak any external resources, so we will generate all the names we will use
	// and persist them to our status before creating anything. That way during delete we know
	// everything to look for. In fact, all manipulations of GCP resources will be done looking
	// only at the status, not the spec.
	persist, plannedPCS, subsToSync, err := r.planGcpResources(ctx, c, originalPCS)
	if err != nil {
		r.recorder.Eventf(c, v1.EventTypeWarning, gcpResourcesPlanFailed, "Failed to plan Channel's resources: %v", err)
		return false, err
	}
	if persist == persistStatus {
		if err = pubsubutil.SetInternalStatus(ctx, c, plannedPCS); err != nil {
			r.recorder.Eventf(c, v1.EventTypeWarning, gcpResourcesPersistFailed, "Failed to persist Channel's resources: %v", err)
			return false, err
		}
		// Persist this and run another reconcile loop to enact it.
		return true, nil
	}

	svc, err := r.createK8sService(ctx, c)
	if err != nil {
		r.recorder.Eventf(c, v1.EventTypeWarning, k8sServiceCreateFailed, "Failed to reconcile Channel's K8s Service: %v", err)
		return false, err
	}

	err = r.createVirtualService(ctx, c, svc)
	if err != nil {
		r.recorder.Eventf(c, v1.EventTypeWarning, virtualServiceCreateFailed, "Failed to reconcile Virtual Service for the Channel: %v", err)
		return false, err
	}

	topic, err := r.createTopic(ctx, plannedPCS, gcpCreds)
	if err != nil {
		r.recorder.Eventf(c, v1.EventTypeWarning, topicCreateFailed, "Failed to reconcile Topic for the Channel: %v", err)
		return false, err
	}

	err = r.syncSubscriptions(ctx, plannedPCS, gcpCreds, topic, subsToSync)
	if err != nil {
		r.recorder.Eventf(c, v1.EventTypeWarning, subscriptionSyncFailed, "Failed to reconcile Subscription for the Channel: %v", err)
		return false, err
	}
	// Now that the subs have synced successfully, remove the old ones from the status.
	plannedPCS.Subscriptions = subsToSync.subsToCreate
	if err = pubsubutil.SetInternalStatus(ctx, c, plannedPCS); err != nil {
		r.recorder.Eventf(c, v1.EventTypeWarning, subscriptionDeleteFailed, "Failed to delete old Subscriptions from the Channel's status: %v", err)
		return false, err
	}

	c.Status.MarkProvisioned()
	return false, nil
}

// planGcpResources creates the plan for every resource that needs to be created outside of the
// cluster. The plan is returned as both the GcpPubSubChannelStatus to save to the Channel object,
// as well as a list of the Subscriptions to create and a list of the Subscriptions to delete in the
// form of syncSubs. syncSubs should be used to drive the reconciliation of Subscriptions for the
// remainder of this reconcile loop (it contains a superset of the information in the status).
func (r *reconciler) planGcpResources(ctx context.Context, c *eventingv1alpha1.Channel, originalPCS *pubsubutil.GcpPubSubChannelStatus) (persistence, *pubsubutil.GcpPubSubChannelStatus, *syncSubs, error) {
	persist := noNeedToPersist
	subsToSync := &syncSubs{
		subsToCreate: make([]pubsubutil.GcpPubSubSubscriptionStatus, 0),
		subsToDelete: make([]pubsubutil.GcpPubSubSubscriptionStatus, 0),
	}

	topicName := originalPCS.Topic
	if topicName == "" {
		topicName = generateTopicName(c)
	}
	// Everything except the subscriptions.
	newPCS := &pubsubutil.GcpPubSubChannelStatus{
		// TODO Allow arguments to set the secret and project.
		Secret:     r.defaultSecret,
		SecretKey:  r.defaultSecretKey,
		GCPProject: r.defaultGcpProject,
		Topic:      topicName,
	}
	// Note that when we allow Channels to provide arguments for GCPProject and Topic, we probably
	// won't want to let them change once the GCP resources have been created. So the following
	// logic to detect a change will be insufficient.
	if !equality.Semantic.DeepEqual(originalPCS.Secret, newPCS.Secret) || originalPCS.SecretKey != newPCS.SecretKey || originalPCS.GCPProject != newPCS.GCPProject || originalPCS.Topic != newPCS.Topic {
		persist = persistStatus
	}

	// We are going to correlate subscriptions from the spec and the existing status. We use the
	// Subscription's UID to correlate between the two lists. If for any reason we can't get the UID
	// of a Subscription in the spec, then immediately error (another option would be to do all the
	// work we know we need to do, but then we need to make sure we don't delete anything as it may
	// match the unknown Subscription). We always expect to see the Subscription UID in the status
	// because this is the only code that writes out the status message and won't do it unless the
	// UID is present. But, if for some reason its not there, ignore that entry and allow the others
	// to process normally.

	existingSubs := make(map[types.UID]pubsubutil.GcpPubSubSubscriptionStatus, len(originalPCS.Subscriptions))
	for _, existingSub := range originalPCS.Subscriptions {
		// I don't think this can ever happen, but let's just be sure.
		if existingSub.Ref != nil && existingSub.Ref.UID != "" {
			existingSubs[existingSub.Ref.UID] = existingSub
		}
	}
	if c.Spec.Subscribable != nil {
		for _, subscriber := range c.Spec.Subscribable.Subscribers {
			if subscriber.Ref == nil || subscriber.Ref.UID == "" {
				return noNeedToPersist, nil, nil, fmt.Errorf("empty reference UID: %v", subscriber)
			}
			// Have we already synced this Subscription before? If so, reuse its existing
			// subscription. Everything else is allowed to change to the new values and doesn't need
			// to be persisted before processing (as it only affects the dispatcher, not anything in
			// GCP).
			var subscription string
			if existingSub, present := existingSubs[subscriber.Ref.UID]; present {
				delete(existingSubs, subscriber.Ref.UID)
				subscription = existingSub.Subscription
			} else {
				persist = persistStatus
				subscription = generateSubName(&subscriber)
			}
			subsToSync.subsToCreate = append(subsToSync.subsToCreate, pubsubutil.GcpPubSubSubscriptionStatus{
				Ref:           subscriber.Ref,
				SubscriberURI: subscriber.SubscriberURI,
				ReplyURI:      subscriber.ReplyURI,
				Subscription:  subscription,
			})
		}
	}

	// Any remaining existingSubs are no longer in the Channel spec, so should be deleted.
	for _, existingSub := range existingSubs {
		subsToSync.subsToDelete = append(subsToSync.subsToDelete, existingSub)
	}

	// Generate the subs for the status by copying all the subs to create and all the subs to
	// delete.
	newPCS.Subscriptions = make([]pubsubutil.GcpPubSubSubscriptionStatus, 0, len(subsToSync.subsToCreate)+len(subsToSync.subsToDelete))
	for _, sub := range subsToSync.subsToCreate {
		newPCS.Subscriptions = append(newPCS.Subscriptions, sub)
	}
	for _, sub := range subsToSync.subsToDelete {
		newPCS.Subscriptions = append(newPCS.Subscriptions, sub)
	}
	return persist, newPCS, subsToSync, nil
}

func (r *reconciler) createK8sService(ctx context.Context, c *eventingv1alpha1.Channel) (*v1.Service, error) {
	svc, err := util.CreateK8sService(ctx, r.client, c)
	if err != nil {
		logging.FromContext(ctx).Info("Error creating the Channel's K8s Service", zap.Error(err))
		return nil, err
	}

	c.Status.SetAddress(names.ServiceHostName(svc.Name, svc.Namespace))
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

func (r *reconciler) createTopic(ctx context.Context, plannedPCS *pubsubutil.GcpPubSubChannelStatus, gcpCreds *google.Credentials) (pubsubutil.PubSubTopic, error) {
	psc, err := r.pubSubClientCreator(ctx, gcpCreds, plannedPCS.GCPProject)
	if err != nil {
		logging.FromContext(ctx).Info("Unable to create PubSub client", zap.Error(err))
		return nil, err
	}

	topic := psc.Topic(plannedPCS.Topic)
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

func (r *reconciler) deleteTopic(ctx context.Context, pcs *pubsubutil.GcpPubSubChannelStatus, gcpCreds *google.Credentials) error {
	if pcs.Topic == "" {
		// The topic was never planned, let alone created, so there is nothing to delete.
		return nil
	}
	psc, err := r.pubSubClientCreator(ctx, gcpCreds, pcs.GCPProject)
	if err != nil {
		logging.FromContext(ctx).Info("Unable to create PubSubClient", zap.Error(err))
		return err
	}

	topic := psc.Topic(pcs.Topic)
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

type syncSubs struct {
	subsToCreate []pubsubutil.GcpPubSubSubscriptionStatus
	subsToDelete []pubsubutil.GcpPubSubSubscriptionStatus
}

func (r *reconciler) syncSubscriptions(ctx context.Context, plannedPCS *pubsubutil.GcpPubSubChannelStatus, gcpCreds *google.Credentials, topic pubsubutil.PubSubTopic, subsToSync *syncSubs) error {
	for _, subToCreate := range subsToSync.subsToCreate {
		_, err := r.createSubscription(ctx, gcpCreds, plannedPCS.GCPProject, topic, subToCreate.Subscription)
		if err != nil {
			logging.FromContext(ctx).Error("Unable to create subscriber", zap.Error(err), zap.Any("channelSubscriber", subToCreate))
			return err
		}
	}

	for _, subToDelete := range subsToSync.subsToDelete {
		err := r.deleteSubscription(ctx, gcpCreds, plannedPCS.GCPProject, &subToDelete)
		if err != nil {
			logging.FromContext(ctx).Error("Unable to delete subscriber", zap.Error(err), zap.Any("channelSubscriber", subToDelete))
			return err
		}
	}

	return nil
}

func (r *reconciler) createSubscription(ctx context.Context, gcpCreds *google.Credentials, gcpProject string, topic pubsubutil.PubSubTopic, subName string) (pubsubutil.PubSubSubscription, error) {
	psc, err := r.pubSubClientCreator(ctx, gcpCreds, gcpProject)
	if err != nil {
		return nil, err
	}
	sub := psc.SubscriptionInProject(subName, gcpProject)
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

func (r *reconciler) deleteSubscription(ctx context.Context, gcpCreds *google.Credentials, gcpProject string, subStatus *pubsubutil.GcpPubSubSubscriptionStatus) error {
	psc, err := r.pubSubClientCreator(ctx, gcpCreds, gcpProject)
	if err != nil {
		return err
	}
	sub := psc.SubscriptionInProject(subStatus.Subscription, gcpProject)
	exists, err := sub.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return sub.Delete(ctx)
}

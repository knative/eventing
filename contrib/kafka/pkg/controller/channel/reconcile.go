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
	"encoding/json"
	"fmt"

	"github.com/knative/eventing/contrib/kafka/pkg/utils"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/knative/eventing/contrib/kafka/pkg/controller"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	util "github.com/knative/eventing/pkg/provisioners"
	topicUtils "github.com/knative/eventing/pkg/provisioners/utils"
	eventingNames "github.com/knative/eventing/pkg/reconciler/names"
	"knative.dev/pkg/apis"
)

const (
	finalizerName = controllerAgentName

	// Name of the corev1.Events emitted from the reconciliation process
	dispatcherReconcileFailed    = "DispatcherReconcileFailed"
	dispatcherUpdateStatusFailed = "DispatcherUpdateStatusFailed"

	deprecatedMessage = "The `kafka` ClusterChannelProvisioner is deprecated and will be removed in 0.9. Recommended replacement is using `KafkaChannel` CRD."
)

type channelArgs struct {
	NumPartitions     int32
	ReplicationFactor int16
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Channel resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	r.logger.Info("Reconciling channel", zap.Any("request", request))
	channel := &eventingv1alpha1.Channel{}
	err := r.client.Get(context.TODO(), request.NamespacedName, channel)

	// The Channel may have been deleted since it was added to the workqueue. If so, there is
	// nothing to be done since the dependent resources would have been deleted as well.
	if errors.IsNotFound(err) {
		r.logger.Info("could not find channel", zap.Any("request", request))
		return reconcile.Result{}, nil
	}

	// Any other error should be retried in another reconciliation.
	if err != nil {
		r.logger.Error("could not fetch channel", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Skip Channel as it is not targeting any provisioner
	if channel.Spec.Provisioner == nil {
		return reconcile.Result{}, nil
	}

	// Skip channel not managed by this provisioner
	clusterChannelProvisioner, err := r.getClusterChannelProvisioner()
	if err != nil {
		return reconcile.Result{}, err
	}

	if !r.shouldReconcile(channel, clusterChannelProvisioner) {
		return reconcile.Result{}, nil
	}

	channel.Status.InitializeConditions()

	channel.Status.MarkDeprecated("ClusterChannelProvisionerDeprecated", deprecatedMessage)

	var requeue = false
	if clusterChannelProvisioner.Status.IsReady() {
		// Reconcile this copy of the Channel and then write back any status
		// updates regardless of whether the reconcile error out.
		requeue, err = r.reconcile(ctx, channel)
	} else {
		channel.Status.MarkNotProvisioned("NotProvisioned", "ClusterChannelProvisioner %s is not ready", clusterChannelProvisioner.Name)
		err = fmt.Errorf("ClusterChannelProvisioner %s is not ready", clusterChannelProvisioner.Name)
	}

	if err != nil {
		r.logger.Error("Dispatcher reconciliation failed", zap.Error(err))
		r.recorder.Eventf(channel, v1.EventTypeWarning, dispatcherReconcileFailed, "Dispatcher reconciliation failed: %v", err)
	} else {
		r.logger.Debug("Channel reconciled")
	}

	if updateChannelErr := util.UpdateChannel(ctx, r.client, channel); updateChannelErr != nil {
		r.logger.Info("failed to update channel status", zap.Error(updateChannelErr))
		r.recorder.Eventf(channel, v1.EventTypeWarning, dispatcherUpdateStatusFailed, "Failed to update Channel's dispatcher status: %v", err)
		return reconcile.Result{}, updateChannelErr
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{
		Requeue: requeue,
	}, err
}

// reconcile reconciles this Channel so that the real world matches the intended state. The returned
// boolean indicates if this Channel should be immediately requeued for another reconcile loop. The
// returned error indicates an error during reconciliation.
func (r *reconciler) reconcile(ctx context.Context, channel *eventingv1alpha1.Channel) (bool, error) {
	// We don't currently initialize r.kafkaClusterAdmin, hence we end up creating the cluster admin client every time.
	// This is because of an issue with Shopify/sarama. See https://github.com/Shopify/sarama/issues/1162.
	// Once the issue is fixed we should use a shared cluster admin client. Also, r.kafkaClusterAdmin is currently
	// used to pass a fake admin client in the tests.
	kafkaClusterAdmin := r.kafkaClusterAdmin
	if kafkaClusterAdmin == nil {
		var err error
		kafkaClusterAdmin, err = createKafkaAdminClient(r.config)
		if err != nil {
			r.logger.Error(fmt.Sprintf("unable to build kafka admin client for %v", r.config), zap.Error(err))
			return false, err
		}
	}

	// See if the channel has been deleted
	if channel.DeletionTimestamp != nil {
		r.logger.Info(fmt.Sprintf("DeletionTimestamp: %v", channel.DeletionTimestamp))
		if err := r.deprovisionChannel(channel, kafkaClusterAdmin); err != nil {
			return false, err
		}
		util.RemoveFinalizer(channel, finalizerName)
		return false, nil
	}

	// If we are adding the finalizer for the first time, then ensure that finalizer is persisted
	// before manipulating Kafka, which will not be automatically garbage collected by K8s if this
	// Channel is deleted.
	if addFinalizerResult := util.AddFinalizer(channel, finalizerName); addFinalizerResult == util.FinalizerAdded {
		return true, nil
	}

	if err := r.provisionChannel(channel, kafkaClusterAdmin); err != nil {
		channel.Status.MarkNotProvisioned("NotProvisioned", "error while provisioning: %s", err)
		return false, err
	}

	svc, err := util.CreateK8sService(ctx, r.client, channel, util.ExternalService(channel))
	if err != nil {
		r.logger.Info("error creating the Channel's K8s Service", zap.Error(err))
		return false, err
	}
	channel.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   eventingNames.ServiceHostName(svc.Name, svc.Namespace),
	})
	channel.Status.MarkProvisioned()

	// close the connection
	err = kafkaClusterAdmin.Close()
	if err != nil {
		r.logger.Error("error closing the connection", zap.Error(err))
		return false, err
	}

	return false, nil
}

func (r *reconciler) shouldReconcile(channel *eventingv1alpha1.Channel, clusterChannelProvisioner *eventingv1alpha1.ClusterChannelProvisioner) bool {
	return channel.Spec.Provisioner.Name == clusterChannelProvisioner.Name
}

func (r *reconciler) provisionChannel(channel *eventingv1alpha1.Channel, kafkaClusterAdmin sarama.ClusterAdmin) error {
	topicName := topicUtils.TopicName(utils.KafkaChannelSeparator, channel.Namespace, channel.Name)
	r.logger.Info("creating topic on kafka cluster", zap.String("topic", topicName))

	var arguments channelArgs

	if channel.Spec.Arguments != nil {
		var err error
		arguments, err = unmarshalArguments(channel.Spec.Arguments.Raw)
		if err != nil {
			return err
		}
	}

	if arguments.NumPartitions == 0 {
		arguments.NumPartitions = utils.DefaultNumPartitions
	}

	if arguments.ReplicationFactor == 0 {
		arguments.ReplicationFactor = utils.DefaultReplicationFactor
	}

	err := kafkaClusterAdmin.CreateTopic(topicName, &sarama.TopicDetail{
		ReplicationFactor: arguments.ReplicationFactor,
		NumPartitions:     arguments.NumPartitions,
	}, false)
	if err == sarama.ErrTopicAlreadyExists {
		return nil
	} else if err != nil {
		r.logger.Error("error creating topic", zap.String("topic", topicName), zap.Error(err))
	} else {
		r.logger.Info("successfully created topic", zap.String("topic", topicName))
	}
	return err
}

func (r *reconciler) deprovisionChannel(channel *eventingv1alpha1.Channel, kafkaClusterAdmin sarama.ClusterAdmin) error {
	topicName := topicUtils.TopicName(utils.KafkaChannelSeparator, channel.Namespace, channel.Name)
	r.logger.Info("deleting topic on kafka cluster", zap.String("topic", topicName))

	err := kafkaClusterAdmin.DeleteTopic(topicName)
	if err == sarama.ErrUnknownTopicOrPartition {
		return nil
	} else if err != nil {
		r.logger.Error("error deleting topic", zap.String("topic", topicName), zap.Error(err))
	} else {
		r.logger.Info("successfully deleted topic", zap.String("topic", topicName))
	}
	return err
}

func (r *reconciler) getClusterChannelProvisioner() (*eventingv1alpha1.ClusterChannelProvisioner, error) {
	clusterChannelProvisioner := &eventingv1alpha1.ClusterChannelProvisioner{}
	objKey := client.ObjectKey{
		Name: controller.Name,
	}
	if err := r.client.Get(context.Background(), objKey, clusterChannelProvisioner); err != nil {
		return nil, err
	}
	return clusterChannelProvisioner, nil
}

func createKafkaAdminClient(config *utils.KafkaConfig) (sarama.ClusterAdmin, error) {
	saramaConf := sarama.NewConfig()
	saramaConf.Version = sarama.V1_1_0_0
	saramaConf.ClientID = controllerAgentName
	return sarama.NewClusterAdmin(config.Brokers, saramaConf)
}

// unmarshalArguments unmarshal's a json/yaml serialized input and returns channelArgs
func unmarshalArguments(bytes []byte) (channelArgs, error) {
	var arguments channelArgs
	if len(bytes) > 0 {
		if err := json.Unmarshal(bytes, &arguments); err != nil {
			return arguments, fmt.Errorf("error unmarshalling arguments: %s", err)
		}
	}
	return arguments, nil
}

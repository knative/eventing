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

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/knative/eventing/contrib/kafka/pkg/controller"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	util "github.com/knative/eventing/pkg/provisioners"
	topicUtils "github.com/knative/eventing/pkg/provisioners/utils"
	eventingNames "github.com/knative/eventing/pkg/reconciler/names"
	"github.com/knative/eventing/pkg/sidecar/configmap"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"k8s.io/apimachinery/pkg/api/equality"
)

const (
	finalizerName = controllerAgentName

	DefaultNumPartitions = 1
)

type channelArgs struct {
	NumPartitions int32
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

	newChannel := channel.DeepCopy()

	newChannel.Status.InitializeConditions()

	var requeue = false
	if clusterChannelProvisioner.Status.IsReady() {
		// Reconcile this copy of the Channel and then write back any status
		// updates regardless of whether the reconcile error out.
		requeue, err = r.reconcile(ctx, newChannel)
	} else {
		newChannel.Status.MarkNotProvisioned("NotProvisioned", "ClusterChannelProvisioner %s is not ready", clusterChannelProvisioner.Name)
		err = fmt.Errorf("ClusterChannelProvisioner %s is not ready", clusterChannelProvisioner.Name)
	}

	if updateChannelErr := util.UpdateChannel(ctx, r.client, newChannel); updateChannelErr != nil {
		r.logger.Info("failed to update channel status", zap.Error(updateChannelErr))
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

	// We always need to sync the Channel config, so do it first.
	if err := r.syncChannelConfig(ctx); err != nil {
		r.logger.Info("error updating syncing the Channel config", zap.Error(err))
		return false, err
	}

	// We don't currently initialize r.kafkaClusterAdmin, hence we end up creating the cluster admin client every time.
	// This is because of an issue with Shopify/sarama. See https://github.com/Shopify/sarama/issues/1162.
	// Once the issue is fixed we should use a shared cluster admin client. Also, r.kafkaClusterAdmin is currently
	// used to pass a fake admin client in the tests.
	kafkaClusterAdmin := r.kafkaClusterAdmin
	if kafkaClusterAdmin == nil {
		var err error
		kafkaClusterAdmin, err = createKafkaAdminClient(r.config)
		if err != nil {
			r.logger.Fatal("unable to build kafka admin client", zap.Error(err))
			return false, err
		}
	}

	// See if the channel has been deleted
	accessor, err := meta.Accessor(channel)
	if err != nil {
		r.logger.Info("failed to get metadata", zap.Error(err))
		return false, err
	}
	deletionTimestamp := accessor.GetDeletionTimestamp()
	if deletionTimestamp != nil {
		r.logger.Info(fmt.Sprintf("DeletionTimestamp: %v", deletionTimestamp))
		if err = r.deprovisionChannel(channel, kafkaClusterAdmin); err != nil {
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

	if err = r.provisionChannel(channel, kafkaClusterAdmin); err != nil {
		channel.Status.MarkNotProvisioned("NotProvisioned", "error while provisioning: %s", err)
		return false, err
	}

	svc, err := util.CreateK8sService(ctx, r.client, channel)
	if err != nil {
		r.logger.Info("error creating the Channel's K8s Service", zap.Error(err))
		return false, err
	}
	channel.Status.SetAddress(eventingNames.ServiceHostName(svc.Name, svc.Namespace))

	_, err = util.CreateVirtualService(ctx, r.client, channel, svc)
	if err != nil {
		r.logger.Info("error creating the Virtual Service for the Channel", zap.Error(err))
		return false, err
	}

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
	topicName := topicUtils.TopicName(controller.KafkaChannelSeparator, channel.Namespace, channel.Name)
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
		arguments.NumPartitions = DefaultNumPartitions
	}

	err := kafkaClusterAdmin.CreateTopic(topicName, &sarama.TopicDetail{
		ReplicationFactor: 1,
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
	topicName := topicUtils.TopicName(controller.KafkaChannelSeparator, channel.Namespace, channel.Name)
	r.logger.Info("deleting topic on kafka cluster", zap.String("topic", topicName))

	err := kafkaClusterAdmin.DeleteTopic(topicName)
	if err == sarama.ErrUnknownTopicOrPartition {
		return nil
	} else if err != nil {
		r.logger.Error("error deleting topic", zap.String("topic", topicName), zap.Error(err))
	} else {
		r.logger.Info("successfully deleted topic %s", zap.String("topic", topicName))
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

func (r *reconciler) syncChannelConfig(ctx context.Context) error {
	channels, err := r.listAllChannels(ctx)
	if err != nil {
		r.logger.Info("Unable to list channels", zap.Error(err))
		return err
	}
	config := multiChannelFanoutConfig(channels)
	return r.writeConfigMap(ctx, config)
}

func (r *reconciler) writeConfigMap(ctx context.Context, config *multichannelfanout.Config) error {
	logger := r.logger.With(zap.Any("configMap", r.configMapKey))

	updated, err := configmap.SerializeConfig(*config)
	if err != nil {
		r.logger.Error("Unable to serialize config", zap.Error(err), zap.Any("config", config))
		return err
	}

	cm := &corev1.ConfigMap{}
	err = r.client.Get(ctx, r.configMapKey, cm)
	if errors.IsNotFound(err) {
		cm = r.createNewConfigMap(updated)
		err = r.client.Create(ctx, cm)
		if err != nil {
			logger.Info("Unable to create ConfigMap", zap.Error(err))
			return err
		}
	}
	if err != nil {
		logger.Info("Unable to get ConfigMap", zap.Error(err))
		return err
	}

	if equality.Semantic.DeepEqual(cm.Data, updated) {
		// Nothing to update.
		return nil
	}

	cm.Data = updated
	return r.client.Update(ctx, cm)
}

func (r *reconciler) createNewConfigMap(data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.configMapKey.Namespace,
			Name:      r.configMapKey.Name,
		},
		Data: data,
	}
}

func multiChannelFanoutConfig(channels []eventingv1alpha1.Channel) *multichannelfanout.Config {
	cc := make([]multichannelfanout.ChannelConfig, 0)
	for _, c := range channels {
		channelConfig := multichannelfanout.ChannelConfig{
			Namespace: c.Namespace,
			Name:      c.Name,
		}
		if c.Spec.Subscribable != nil {
			channelConfig.FanoutConfig = fanout.Config{
				Subscriptions: c.Spec.Subscribable.Subscribers,
			}
		}
		cc = append(cc, channelConfig)
	}
	return &multichannelfanout.Config{
		ChannelConfigs: cc,
	}
}

func (r *reconciler) listAllChannels(ctx context.Context) ([]eventingv1alpha1.Channel, error) {
	clusterChannelProvisioner, err := r.getClusterChannelProvisioner()
	if err != nil {
		return nil, err
	}

	channels := make([]eventingv1alpha1.Channel, 0)

	opts := &client.ListOptions{
		// TODO this is here because the fake client needs it. Remove this when it's no longer
		// needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
				Kind:       "Channel",
			},
		},
	}
	for {
		cl := &eventingv1alpha1.ChannelList{}
		if err = r.client.List(ctx, opts, cl); err != nil {
			return nil, err
		}

		for _, c := range cl.Items {
			if r.shouldReconcile(&c, clusterChannelProvisioner) {
				channels = append(channels, c)
			}
		}
		if cl.Continue != "" {
			opts.Raw.Continue = cl.Continue
		} else {
			return channels, nil
		}
	}
}

func createKafkaAdminClient(config *controller.KafkaProvisionerConfig) (sarama.ClusterAdmin, error) {
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

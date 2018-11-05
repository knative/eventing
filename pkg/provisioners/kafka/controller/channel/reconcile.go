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
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/knative/eventing/pkg/provisioners/kafka/controller"
)

const (
	finalizerName = controllerAgentName

	ArgumentNumPartitions = "NumPartitions"
	DefaultNumPartitions  = 1
)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Channel resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	r.logger.Info("Reconciling channel", zap.Any("request", request))
	channel := &v1alpha1.Channel{}
	err := r.client.Get(context.TODO(), request.NamespacedName, channel)

	if errors.IsNotFound(err) {
		r.logger.Info("could not find channel", zap.Any("request", request))
		return reconcile.Result{}, nil
	}

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
	provisionerRef := channel.Spec.Provisioner
	if provisionerRef.Name != clusterChannelProvisioner.Name {
		return reconcile.Result{}, nil
	}

	newChannel := channel.DeepCopy()

	newChannel.Status.InitializeConditions()

	if clusterChannelProvisioner.Status.IsReady() {
		// Reconcile this copy of the Channel and then write back any status
		// updates regardless of whether the reconcile error out.
		err = r.reconcile(newChannel)
	} else {
		newChannel.Status.MarkNotProvisioned("NotProvisioned", "ClusterChannelProvisioner %s is not ready", clusterChannelProvisioner.Name)
		err = fmt.Errorf("ClusterChannelProvisioner %s is not ready", clusterChannelProvisioner.Name)
	}

	if err := r.updateChannel(ctx, newChannel); err != nil {
		r.logger.Info("failed to update channel status", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, err
}

func (r *reconciler) reconcile(channel *v1alpha1.Channel) error {
	// See if the channel has been deleted
	accessor, err := meta.Accessor(channel)
	if err != nil {
		r.logger.Info("failed to get metadata", zap.Error(err))
		return err
	}

	kafkaClusterAdmin, err := createKafkaAdminClient(r.config)
	if err != nil {
		r.logger.Fatal("unable to build kafka admin client", zap.Error(err))
		return err
	}

	deletionTimestamp := accessor.GetDeletionTimestamp()
	if deletionTimestamp != nil {
		r.logger.Info(fmt.Sprintf("DeletionTimestamp: %v", deletionTimestamp))
		if err := r.deprovisionChannel(channel, kafkaClusterAdmin); err != nil {
			return err
		}
		r.removeFinalizer(channel)
		return nil
	}

	r.addFinalizer(channel)

	if err := r.provisionChannel(channel, kafkaClusterAdmin); err != nil {
		channel.Status.MarkNotProvisioned("NotProvisioned", "error while provisioning: %s", err)
		return err
	}
	channel.Status.MarkProvisioned()

	// close the connection
	kafkaClusterAdmin.Close();

	return nil
}

func (r *reconciler) provisionChannel(channel *v1alpha1.Channel, kafkaClusterAdmin sarama.ClusterAdmin) error {
	topicName := topicName(channel)
	r.logger.Info("creating topic on kafka cluster", zap.String("topic", topicName))

	partitions := DefaultNumPartitions

	if channel.Spec.Arguments != nil {
		var err error
		arguments, err := unmarshalArguments(channel.Spec.Arguments.Raw)
		if err != nil {
			return err
		}
		if num, ok := arguments[ArgumentNumPartitions]; ok {
			parsedNum, ok := num.(float64)
			if !ok {
				return fmt.Errorf("could not parse argument %s for channel %s", ArgumentNumPartitions, fmt.Sprintf("%s/%s", channel.Namespace, channel.Name))
			}
			partitions = int(parsedNum)
		}
	}

	err := kafkaClusterAdmin.CreateTopic(topicName, &sarama.TopicDetail{
		ReplicationFactor: 1,
		NumPartitions:     int32(partitions),
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

func (r *reconciler) deprovisionChannel(channel *v1alpha1.Channel, kafkaClusterAdmin sarama.ClusterAdmin) error {
	topicName := topicName(channel)
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

func (r *reconciler) getClusterChannelProvisioner() (*v1alpha1.ClusterChannelProvisioner, error) {
	clusterChannelProvisioner := &v1alpha1.ClusterChannelProvisioner{}
	objKey := client.ObjectKey{
		Name: controller.Name,
	}
	if err := r.client.Get(context.Background(), objKey, clusterChannelProvisioner); err != nil {
		return nil, err
	}
	return clusterChannelProvisioner, nil
}

func (r *reconciler) updateChannel(ctx context.Context, u *v1alpha1.Channel) error {
	channel := &v1alpha1.Channel{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, channel)
	if err != nil {
		return err
	}

	updated := false
	if !equality.Semantic.DeepEqual(channel.Finalizers, u.Finalizers) {
		channel.SetFinalizers(u.ObjectMeta.Finalizers)
		updated = true
	}

	if !equality.Semantic.DeepEqual(channel.Status, u.Status) {
		channel.Status = u.Status
		updated = true
	}

	if updated == false {
		return nil
	}
	return r.client.Update(ctx, channel)
}

func (r *reconciler) addFinalizer(channel *v1alpha1.Channel) {
	finalizers := sets.NewString(channel.Finalizers...)
	finalizers.Insert(finalizerName)
	channel.Finalizers = finalizers.List()
}

func (r *reconciler) removeFinalizer(channel *v1alpha1.Channel) {
	finalizers := sets.NewString(channel.Finalizers...)
	finalizers.Delete(finalizerName)
	channel.Finalizers = finalizers.List()
}

func createKafkaAdminClient(config *controller.KafkaProvisionerConfig) (sarama.ClusterAdmin, error) {
	saramaConf := sarama.NewConfig()
	saramaConf.Version = sarama.V1_1_0_0
	saramaConf.ClientID = controllerAgentName
	return sarama.NewClusterAdmin(config.Brokers, saramaConf)
}

func topicName(channel *v1alpha1.Channel) string {
	return fmt.Sprintf("%s.%s", channel.Namespace, channel.Name)
}

// unmarshalArguments unmarshal's a json/yaml serialized input and returns a map structure
func unmarshalArguments(bytes []byte) (map[string]interface{}, error) {
	arguments := make(map[string]interface{})
	if len(bytes) > 0 {
		if err := json.Unmarshal(bytes, &arguments); err != nil {
			return nil, fmt.Errorf("error unmarshalling arguments: %s", err)
		}
	}
	return arguments, nil
}

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

	"github.com/Shopify/sarama"
	"github.com/ghodss/yaml"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"go.uber.org/zap"
)

const (
	ArgumentNumPartitions = "NumPartitions"
	DefaultNumPartitions  = 1
)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Channel resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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
	if channel.Spec.Provisioner == nil || channel.Spec.Provisioner.Ref == nil {
		return reconcile.Result{}, nil
	}

	// Skip channel not managed by this provisioner
	provisionerRef := channel.Spec.Provisioner.Ref
	clusterProvisioner, err := r.getClusterProvisioner()
	if err != nil {
		return reconcile.Result{}, err
	}

	if provisionerRef.Name != clusterProvisioner.Name || provisionerRef.Namespace != clusterProvisioner.Namespace {
		return reconcile.Result{}, nil
	}

	original := channel.DeepCopy()

	if clusterProvisioner.Status.IsReady() {
		// Reconcile this copy of the Channel and then write back any status
		// updates regardless of whether the reconcile error out.
		err = r.reconcile(channel)
	} else {
		channel.Status.MarkNotProvisioned("NotProvisioned", "ClusterProvisioner %s is not ready", clusterProvisioner.Name)
		err = fmt.Errorf("ClusterProvisioner %s is not ready", clusterProvisioner.Name)
	}

	if !equality.Semantic.DeepEqual(original.Status, channel.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
		if _, err := r.updateStatus(channel); err != nil {
			r.logger.Info("failed to update channel status", zap.Error(err))
			return reconcile.Result{}, err
		}
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
	deletionTimestamp := accessor.GetDeletionTimestamp()
	if deletionTimestamp != nil {
		r.logger.Info(fmt.Sprintf("DeletionTimestamp: %v", deletionTimestamp))
		//TODO: Handle deletion
		return nil
	}

	channel.Status.InitializeConditions()
	if err := r.provisionChannel(channel); err != nil {
		channel.Status.MarkNotProvisioned("NotProvisioned", "error while provisioning: %s", err)
		return err
	}
	channel.Status.MarkProvisioned()
	return nil
}

func (r *reconciler) provisionChannel(channel *v1alpha1.Channel) error {
	topicName := topicName(channel)
	r.logger.Info("provisioning topic on kafka cluster", zap.String("topic", topicName))

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

	err := r.kafkaClusterAdmin.CreateTopic(topicName, &sarama.TopicDetail{
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

func (r *reconciler) getClusterProvisioner() (*v1alpha1.ClusterProvisioner, error) {
	clusterProvisioner := &v1alpha1.ClusterProvisioner{}
	objKey := client.ObjectKey{
		Name: r.config.Name,
	}
	if err := r.client.Get(context.TODO(), objKey, clusterProvisioner); err != nil {
		return nil, err
	}
	return clusterProvisioner, nil
}

func (r *reconciler) updateStatus(channel *v1alpha1.Channel) (*v1alpha1.Channel, error) {
	newChannel := &v1alpha1.Channel{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: channel.Namespace, Name: channel.Name}, newChannel)

	if err != nil {
		return nil, err
	}
	newChannel.Status = channel.Status

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Channel resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	if err = r.client.Update(context.TODO(), newChannel); err != nil {
		return nil, err
	}
	return newChannel, nil
}

func topicName(channel *v1alpha1.Channel) string {
	return fmt.Sprintf("%s.%s", channel.Namespace, channel.Name)
}

// unmarshalArguments unmarshal's a json/yaml serialized input and returns a map structure
func unmarshalArguments(bytes []byte) (map[string]interface{}, error) {
	arguments := make(map[string]interface{})
	if len(bytes) > 0 {
		if err := yaml.Unmarshal(bytes, &arguments); err != nil {
			return nil, err
		}
	}
	return arguments, nil
}

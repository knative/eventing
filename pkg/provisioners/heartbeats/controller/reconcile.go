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

package controller

import (
	"context"
	"encoding/json"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners/heartbeats/controller/resources"
	"github.com/knative/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	provisionerName = "heartbeats"
)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two.

func (r *reconciler) Reconcile(ctx context.Context, object runtime.Object) (runtime.Object, error) {
	logger := logging.FromContext(ctx)

	source, ok := object.(*v1alpha1.Source)
	if !ok {
		logger.Errorf("could not find source %v\n", object)
		return object, nil
	}

	if source.Spec.Provisioner.Ref.Name != provisionerName {
		logger.Errorf("heartbeats skipping source %s, provisioned by %s\n", source.Name, source.Spec.Provisioner.Ref.Name)
		return source, nil
	}

	source.Status.InitializeConditions()

	// TODO:
	//// See if the source has been deleted
	//accessor, err := meta.Accessor(source)
	//if err != nil {
	//	logger.Warnf("Failed to get metadata accessor: %s", err)
	//	return err
	//}
	//deletionTimestamp := accessor.GetDeletionTimestamp()

	args := &resources.HeartBeatArguments{}

	if source.Spec.Arguments != nil {
		if err := json.Unmarshal(source.Spec.Arguments.Raw, args); err != nil {
			logger.Infof("Error: %s failed to unmarshal arguments, %v", source.Name, err)
		}
	}
	args.Name = source.Name
	args.Namespace = source.Namespace

	channel, err := r.getChannel(ctx, source)
	if err != nil {
		fqn := "Channel.eventing.knative.dev/v1alpha1"
		if errors.IsNotFound(err) {
			channel, err = r.createChannel(ctx, source, nil, args)
			if err != nil {
				return object, err
			}
			r.recorder.Eventf(source, corev1.EventTypeNormal, "Provisioned", "Created channel %q", channel.Name)
			source.Status.SetProvisionedObjectState(channel.Name, fqn, "Created", "Created channel %q", channel.Name)
			source.Status.MarkDeprovisioned("Provisioning", "Provisioning Channel %s", args.Name)
		} else {
			if channel.Status.IsReady() {
				source.Status.SetProvisionedObjectState(channel.Name, fqn, "Ready", "")
			}
		}

		source.Status.Subscribable.Channelable.Namespace = channel.Namespace
		source.Status.Subscribable.Channelable.Name = channel.Name
		source.Status.Subscribable.Channelable.APIVersion = "eventing.knative.dev/v1alpha1"
		source.Status.Subscribable.Channelable.Kind = "Channel"

	}

	pod, err := r.getPod(ctx, source)
	if err != nil {
		fqn := "Pod.core/v1"
		if errors.IsNotFound(err) {
			pod, err = r.createPod(ctx, source, nil, channel, args)
			if err != nil {
				r.recorder.Eventf(source, corev1.EventTypeNormal, "Blocked", "waiting for %v", err)
				source.Status.SetProvisionedObjectState(args.Name, fqn, "Blocked", "waiting for %v", args.Name, err)
				return object, err
			}
			r.recorder.Eventf(source, corev1.EventTypeNormal, "Provisioned", "Created pod %q", pod.Name)
			source.Status.SetProvisionedObjectState(pod.Name, fqn, "Created", "Created pod %q", pod.Name)
			source.Status.MarkDeprovisioned("Provisioning", "Provisioning Pod %s", args.Name)
		} else {
			if pod.Status.Phase == corev1.PodRunning {
				source.Status.SetProvisionedObjectState(pod.Name, fqn, "Ready", "")
				source.Status.MarkProvisioned()
			}
		}
	}

	// TODO: need informers for the object we are controlling

	return source, nil
}

func (r *reconciler) getChannel(ctx context.Context, source *v1alpha1.Source) (*v1alpha1.Channel, error) {
	logger := logging.FromContext(ctx)

	list := &v1alpha1.ChannelList{}
	err := r.client.List(
		ctx,
		&client.ListOptions{
			Namespace:     source.Namespace,
			LabelSelector: labels.Everything(),
			// TODO this is here because the fake client needs it. Remove this when it's no longer
			// needed.
			Raw: &metav1.ListOptions{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1alpha1.SchemeGroupVersion.String(),
					Kind:       "Channel",
				},
			},
		},
		list)
	if err != nil {
		logger.Errorf("Unable to list channels: %v", err)
		return nil, err
	}
	for _, c := range list.Items {
		if metav1.IsControlledBy(&c, source) {
			return &c, nil
		}
	}
	return nil, errors.NewNotFound(schema.GroupResource{}, "")
}

func (r *reconciler) createChannel(ctx context.Context, source *v1alpha1.Source, org *v1alpha1.Channel, args *resources.HeartBeatArguments) (*v1alpha1.Channel, error) {
	channel, err := resources.MakeChannel(source, org, args)
	if err != nil {
		return nil, err
	}

	if err := r.client.Create(ctx, channel); err != nil {
		return nil, err
	}
	return channel, nil
}

func (r *reconciler) getPod(ctx context.Context, source *v1alpha1.Source) (*corev1.Pod, error) {
	logger := logging.FromContext(ctx)

	list := &corev1.PodList{}
	err := r.client.List(
		ctx,
		&client.ListOptions{
			Namespace:     source.Namespace,
			LabelSelector: labels.Everything(),
			// TODO this is here because the fake client needs it. Remove this when it's no longer
			// needed.
			Raw: &metav1.ListOptions{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Pod",
				},
			},
		},
		list)
	if err != nil {
		logger.Errorf("Unable to list pods: %v", err)
		return nil, err
	}
	for _, c := range list.Items {
		if metav1.IsControlledBy(&c, source) {
			return &c, nil
		}
	}
	return nil, errors.NewNotFound(schema.GroupResource{}, "")
}

func (r *reconciler) createPod(ctx context.Context, source *v1alpha1.Source, org *corev1.Pod, channel *v1alpha1.Channel, args *resources.HeartBeatArguments) (*corev1.Pod, error) {
	pod, err := resources.MakePod(source, org, channel, args)
	if err != nil {
		return nil, err
	}

	if err := r.client.Create(ctx, pod); err != nil {
		return nil, err
	}
	return pod, nil
}

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
	"github.com/knative/eventing/pkg/provisioners/container/controller/resources"
	"github.com/knative/pkg/logging"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	provisionerName = "container"
)

type reconciler struct {
	client        client.Client
	restConfig    *rest.Config
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder
}

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
		logger.Errorf("skipping source %s, provisioned by %s\n", source.Name, source.Spec.Provisioner.Ref.Name)
		return source, nil
	}

	// See if the source has been deleted
	accessor, err := meta.Accessor(source)
	if err != nil {
		logger.Warnf("Failed to get metadata accessor: %s", err)
		return object, err
	}
	// No need to reconcile if the source has been marked for deletion.
	deletionTimestamp := accessor.GetDeletionTimestamp()
	if deletionTimestamp != nil {
		return object, err
	}

	source.Status.InitializeConditions()

	args := &resources.ContainerArguments{}
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

	deploy, err := r.getDeployment(ctx, source)
	if err != nil {
		fqn := "Deployment.apps/v1"
		if errors.IsNotFound(err) {
			deploy, err = r.createDeployment(ctx, source, nil, channel, args)
			if err != nil {
				r.recorder.Eventf(source, corev1.EventTypeNormal, "Blocked", "waiting for %v", err)
				source.Status.SetProvisionedObjectState(args.Name, fqn, "Blocked", "waiting for %v", args.Name, err)
				return object, err
			}
			r.recorder.Eventf(source, corev1.EventTypeNormal, "Provisioned", "Created deployment %q", deploy.Name)
			source.Status.SetProvisionedObjectState(deploy.Name, fqn, "Created", "Created deployment %q", deploy.Name)
			source.Status.MarkDeprovisioned("Provisioning", "Provisioning deployment %s", args.Name)
		} else {
			if deploy.Status.ReadyReplicas > 0 {
				source.Status.SetProvisionedObjectState(deploy.Name, fqn, "Ready", "")
				source.Status.MarkProvisioned()
			}
		}
	}

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

func (r *reconciler) createChannel(ctx context.Context, source *v1alpha1.Source, org *v1alpha1.Channel, args *resources.ContainerArguments) (*v1alpha1.Channel, error) {
	channel, err := resources.MakeChannel(source, org, args)
	if err != nil {
		return nil, err
	}

	if err := r.client.Create(ctx, channel); err != nil {
		return nil, err
	}
	return channel, nil
}

func (r *reconciler) getDeployment(ctx context.Context, source *v1alpha1.Source) (*appsv1.Deployment, error) {
	logger := logging.FromContext(ctx)

	list := &appsv1.DeploymentList{}
	err := r.client.List(
		ctx,
		&client.ListOptions{
			Namespace:     source.Namespace,
			LabelSelector: labels.Everything(),
			// TODO this is here because the fake client needs it. Remove this when it's no longer
			// needed.
			Raw: &metav1.ListOptions{
				TypeMeta: metav1.TypeMeta{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
				},
			},
		},
		list)
	if err != nil {
		logger.Errorf("Unable to list deployments: %v", err)
		return nil, err
	}
	for _, c := range list.Items {
		if metav1.IsControlledBy(&c, source) {
			return &c, nil
		}
	}
	return nil, errors.NewNotFound(schema.GroupResource{}, "")
}

func (r *reconciler) createDeployment(ctx context.Context, source *v1alpha1.Source, org *appsv1.Deployment, channel *v1alpha1.Channel, args *resources.ContainerArguments) (*appsv1.Deployment, error) {
	deployment, err := resources.MakeDeployment(source, org, channel, args)
	if err != nil {
		return nil, err
	}

	if err := r.client.Create(ctx, deployment); err != nil {
		return nil, err
	}
	return deployment, nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectConfig(c *rest.Config) error {
	r.restConfig = c
	var err error
	r.dynamicClient, err = dynamic.NewForConfig(c)
	return err
}

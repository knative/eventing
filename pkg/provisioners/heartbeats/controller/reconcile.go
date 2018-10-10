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
	"fmt"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners/heartbeats/controller/resources"
	"github.com/knative/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	provisionerName = "heartbeats"
)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	logger := logging.FromContext(ctx)

	logger.Infof("Reconciling source %v", request)
	source := &v1alpha1.Source{}
	err := r.client.Get(context.TODO(), request.NamespacedName, source)

	if errors.IsNotFound(err) {
		logger.Errorf("could not find source %v\n", request)
		return reconcile.Result{}, nil
	}

	if err != nil {
		logger.Errorf("could not fetch Source %v for %+v\n", err, request)
		return reconcile.Result{}, err
	}

	if source.Spec.Provisioner.Ref.Name != provisionerName {
		logger.Errorf("heartbeats skipping source %s, provisioned by %s\n", source.Name, source.Spec.Provisioner.Ref.Name)
		return reconcile.Result{}, nil
	}

	original := source.DeepCopy()

	// Reconcile this copy of the Source and then write back any status
	// updates regardless of whether the reconcile error out.
	err = r.reconcile(ctx, source)
	if err != nil {
		logger.Warnf("Failed to reconcile source: %v", err)
	}
	if equality.Semantic.DeepEqual(original.Status, source.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := r.updateStatus(source); err != nil {
		logger.Warnf("Failed to update source status: %v", err)
		return reconcile.Result{}, err
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, err
}

func (r *reconciler) reconcile(ctx context.Context, source *v1alpha1.Source) error {
	logger := logging.FromContext(ctx)

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

	channel := &v1alpha1.Channel{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: args.Namespace, Name: args.Name}, channel); err != nil {
		fqn := fmt.Sprintf("%s/%s", channel.Kind, channel.APIVersion)
		if errors.IsNotFound(err) {
			channel, err = r.createChannel(ctx, source, nil, args)
			if err != nil {
				return err
			}
			r.recorder.Eventf(source, corev1.EventTypeNormal, "Provisioned", "Created channel %q", channel.Name)
			source.Status.SetProvisionedObjectState(channel.Name, fqn, "Created", "Created channel %q", channel.Name)
			source.Status.MarkDeprovisioned("Provisioning", "Provisioning Channel %s", args.Name)
		} else {
			if channel.Status.IsReady() {
				source.Status.SetProvisionedObjectState(channel.Name, fqn, "Ready", "")
			}
		}
	}

	pod := &corev1.Pod{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: args.Namespace, Name: args.Name}, pod); err != nil {
		fqn := fmt.Sprintf("%s/%s", pod.Kind, pod.APIVersion)
		if errors.IsNotFound(err) {
			pod, err = r.createPod(ctx, source, nil, channel, args)
			if err != nil {
				r.recorder.Eventf(source, corev1.EventTypeNormal, "Blocked", "waiting for %v", err)
				source.Status.SetProvisionedObjectState(args.Name, fqn, "Blocked", "waiting for %v", args.Name, err)
				return err
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

	return nil
}

func (r *reconciler) updateStatus(source *v1alpha1.Source) (*v1alpha1.Source, error) {
	newSource := &v1alpha1.Source{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: source.Namespace, Name: source.Name}, newSource)

	if err != nil {
		return nil, err
	}
	newSource.Status = source.Status

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Source resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	if err = r.client.Update(context.TODO(), newSource); err != nil {
		return nil, err
	}
	return newSource, nil
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

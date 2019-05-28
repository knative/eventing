/*
Copyright 2019 The Knative Authors

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

package testing

import (
	"context"
	"time"

	"github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	//	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	"k8s.io/apimachinery/pkg/types"
)

// PipelineOption enables further configuration of a Pipeline.
type PipelineOption func(*v1alpha1.Pipeline)

// NewPipeline creates an Pipeline with PipelineOptions.
func NewPipeline(name, namespace string, popt ...PipelineOption) *v1alpha1.Pipeline {
	p := &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.PipelineSpec{},
	}
	for _, opt := range popt {
		opt(p)
	}
	p.SetDefaults(context.Background())
	return p
}

func WithInitPipelineConditions(p *v1alpha1.Pipeline) {
	p.Status.InitializeConditions()
}

func WithPipelineDeleted(p *v1alpha1.Pipeline) {
	deleteTime := metav1.NewTime(time.Unix(1e9, 0))
	p.ObjectMeta.SetDeletionTimestamp(&deleteTime)
}

func WithPipelineChannelTemplateSpecCRD(gvk metav1.GroupVersionKind) PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Spec.ChannelTemplate = v1alpha1.ChannelTemplateSpec{
			ChannelCRD: corev1.ObjectReference{
				APIVersion: apiVersion(gvk),
				Kind:       gvk.Kind,
			},
		}
	}
}

/*
func WithPipelineDeploymentNotReady(reason, message string) PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Status.MarkDispatcherFailed(reason, message)
	}
}

func WithPipelineDeploymentReady() PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Status.PropagateDispatcherStatus(&appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue}}})
	}
}

func WithPipelineServicetNotReady(reason, message string) PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Status.MarkServiceFailed(reason, message)
	}
}

func WithPipelineServiceReady() PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Status.MarkServiceTrue()
	}
}

func WithPipelineChannelServicetNotReady(reason, message string) PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Status.MarkChannelServiceFailed(reason, message)
	}
}

func WithPipelineChannelServiceReady() PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Status.MarkChannelServiceTrue()
	}
}

func WithPipelineEndpointsNotReady(reason, message string) PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Status.MarkEndpointsFailed(reason, message)
	}
}

func WithPipelineEndpointsReady() PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Status.MarkEndpointsTrue()
	}
}

func WithPipelineAddress(a string) PipelineOption {
	return func(p *v1alpha1.Pipeline) {
		p.Status.SetAddress(a)
	}
}
*/

/*
Copyright 2020 The Knative Authors

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
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	fakeapiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apiextensionsv1listers "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	batchv1listers "k8s.io/client-go/listers/batch/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1beta3 "knative.dev/eventing/pkg/apis/eventing/v1beta3"
	flowsv1 "knative.dev/eventing/pkg/apis/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	sinksv1alpha1 "knative.dev/eventing/pkg/apis/sinks/v1alpha1"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	fakeeventingclientset "knative.dev/eventing/pkg/client/clientset/versioned/fake"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1"
	eventingv1alpha1listers "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	eventingv1beta3listers "knative.dev/eventing/pkg/client/listers/eventing/v1beta3"
	flowslisters "knative.dev/eventing/pkg/client/listers/flows/v1"
	messaginglisters "knative.dev/eventing/pkg/client/listers/messaging/v1"
	sinkslisters "knative.dev/eventing/pkg/client/listers/sinks/v1alpha1"
	sourcelisters "knative.dev/eventing/pkg/client/listers/sources/v1"
	testscheme "knative.dev/eventing/pkg/reconciler/testing/scheme"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/reconciler/testing"
)

var clientSetSchemes = []func(*runtime.Scheme) error{
	fakekubeclientset.AddToScheme,
	fakeeventingclientset.AddToScheme,
	fakeapiextensionsclientset.AddToScheme,
	duckv1.AddToScheme,
	eventingduck.AddToScheme,
	testscheme.Eventing.AddToScheme,
	testscheme.Serving.AddToScheme,
}

type Listers struct {
	sorter testing.ObjectSorter
}

func NewScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	for _, addTo := range clientSetSchemes {
		addTo(scheme)
	}
	return scheme
}

func NewListers(objs []runtime.Object) Listers {
	scheme := runtime.NewScheme()

	for _, addTo := range clientSetSchemes {
		addTo(scheme)
	}

	ls := Listers{
		sorter: testing.NewObjectSorter(scheme),
	}

	ls.sorter.AddObjects(objs...)

	return ls
}

func (l *Listers) indexerFor(obj runtime.Object) cache.Indexer {
	return l.sorter.IndexerForObjectType(obj)
}

func (l *Listers) GetKubeObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fakekubeclientset.AddToScheme)
}

func (l *Listers) GetEventingObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fakeeventingclientset.AddToScheme)
}

func (l *Listers) GetSubscriberObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(testscheme.SubscriberToScheme)
}

func (l *Listers) GetAllObjects() []runtime.Object {
	all := l.GetSubscriberObjects()
	all = append(all, l.GetEventingObjects()...)
	all = append(all, l.GetKubeObjects()...)
	return all
}

func (l *Listers) GetEventTypeLister() eventingv1beta3listers.EventTypeLister {
	return eventingv1beta3listers.NewEventTypeLister(l.indexerFor(&eventingv1beta3.EventType{}))
}

func (l *Listers) GetEventPolicyLister() eventingv1alpha1listers.EventPolicyLister {
	return eventingv1alpha1listers.NewEventPolicyLister(l.indexerFor(&eventingv1alpha1.EventPolicy{}))
}

func (l *Listers) GetJobSinkLister() sinkslisters.JobSinkLister {
	return sinkslisters.NewJobSinkLister(l.indexerFor(&sinksv1alpha1.JobSink{}))
}

func (l *Listers) GetPingSourceLister() sourcelisters.PingSourceLister {
	return sourcelisters.NewPingSourceLister(l.indexerFor(&sourcesv1.PingSource{}))
}

func (l *Listers) GetSubscriptionLister() messaginglisters.SubscriptionLister {
	return messaginglisters.NewSubscriptionLister(l.indexerFor(&messagingv1.Subscription{}))
}

func (l *Listers) GetSequenceLister() flowslisters.SequenceLister {
	return flowslisters.NewSequenceLister(l.indexerFor(&flowsv1.Sequence{}))
}

func (l *Listers) GetTriggerLister() eventinglisters.TriggerLister {
	return eventinglisters.NewTriggerLister(l.indexerFor(&eventingv1.Trigger{}))
}

func (l *Listers) GetBrokerLister() eventinglisters.BrokerLister {
	return eventinglisters.NewBrokerLister(l.indexerFor(&eventingv1.Broker{}))
}

func (l *Listers) GetInMemoryChannelLister() messaginglisters.InMemoryChannelLister {
	return messaginglisters.NewInMemoryChannelLister(l.indexerFor(&messagingv1.InMemoryChannel{}))
}

func (l *Listers) GetMessagingChannelLister() messaginglisters.ChannelLister {
	return messaginglisters.NewChannelLister(l.indexerFor(&messagingv1.Channel{}))
}

func (l *Listers) GetParallelLister() flowslisters.ParallelLister {
	return flowslisters.NewParallelLister(l.indexerFor(&flowsv1.Parallel{}))
}

func (l *Listers) GetApiServerSourceLister() sourcelisters.ApiServerSourceLister {
	return sourcelisters.NewApiServerSourceLister(l.indexerFor(&sourcesv1.ApiServerSource{}))
}

func (l *Listers) GetSinkBindingLister() sourcelisters.SinkBindingLister {
	return sourcelisters.NewSinkBindingLister(l.indexerFor(&sourcesv1.SinkBinding{}))
}

func (l *Listers) GetContainerSourceLister() sourcelisters.ContainerSourceLister {
	return sourcelisters.NewContainerSourceLister(l.indexerFor(&sourcesv1.ContainerSource{}))
}

func (l *Listers) GetDeploymentLister() appsv1listers.DeploymentLister {
	return appsv1listers.NewDeploymentLister(l.indexerFor(&appsv1.Deployment{}))
}

func (l *Listers) GetK8sServiceLister() corev1listers.ServiceLister {
	return corev1listers.NewServiceLister(l.indexerFor(&corev1.Service{}))
}

func (l *Listers) GetSecretLister() corev1listers.SecretLister {
	return corev1listers.NewSecretLister(l.indexerFor(&corev1.Secret{}))
}

func (l *Listers) GetNamespaceLister() corev1listers.NamespaceLister {
	return corev1listers.NewNamespaceLister(l.indexerFor(&corev1.Namespace{}))
}

func (l *Listers) GetServiceAccountLister() corev1listers.ServiceAccountLister {
	return corev1listers.NewServiceAccountLister(l.indexerFor(&corev1.ServiceAccount{}))
}

func (l *Listers) GetServiceLister() corev1listers.ServiceLister {
	return corev1listers.NewServiceLister(l.indexerFor(&corev1.Service{}))
}

func (l *Listers) GetRoleLister() rbacv1listers.RoleLister {
	return rbacv1listers.NewRoleLister(l.indexerFor(&rbacv1.Role{}))
}

func (l *Listers) GetRoleBindingLister() rbacv1listers.RoleBindingLister {
	return rbacv1listers.NewRoleBindingLister(l.indexerFor(&rbacv1.RoleBinding{}))
}

func (l *Listers) GetEndpointsLister() corev1listers.EndpointsLister {
	return corev1listers.NewEndpointsLister(l.indexerFor(&corev1.Endpoints{}))
}

func (l *Listers) GetConfigMapLister() corev1listers.ConfigMapLister {
	return corev1listers.NewConfigMapLister(l.indexerFor(&corev1.ConfigMap{}))
}

func (l *Listers) GetCustomResourceDefinitionLister() apiextensionsv1listers.CustomResourceDefinitionLister {
	return apiextensionsv1listers.NewCustomResourceDefinitionLister(l.indexerFor(&apiextensionsv1.CustomResourceDefinition{}))
}

func (l *Listers) GetNodeLister() corev1listers.NodeLister {
	return corev1listers.NewNodeLister(l.indexerFor(&corev1.Node{}))
}

func (l *Listers) GetPodLister() corev1listers.PodLister {
	return corev1listers.NewPodLister(l.indexerFor(&corev1.Pod{}))
}

func (l *Listers) GetJobLister() batchv1listers.JobLister {
	return batchv1listers.NewJobLister(l.indexerFor(&batchv1.Job{}))
}

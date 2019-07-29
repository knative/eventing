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

package testing

import (
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "github.com/knative/eventing/pkg/apis/messaging/v1alpha1"
	sourcesv1alpha1 "github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	fakeeventingclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	eventinglisters "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	messaginglisters "github.com/knative/eventing/pkg/client/listers/messaging/v1alpha1"
	sourcelisters "github.com/knative/eventing/pkg/client/listers/sources/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	fakeapiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apiextensionsv1beta1listers "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	fakesharedclientset "knative.dev/pkg/client/clientset/versioned/fake"
	"knative.dev/pkg/reconciler/testing"
)

var subscriberAddToScheme = func(scheme *runtime.Scheme) error {
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "testing.eventing.knative.dev", Version: "v1alpha1", Kind: "Subscriber"}, &unstructured.Unstructured{})
	return nil
}

var clientSetSchemes = []func(*runtime.Scheme) error{
	fakekubeclientset.AddToScheme,
	fakesharedclientset.AddToScheme,
	fakeeventingclientset.AddToScheme,
	fakeapiextensionsclientset.AddToScheme,
	subscriberAddToScheme,
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
	return l.sorter.ObjectsForSchemeFunc(subscriberAddToScheme)
}

func (l *Listers) GetAllObjects() []runtime.Object {
	all := l.GetSubscriberObjects()
	all = append(all, l.GetEventingObjects()...)
	all = append(all, l.GetKubeObjects()...)
	return all
}

func (l *Listers) GetSharedObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fakesharedclientset.AddToScheme)
}

func (l *Listers) GetSubscriptionLister() eventinglisters.SubscriptionLister {
	return eventinglisters.NewSubscriptionLister(l.indexerFor(&eventingv1alpha1.Subscription{}))
}

func (l *Listers) GetTriggerLister() eventinglisters.TriggerLister {
	return eventinglisters.NewTriggerLister(l.indexerFor(&eventingv1alpha1.Trigger{}))
}

func (l *Listers) GetBrokerLister() eventinglisters.BrokerLister {
	return eventinglisters.NewBrokerLister(l.indexerFor(&eventingv1alpha1.Broker{}))
}

func (l *Listers) GetEventTypeLister() eventinglisters.EventTypeLister {
	return eventinglisters.NewEventTypeLister(l.indexerFor(&eventingv1alpha1.EventType{}))
}

func (l *Listers) GetInMemoryChannelLister() messaginglisters.InMemoryChannelLister {
	return messaginglisters.NewInMemoryChannelLister(l.indexerFor(&messagingv1alpha1.InMemoryChannel{}))
}

func (l *Listers) GetChannelLister() eventinglisters.ChannelLister {
	return eventinglisters.NewChannelLister(l.indexerFor(&eventingv1alpha1.Channel{}))
}

func (l *Listers) GetMessagingChannelLister() messaginglisters.ChannelLister {
	return messaginglisters.NewChannelLister(l.indexerFor(&messagingv1alpha1.Channel{}))
}

func (l *Listers) GetSequenceLister() messaginglisters.SequenceLister {
	return messaginglisters.NewSequenceLister(l.indexerFor(&messagingv1alpha1.Sequence{}))
}

func (l *Listers) GetChoiceLister() messaginglisters.ChoiceLister {
	return messaginglisters.NewChoiceLister(l.indexerFor(&messagingv1alpha1.Choice{}))
}

func (l *Listers) GetCronJobSourceLister() sourcelisters.CronJobSourceLister {
	return sourcelisters.NewCronJobSourceLister(l.indexerFor(&sourcesv1alpha1.CronJobSource{}))
}

func (l *Listers) GetApiServerSourceLister() sourcelisters.ApiServerSourceLister {
	return sourcelisters.NewApiServerSourceLister(l.indexerFor(&sourcesv1alpha1.ApiServerSource{}))
}

func (l *Listers) GetContainerSourceLister() sourcelisters.ContainerSourceLister {
	return sourcelisters.NewContainerSourceLister(l.indexerFor(&sourcesv1alpha1.ContainerSource{}))
}

func (l *Listers) GetDeploymentLister() appsv1listers.DeploymentLister {
	return appsv1listers.NewDeploymentLister(l.indexerFor(&appsv1.Deployment{}))
}

func (l *Listers) GetK8sServiceLister() corev1listers.ServiceLister {
	return corev1listers.NewServiceLister(l.indexerFor(&corev1.Service{}))
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

func (l *Listers) GetRoleBindingLister() rbacv1listers.RoleBindingLister {
	return rbacv1listers.NewRoleBindingLister(l.indexerFor(&rbacv1.RoleBinding{}))
}

func (l *Listers) GetEndpointsLister() corev1listers.EndpointsLister {
	return corev1listers.NewEndpointsLister(l.indexerFor(&corev1.Endpoints{}))
}

func (l *Listers) GetConfigMapLister() corev1listers.ConfigMapLister {
	return corev1listers.NewConfigMapLister(l.indexerFor(&corev1.ConfigMap{}))
}

func (l *Listers) GetCustomResourceDefinitionLister() apiextensionsv1beta1listers.CustomResourceDefinitionLister {
	return apiextensionsv1beta1listers.NewCustomResourceDefinitionLister(l.indexerFor(&apiextensionsv1beta1.CustomResourceDefinition{}))
}

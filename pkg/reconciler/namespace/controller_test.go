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

package namespace

import (
	"testing"

	fakeclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/reconciler"
	logtesting "github.com/knative/pkg/logging/testing"
	kubeinformers "k8s.io/client-go/informers"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
)

func TestNew(t *testing.T) {
	defer logtesting.ClearAll()
	kubeClient := fakekubeclientset.NewSimpleClientset()
	eventingClient := fakeclientset.NewSimpleClientset()
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	eventingInformer := eventinginformers.NewSharedInformerFactory(eventingClient, 0)

	namespaceInformer := kubeInformer.Core().V1().Namespaces()
	serviceAccountInformer := kubeInformer.Core().V1().ServiceAccounts()
	roleBindingInformer := kubeInformer.Rbac().V1().RoleBindings()
	brokerInformer := eventingInformer.Eventing().V1alpha1().Brokers()

	c := NewController(reconciler.Options{
		KubeClientSet:     kubeClient,
		EventingClientSet: eventingClient,
		Logger:            logtesting.TestLogger(t),
	}, namespaceInformer, serviceAccountInformer, roleBindingInformer, brokerInformer)

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

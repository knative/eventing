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

package duck

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"

	eventingduckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	pkgapisduck "github.com/knative/pkg/apis/duck"

	"github.com/knative/eventing/pkg/reconciler"
)

type SubscribableInformer interface {
	VerifyType(objRef *corev1.ObjectReference) error
}

type subscribableInformer struct {
	duck pkgapisduck.InformerFactory
}

// NewSubscribableInformer creates a new Subscribable Informer.
func NewSubscribableInformer(opt reconciler.Options) SubscribableInformer {
	return &subscribableInformer{
		duck: &pkgapisduck.TypedInformerFactory{
			Client: opt.DynamicClientSet,
			// TODO the duck.Channel should probably be called SubscribableType.
			Type:         &eventingduckv1alpha1.Channel{},
			ResyncPeriod: opt.ResyncPeriod,
			StopChannel:  opt.StopChannel,
		},
	}
}

// VerifyType verifies that object reference is a Subscribable.
func (i *subscribableInformer) VerifyType(objRef *corev1.ObjectReference) error {
	if objRef == nil {
		return fmt.Errorf("objRef is nil")
	}

	gvr, _ := meta.UnsafeGuessKindToResource(objRef.GroupVersionKind())
	_, lister, err := i.duck.Get(gvr)
	if err != nil {
		return fmt.Errorf("error getting a lister for a channel resource '%+v': %+v", gvr, err)
	}

	obj, err := lister.ByNamespace(objRef.Namespace).Get(objRef.Name)
	if err != nil {
		return fmt.Errorf("error fetching channel %+v: %v", objRef, err)
	}

	_, ok := obj.(*eventingduckv1alpha1.Channel)
	if !ok {
		return fmt.Errorf("object %+v is of the wrong kind", objRef)
	}
	return nil
}

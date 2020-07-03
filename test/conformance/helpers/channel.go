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

package helpers

import (
	"github.com/pkg/errors"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/duck"
	"knative.dev/eventing/test/lib/resources"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"

	"knative.dev/eventing/pkg/apis/messaging"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
)

var (
	channelv1beta1GVK = (&messagingv1beta1.Channel{}).GetGroupVersionKind()

	channelv1beta1 = metav1.TypeMeta{
		Kind:       channelv1beta1GVK.Kind,
		APIVersion: channelv1beta1GVK.GroupVersion().String(),
	}

	channelv1GVK = (&messagingv1.Channel{}).GetGroupVersionKind()

	channelv1 = metav1.TypeMeta{
		Kind:       channelv1GVK.Kind,
		APIVersion: channelv1GVK.GroupVersion().String(),
	}
)

func getChannelDuckTypeSupportVersion(channelName string, client *testlib.Client, channel *metav1.TypeMeta) (string, error) {
	metaResource := resources.NewMetaResource(channelName, client.Namespace, channel)
	obj, err := duck.GetGenericObject(client.Dynamic, metaResource, &eventingduckv1beta1.Channelable{})
	if err != nil {
		return "", errors.Wrapf(err, "Unable to GET the channel %v", metaResource)
	}
	channelable, ok := obj.(*eventingduckv1beta1.Channelable)
	if !ok {
		return "", errors.Wrapf(err, "Unable to cast the channel %v", metaResource)
	}
	return channelable.ObjectMeta.Annotations[messaging.SubscribableDuckVersionAnnotation], nil
}

func getChannelAsV1Beta1Channelable(channelName string, client *testlib.Client, channel metav1.TypeMeta) (*eventingduckv1beta1.Channelable, error) {
	metaResource := resources.NewMetaResource(channelName, client.Namespace, &channel)
	obj, err := duck.GetGenericObject(client.Dynamic, metaResource, &eventingduckv1beta1.Channelable{})
	if err != nil {
		return nil, errors.Wrapf(err, "Unable to get the channel as v1beta1 Channel duck type: %q", channel)
	}
	channelable, ok := obj.(*eventingduckv1beta1.Channelable)
	if !ok {
		return nil, errors.Errorf("Unable to cast channel %q to v1beta1 duck type", channel)
	}

	return channelable, nil
}

func getChannelAsV1Alpha1Channelable(channelName string, client *testlib.Client, channel metav1.TypeMeta) (*eventingduckv1alpha1.Channelable, error) {
	metaResource := resources.NewMetaResource(channelName, client.Namespace, &channel)
	obj, err := duck.GetGenericObject(client.Dynamic, metaResource, &eventingduckv1alpha1.Channelable{})
	if err != nil {
		return nil, errors.Wrapf(err, "Unable to get the channel as v1alpha1 Channel duck type: %q", channel)
	}
	channelable, ok := obj.(*eventingduckv1alpha1.Channelable)
	if !ok {
		return nil, errors.Errorf("Unable to cast channel %q to v1alpha1 duck type", channel)
	}

	return channelable, nil
}

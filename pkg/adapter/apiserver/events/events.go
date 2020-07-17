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

package events

import (
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	sourcesv1beta1 "knative.dev/eventing/pkg/apis/sources/v1beta1"
)

func MakeAddEvent(source string, obj interface{}, ref bool) (cloudevents.Event, error) {
	if obj == nil {
		return cloudevents.Event{}, fmt.Errorf("resource can not be nil")
	}
	object := obj.(*unstructured.Unstructured)

	var data interface{}
	var eventType string
	if ref {
		data = getRef(object)
		eventType = sourcesv1beta1.ApiServerSourceAddRefEventType
	} else {
		data = object
		eventType = sourcesv1beta1.ApiServerSourceAddEventType
	}

	return makeEvent(source, eventType, object, data)
}

func MakeUpdateEvent(source string, obj interface{}, ref bool) (cloudevents.Event, error) {
	if obj == nil {
		return cloudevents.Event{}, fmt.Errorf("resource can not be nil")
	}
	object := obj.(*unstructured.Unstructured)

	var data interface{}
	var eventType string
	if ref {
		data = getRef(object)
		eventType = sourcesv1beta1.ApiServerSourceUpdateRefEventType
	} else {
		data = object
		eventType = sourcesv1beta1.ApiServerSourceUpdateEventType
	}

	return makeEvent(source, eventType, object, data)
}

func MakeDeleteEvent(source string, obj interface{}, ref bool) (cloudevents.Event, error) {
	if obj == nil {
		return cloudevents.Event{}, fmt.Errorf("resource can not be nil")
	}
	object := obj.(*unstructured.Unstructured)
	var data interface{}
	var eventType string
	if ref {
		data = getRef(object)
		eventType = sourcesv1beta1.ApiServerSourceDeleteRefEventType
	} else {
		data = object
		eventType = sourcesv1beta1.ApiServerSourceDeleteEventType
	}

	return makeEvent(source, eventType, object, data)
}

func getRef(object *unstructured.Unstructured) corev1.ObjectReference {
	return corev1.ObjectReference{
		APIVersion: object.GetAPIVersion(),
		Kind:       object.GetKind(),
		Name:       object.GetName(),
		Namespace:  object.GetNamespace(),
	}
}

func makeEvent(source, eventType string, obj *unstructured.Unstructured, data interface{}) (cloudevents.Event, error) {
	subject := createSelfLink(corev1.ObjectReference{
		APIVersion: obj.GetAPIVersion(),
		Kind:       obj.GetKind(),
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
	})

	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetType(eventType)
	event.SetSource(source)
	event.SetSubject(subject)

	if err := event.SetData(cloudevents.ApplicationJSON, data); err != nil {
		return event, err
	}
	return event, nil
}

// Creates a URI of the form found in object metadata selfLinks
// Format looks like: /apis/feeds.knative.dev/v1alpha1/namespaces/default/feeds/k8s-events-example
// KNOWN ISSUES:
// * ObjectReference.APIVersion has no version information (e.g. serving.knative.dev rather than serving.knative.dev/v1alpha1)
// * ObjectReference does not have enough information to create the pluaralized list type (e.g. "revisions" from kind: Revision)
//
// Track these issues at https://github.com/kubernetes/kubernetes/issues/66313
// We could possibly work around this by adding a lister for the resources referenced by these events.
func createSelfLink(o corev1.ObjectReference) string {
	gvr, _ := meta.UnsafeGuessKindToResource(o.GroupVersionKind())
	versionNameHack := o.APIVersion

	// Core API types don't have a separate package name and only have a version string (e.g. /apis/v1/namespaces/default/pods/myPod)
	// To avoid weird looking strings like "v1/versionUnknown" we'll sniff for a "." in the version
	if strings.Contains(versionNameHack, ".") && !strings.Contains(versionNameHack, "/") {
		versionNameHack = versionNameHack + "/versionUnknown"
	}
	return fmt.Sprintf("/apis/%s/namespaces/%s/%s/%s", versionNameHack, o.Namespace, gvr.Resource, o.Name)
}

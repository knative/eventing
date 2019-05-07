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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	cloudevents "github.com/cloudevents/sdk-go"
)

const (
	addEventType    = "dev.knative.apiserver.resource.add"
	updateEventType = "dev.knative.apiserver.resource.update"
	deleteEventType = "dev.knative.apiserver.resource.delete"

	addEventRefType    = "dev.knative.apiserver.ref.add"
	updateEventRefType = "dev.knative.apiserver.ref.update"
	deleteEventRefType = "dev.knative.apiserver.ref.delete"
)

type ResourceEvent struct {
	Object    *unstructured.Unstructured `json:"obj,omitempty"`
	NewObject *unstructured.Unstructured `json:"newObj,omitempty"`
	OldObject *unstructured.Unstructured `json:"oldObj,omitempty"`
}

func MakeAddEvent(source string, obj interface{}) (*cloudevents.Event, error) {
	object := obj.(*unstructured.Unstructured)

	return makeEvent(source, addEventType, object, &ResourceEvent{
		Object: object,
	})
}

func MakeUpdateEvent(source string, oldObj, newObj interface{}) (*cloudevents.Event, error) {
	object := newObj.(*unstructured.Unstructured)

	return makeEvent(source, updateEventType, object, &ResourceEvent{
		NewObject: object,
		OldObject: oldObj.(*unstructured.Unstructured),
	})
}

func MakeDeleteEvent(source string, obj interface{}) (*cloudevents.Event, error) {
	object := obj.(*unstructured.Unstructured)

	return makeEvent(source, deleteEventType, object, &ResourceEvent{
		Object: object,
	})
}

func getRef(object *unstructured.Unstructured, controller bool) corev1.ObjectReference {
	if controller {
		if owner := metav1.GetControllerOf(object); owner != nil {
			return corev1.ObjectReference{
				APIVersion: owner.APIVersion,
				Kind:       owner.Kind,
				Name:       owner.Name,
				Namespace:  object.GetNamespace(),
			}
		}
	}
	return corev1.ObjectReference{
		APIVersion: object.GetAPIVersion(),
		Kind:       object.GetKind(),
		Name:       object.GetName(),
		Namespace:  object.GetNamespace(),
	}
}

func MakeAddRefEvent(source string, asController bool, obj interface{}) (*cloudevents.Event, error) {
	object := obj.(*unstructured.Unstructured)
	return makeEvent(source, addEventRefType, object, getRef(object, asController))
}

func MakeUpdateRefEvent(source string, asController bool, oldObj, newObj interface{}) (*cloudevents.Event, error) {
	object := newObj.(*unstructured.Unstructured)
	return makeEvent(source, updateEventRefType, object, getRef(object, asController))
}

func MakeDeleteRefEvent(source string, asController bool, obj interface{}) (*cloudevents.Event, error) {
	object := obj.(*unstructured.Unstructured)
	return makeEvent(source, deleteEventRefType, object, getRef(object, asController))
}

func makeEvent(source, eventType string, obj *unstructured.Unstructured, data interface{}) (*cloudevents.Event, error) {
	subject := createSelfLink(corev1.ObjectReference{
		APIVersion: obj.GetAPIVersion(),
		Kind:       obj.GetKind(),
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
	})

	event := cloudevents.NewEvent()
	event.SetType(eventType)
	event.SetSource(source)
	event.SetSubject(subject)

	if err := event.SetData(data); err != nil {
		return nil, err
	}
	return &event, nil
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
	collectionNameHack := strings.ToLower(o.Kind) + "s"
	versionNameHack := o.APIVersion

	// Core API types don't have a separate package name and only have a version string (e.g. /apis/v1/namespaces/default/pods/myPod)
	// To avoid weird looking strings like "v1/versionUnknown" we'll sniff for a "." in the version
	if strings.Contains(versionNameHack, ".") && !strings.Contains(versionNameHack, "/") {
		versionNameHack = versionNameHack + "/versionUnknown"
	}
	return fmt.Sprintf("/apis/%s/namespaces/%s/%s/%s", versionNameHack, o.Namespace, collectionNameHack, o.Name)
}

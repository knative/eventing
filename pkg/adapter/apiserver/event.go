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

package apiserver

import (
	"fmt"
	cloudevents "github.com/cloudevents/sdk-go"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"strings"
)

const (
	addEventType    = "dev.knative.apiserver.object.add"
	updateEventType = "dev.knative.apiserver.object.update"
	deleteEventType = "dev.knative.apiserver.object.delete"
)

type KubernetesEvent struct {
	Object    *duckv1alpha1.KResource `json:"obj,omitempty"`
	NewObject *duckv1alpha1.KResource `json:"newObj,omitempty"`
	OldObject *duckv1alpha1.KResource `json:"oldObj,omitempty"`
}

func (a *adapter) makeAddEvent(obj interface{}) (*cloudevents.Event, error) {
	object := obj.(*duckv1alpha1.KResource)

	return a.makeEvent(addEventType, object, &KubernetesEvent{
		Object: object,
	})
}

func (a *adapter) makeUpdateEvent(oldObj, newObj interface{}) (*cloudevents.Event, error) {
	object := newObj.(*duckv1alpha1.KResource)

	return a.makeEvent(updateEventType, object, &KubernetesEvent{
		NewObject: object,
		OldObject: oldObj.(*duckv1alpha1.KResource),
	})
}

func (a *adapter) makeDeleteEvent(obj interface{}) (*cloudevents.Event, error) {
	object := obj.(*duckv1alpha1.KResource)

	return a.makeEvent(deleteEventType, object, &KubernetesEvent{
		Object: object,
	})
}

func (a *adapter) makeEvent(eventType string, obj *duckv1alpha1.KResource, data *KubernetesEvent) (*cloudevents.Event, error) {
	subject := createSelfLink(corev1.ObjectReference{
		APIVersion: obj.APIVersion,
		Kind:       obj.Kind,
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
	})

	event := cloudevents.NewEvent()
	event.SetType(eventType)
	event.SetSource(a.source)
	event.SetSubject(subject)

	if err := event.SetData(data); err != nil {
		a.logger.Warn("failed to set event data for kubernetes event")
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

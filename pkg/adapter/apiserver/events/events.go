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
	"context"
	"fmt"
	"strings"

	kncloudevents "knative.dev/eventing/pkg/adapter/v2"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	sources "knative.dev/eventing/pkg/apis/sources"
)

const (
	resourceGroup = "apiserversources.sources.knative.dev"
)

// MakeAddEvent returns a cloudevent when a k8s api event is created.
func MakeAddEvent(source string, obj interface{}, ref bool) (context.Context, cloudevents.Event, error) {
	if obj == nil {
		return nil, cloudevents.Event{}, fmt.Errorf("resource can not be nil")
	}
	object := obj.(*unstructured.Unstructured)

	var data interface{}
	var eventType string
	if ref {
		data = getRef(object)
		eventType = sources.ApiServerSourceAddRefEventType
	} else {
		data = object
		eventType = sources.ApiServerSourceAddEventType
	}

	return makeEvent(source, eventType, object, data)
}

// MakeUpdateEvent returns a cloudevent when a k8s api event is updated.
func MakeUpdateEvent(source string, obj interface{}, ref bool) (context.Context, cloudevents.Event, error) {
	if obj == nil {
		return nil, cloudevents.Event{}, fmt.Errorf("resource can not be nil")
	}
	object := obj.(*unstructured.Unstructured)

	var data interface{}
	var eventType string
	if ref {
		data = getRef(object)
		eventType = sources.ApiServerSourceUpdateRefEventType
	} else {
		data = object
		eventType = sources.ApiServerSourceUpdateEventType
	}

	return makeEvent(source, eventType, object, data)
}

// MakeDeleteEvent returns a cloudevent when a k8s api event is deleted.
func MakeDeleteEvent(source string, obj interface{}, ref bool) (context.Context, cloudevents.Event, error) {
	if obj == nil {
		return nil, cloudevents.Event{}, fmt.Errorf("resource can not be nil")
	}
	object := obj.(*unstructured.Unstructured)
	var data interface{}
	var eventType string

	if ref {
		data = getRef(object)
		eventType = sources.ApiServerSourceDeleteRefEventType
	} else {
		data = object
		eventType = sources.ApiServerSourceDeleteEventType
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

func makeEvent(source, eventType string, obj *unstructured.Unstructured, data interface{}) (context.Context, cloudevents.Event, error) {
	resourceName := obj.GetName()
	kind := obj.GetKind()
	namespace := obj.GetNamespace()
	subject := createSelfLink(corev1.ObjectReference{
		APIVersion: obj.GetAPIVersion(),
		Kind:       kind,
		Name:       resourceName,
		Namespace:  namespace,
	})

	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetType(eventType)
	event.SetSource(source)
	event.SetSubject(subject)
	// We copy the resource kind, name and namespace as extensions so that triggers can do the filter based on these attributes
	event.SetExtension("kind", kind)
	event.SetExtension("name", resourceName)
	event.SetExtension("namespace", namespace)
	if err := event.SetData(cloudevents.ApplicationJSON, data); err != nil {
		return nil, event, err
	}

	ctx := context.Background()
	metricTag := &kncloudevents.MetricTag{
		Namespace:     namespace,
		Name:          resourceName,
		ResourceGroup: resourceGroup,
	}
	ctx = kncloudevents.ContextWithMetricTag(ctx, metricTag)

	return ctx, event, nil
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

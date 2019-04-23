/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cronjobsource

import (
	"context"
	"errors"
	"fmt"
	"testing"

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	controllertesting "github.com/knative/eventing-sources/pkg/controller/testing"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	trueVal = true
)

const (
	raImage = "test-ra-image"

	image        = "github.com/knative/test/image"
	sourceName   = "test-cronjob-source"
	sourceUID    = "1234-5678-90"
	testNS       = "testnamespace"
	testSchedule = "*/2 * * * *"
	testData     = "data"

	addressableName       = "testsink"
	addressableKind       = "Sink"
	addressableAPIVersion = "duck.knative.dev/v1alpha1"
	addressableDNS        = "sinkable.sink.svc.cluster.local"
	addressableURI        = "http://sinkable.sink.svc.cluster.local/"
)

func init() {
	// Add types to scheme
	v1.AddToScheme(scheme.Scheme)
	corev1.AddToScheme(scheme.Scheme)
	sourcesv1alpha1.SchemeBuilder.AddToScheme(scheme.Scheme)
	duckv1alpha1.AddToScheme(scheme.Scheme)
}

func TestReconcile(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name: "not a Cron Job source",
			// This is not a CronJobSource.
			Reconciles: getNonCronJobSource(),
			InitialState: []runtime.Object{
				getNonCronJobSource(),
			},
		}, {
			Name: "invalid schedule",
			InitialState: []runtime.Object{
				getSourceWithInvalidSchedule(),
			},
			Reconciles: getSourceWithInvalidSchedule(),
			WantErrMsg: "Expected exactly 5 fields, found 2: invalid schedule",
		}, {
			Name: "cannot get sinkURI",
			InitialState: []runtime.Object{
				getSource(),
			},
			WantPresent: []runtime.Object{
				getSourceWithoutSink(),
			},
			WantErrMsg: "sinks.duck.knative.dev \"testsink\" not found",
		}, {
			Name: "cannot create receive adapter",
			InitialState: []runtime.Object{
				getSource(),
				getAddressable(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("test-induced-error")
					},
				},
			},
			WantPresent: []runtime.Object{
				getSourceWithSink(),
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "cannot list deployments",
			InitialState: []runtime.Object{
				getSource(),
				getAddressable(),
			},
			Mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, _ *client.ListOptions, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("test-induced-error")
					},
				},
			},
			WantPresent: []runtime.Object{
				getSourceWithSink(),
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "successful create",
			InitialState: []runtime.Object{
				getSource(),
				getAddressable(),
			},
			WantPresent: []runtime.Object{
				getReadySource(),
			},
		}, {
			Name: "successful create - reuse existing receive adapter",
			InitialState: []runtime.Object{
				getSource(),
				getAddressable(),
				getReceiveAdapter(),
			},
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("an error that won't be seen because create is not called")
					},
				},
			},
			WantPresent: []runtime.Object{
				getReadySource(),
			},
		},
	}
	for _, tc := range testCases {
		tc.IgnoreTimes = true
		tc.ReconcileKey = fmt.Sprintf("%s/%s", testNS, sourceName)
		if tc.Reconciles == nil {
			tc.Reconciles = getSource()
		}
		tc.Scheme = scheme.Scheme

		c := tc.GetClient()
		r := &reconciler{
			client: c,
			scheme: tc.Scheme,

			receiveAdapterImage: raImage,
		}
		r.InjectClient(c)
		t.Run(tc.Name, tc.Runner(t, r, c))
	}
}

func getNonCronJobSource() *sourcesv1alpha1.ContainerSource {
	obj := &sourcesv1alpha1.ContainerSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
			Kind:       "ContainerSource",
		},
		ObjectMeta: om(testNS, sourceName),
		Spec: sourcesv1alpha1.ContainerSourceSpec{
			Image: image,
			Args:  []string(nil),
			Sink: &corev1.ObjectReference{
				Name:       addressableName,
				Kind:       addressableKind,
				APIVersion: addressableAPIVersion,
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}

func getSource() *sourcesv1alpha1.CronJobSource {
	obj := &sourcesv1alpha1.CronJobSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
			Kind:       "CronJobSource",
		},
		ObjectMeta: om(testNS, sourceName),
		Spec: sourcesv1alpha1.CronJobSourceSpec{
			Schedule: testSchedule,
			Data:     testData,
			Sink: &corev1.ObjectReference{
				Name:       addressableName,
				Kind:       addressableKind,
				APIVersion: addressableAPIVersion,
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}

func getDeletingSource() *sourcesv1alpha1.CronJobSource {
	src := getSource()
	src.DeletionTimestamp = &deletionTime
	return src
}

func getSourceWithInvalidSchedule() *sourcesv1alpha1.CronJobSource {
	src := getSource()
	src.Spec.Schedule = "invalid schedule"
	return src
}

func getSourceWithoutSink() *sourcesv1alpha1.CronJobSource {
	src := getSource()
	src.Status.InitializeConditions()
	src.Status.MarkSchedule()
	src.Status.MarkNoSink("NotFound", "")
	return src
}

func getSourceWithSink() *sourcesv1alpha1.CronJobSource {
	src := getSource()
	src.Status.InitializeConditions()
	src.Status.MarkSchedule()
	src.Status.MarkSink(addressableURI)
	return src
}

func getReadySource() *sourcesv1alpha1.CronJobSource {
	src := getSourceWithSink()
	src.Status.MarkDeployed()
	return src
}

func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/sources/v1alpha1/namespaces/%s/object/%s", namespace, name),
		UID:       sourceUID,
	}
}

func getAddressable() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": addressableAPIVersion,
			"kind":       addressableKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      addressableName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"hostname": addressableDNS,
				},
			},
		},
	}
}

func getReceiveAdapter() *v1.Deployment {
	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					Controller: &trueVal,
					UID:        sourceUID,
				},
			},
			Labels:    getLabels(getSource()),
			Namespace: testNS,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		Spec: v1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "receive-adapter",
							Image: raImage,
							Env: []corev1.EnvVar{
								{
									Name:  "SCHEDULE",
									Value: testSchedule,
								},
								{
									Name:  "DATA",
									Value: testData,
								},
								{
									Name:  "SINK_URI",
									Value: addressableURI,
								},
							},
						},
					},
				},
			},
		},
	}
}

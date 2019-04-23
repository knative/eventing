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
	//"context"
	//"errors"
	"fmt"
	"github.com/knative/eventing/pkg/reconciler/cronjobsource/resources"
	"os"
	"testing"

	fakeclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/reconciler"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/knative/eventing/pkg/reconciler/testing"
	. "github.com/knative/pkg/reconciler/testing"

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	_ = v1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)

	_ = os.Setenv("CRONJOB_RA_IMAGE", "no-op")
}

func TestAllCases(t *testing.T) {
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
			//}, { // TODO: there is a bug in the controller, it will query for ""
			//	Name: "incomplete subscription",
			//	Objects: []runtime.Object{
			//		NewSubscription(subscriptionName, testNS),
			//	},
			//	Key:     "foo/incomplete",
			//	WantErr: true,
			//	WantEvents: []string{
			//		Eventf(corev1.EventTypeWarning, "ChannelReferenceFetchFailed", "Failed to validate spec.channel exists: s \"\" not found"),
			//	},
		},
	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(listers *Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:             reconciler.NewBase(opt, controllerAgentName),
			cronjobLister:    listers.GetCronJobSourceLister(),
			deploymentLister: listers.GetDeploymentLister(),
		}
	}))

}

func TestNew(t *testing.T) {
	defer logtesting.ClearAll()
	kubeClient := fakekubeclientset.NewSimpleClientset()
	eventingClient := fakeclientset.NewSimpleClientset()
	eventingInformer := informers.NewSharedInformerFactory(eventingClient, 0)
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, 0)

	cronjobInformer := eventingInformer.Sources().V1alpha1().CronJobSources()
	deploymentInformer := kubeInformer.Apps().V1().Deployments()

	c := NewController(reconciler.Options{
		KubeClientSet:     kubeClient,
		EventingClientSet: eventingClient,
		Logger:            logtesting.TestLogger(t),
	},
		cronjobInformer,
		deploymentInformer,
	)

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
}

//
//Name: "not a Cron Job source",
//			Name: "invalid schedule",
//			Name: "cannot get sinkURI",
//
//			Name: "cannot create receive adapter",
//			Name: "cannot list deployments",
//			Name: "successful create",
//			Name: "successful create - reuse existing receive adapter",
//

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
			Labels:    resources.Labels(getSource().Name),
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

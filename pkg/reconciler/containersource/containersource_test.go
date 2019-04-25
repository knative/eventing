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

package containersource

import (
	"fmt"
	"github.com/knative/eventing/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"testing"

	clientgotesting "k8s.io/client-go/testing"

	fakeclientset "github.com/knative/eventing/pkg/client/clientset/versioned/fake"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"github.com/knative/eventing/pkg/reconciler"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	logtesting "github.com/knative/pkg/logging/testing"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"

	. "github.com/knative/eventing/pkg/reconciler/testing"
	. "github.com/knative/pkg/reconciler/testing"

	sourcesv1alpha1 "github.com/knative/eventing/pkg/apis/sources/v1alpha1"
	v1 "k8s.io/api/apps/v1"
)

const (
	image      = "github.com/knative/test/image"
	sourceName = "test-container-source"
	sourceUID  = "1234-5678-90"
	testNS     = "testnamespace"
	sinkName   = "testsink"
)

var (
	trueVal   = true
	targetURI = "http://addressable.sink.svc.cluster.local/"

	sinkRef = corev1.ObjectReference{
		Name:       sinkName,
		Kind:       "Channel",
		APIVersion: "eventing.knative.dev/v1alpha1",
	}
	nonsinkRef = corev1.ObjectReference{
		Name:       sinkName,
		Kind:       "Trigger",
		APIVersion: "eventing.knative.dev/v1alpha1",
	}
	sinkDNS = "sink.mynamespace.svc." + utils.GetClusterDomainName()
	sinkURI = "http://" + sinkDNS + "/"
)

//
//const (
//	containerSourceName = "testcontainersource"
//	containerSourceUID  = "2a2208d1-ce67-11e8-b3a3-42010a8a00af"
//	deployGeneratedName = "" //sad trombone
//
//	addressableDNS = "addressable.sink.svc.cluster.local"
//
//	addressableName       = "testsink"
//	addressableKind       = "Sink"
//	addressableAPIVersion = "duck.knative.dev/v1alpha1"
//
//	unaddressableName       = "testunaddressable"
//	unaddressableKind       = "KResource"
//	unaddressableAPIVersion = "duck.knative.dev/v1alpha1"
//
//	sinkServiceName       = "testsinkservice"
//	sinkServiceKind       = "Service"
//	sinkServiceAPIVersion = "v1"
//)

func init() {
	// Add types to scheme
	_ = v1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = duckv1alpha1.AddToScheme(scheme.Scheme)
}

func TestNew(t *testing.T) {
	defer logtesting.ClearAll()
	kubeClient := fakekubeclientset.NewSimpleClientset()
	eventingClient := fakeclientset.NewSimpleClientset()
	eventingInformer := informers.NewSharedInformerFactory(eventingClient, 0)
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, 0)

	containerSourceInformer := eventingInformer.Sources().V1alpha1().ContainerSources()
	deploymentInformer := kubeInformer.Apps().V1().Deployments()

	c := NewController(reconciler.Options{
		KubeClientSet:     kubeClient,
		EventingClientSet: eventingClient,
		Logger:            logtesting.TestLogger(t),
	},
		containerSourceInformer,
		deploymentInformer,
	)

	if c == nil {
		t.Fatal("Expected NewController to return a non-nil value")
	}
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
		}, {
			Name: "missing sink",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
				),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SetSinkURIFailed", `Failed to set Sink URI: channels.eventing.knative.dev "testsink" not found`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSinkNotFound(`Couldn't get Sink URI from "/testsink": channels.eventing.knative.dev "testsink" not found"`),
				),
			}},
		}, {
			Name: "sink not addressable",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &nonsinkRef,
					}),
				),
				NewTrigger(sinkName, testNS, ""),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SetSinkURIFailed", `Failed to set Sink URI: sink "testnamespace/testsink" (eventing.knative.dev/v1alpha1, Kind=Trigger) does not contain address`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &nonsinkRef,
					}),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSinkNotFound(`Couldn't get Sink URI from "/testsink": sink "testnamespace/testsink" (eventing.knative.dev/v1alpha1, Kind=Trigger) does not contain address"`),
				),
			}},
		}, {
			Name: "sink not ready",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
				),
				NewChannel(sinkName, testNS),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SetSinkURIFailed", `Failed to set Sink URI: sink "testnamespace/testsink" (eventing.knative.dev/v1alpha1, Kind=Channel) contains an empty hostname`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSinkNotFound(`Couldn't get Sink URI from "/testsink": sink "testnamespace/testsink" (eventing.knative.dev/v1alpha1, Kind=Channel) contains an empty hostname"`),
				),
			}},
		}, {
			Name: "sink is nil",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
					}),
				),
			},
			Key:     testNS + "/" + sourceName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "SetSinkURIFailed", `Failed to set Sink URI: Sink missing from spec`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
					}),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSinkMissing("Sink missing from spec"),
				),
			}},
		}, {
			Name: "valid",
			Objects: []runtime.Object{
				NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
				),
				NewChannel(sinkName, testNS,
					WithChannelAddress(sinkDNS),
				),
			},
			Key: testNS + "/" + sourceName,
			WantEvents: []string{
				Eventf(corev1.EventTypeNormal, "DeploymentCreated", `Created deployment ""`), // TODO on noes
				Eventf(corev1.EventTypeNormal, "ContainerSourceReconciled", `ContainerSource reconciled: "testnamespace/test-container-source"`),
			},
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
				Object: NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
						Sink:  &sinkRef,
					}),
					// Status Update:
					WithInitContainerSourceConditions,
					WithContainerSourceSink(sinkURI),
					WithContainerSourceDeploying(`Created deployment ""`),
				),
			}},
			WantCreates: []metav1.Object{
				makeDeployment(NewContainerSource(sourceName, testNS,
					WithContainerSourceSpec(sourcesv1alpha1.ContainerSourceSpec{
						Image: image,
					}))),
			},
		},

		//Name:       "valid containersource, sink is provided",
		//Name:       "valid containersource, labels and annotations given",
		//Name:       "valid containersource, sink, and deployment",
		//Name:       "valid containersource, sink, but deployment needs update",
		//Name:       "Error for create deployment",
		//Name:       "Error for get source, other than not found",
		//Name:       "valid containersource, sink is a k8s service",

	}

	defer logtesting.ClearAll()
	table.Test(t, MakeFactory(func(listers *Listers, opt reconciler.Options) controller.Reconciler {
		return &Reconciler{
			Base:                  reconciler.NewBase(opt, controllerAgentName),
			containerSourceLister: listers.GetContainerSourceLister(),
			deploymentLister:      listers.GetDeploymentLister(),
		}
	}))

}

//
//func getContainerSource() *sourcesv1alpha1.ContainerSource {
//	obj := &sourcesv1alpha1.ContainerSource{
//		TypeMeta:   containerSourceType(),
//		ObjectMeta: om(testNS, containerSourceName),
//		Spec: sourcesv1alpha1.ContainerSourceSpec{
//			Image: image,
//			Args:  []string(nil),
//			Sink: &corev1.ObjectReference{
//				Name:       addressableName,
//				Kind:       addressableKind,
//				APIVersion: addressableAPIVersion,
//			},
//		},
//	}
//	// selflink is not filled in when we create the object, so clear it
//	obj.ObjectMeta.SelfLink = ""
//	return obj
//}
//
//func getContainerSourceFilledIn() *sourcesv1alpha1.ContainerSource {
//	obj := getContainerSource()
//	obj.ObjectMeta.UID = containerSourceUID
//	obj.Spec.Args = []string{"--foo", "bar"}
//	obj.Spec.Env = []corev1.EnvVar{{Name: "FOO", Value: "bar"}}
//	obj.Spec.ServiceAccountName = "foo"
//	return obj
//}
//
//func getContainerSourceSinkService() *sourcesv1alpha1.ContainerSource {
//	obj := &sourcesv1alpha1.ContainerSource{
//		TypeMeta:   containerSourceType(),
//		ObjectMeta: om(testNS, containerSourceName),
//		Spec: sourcesv1alpha1.ContainerSourceSpec{
//			Image: image,
//			Args:  []string(nil),
//			Sink: &corev1.ObjectReference{
//				Name:       sinkServiceName,
//				Kind:       sinkServiceKind,
//				APIVersion: sinkServiceAPIVersion,
//			},
//		},
//	}
//	// selflink is not filled in when we create the object, so clear it
//	obj.ObjectMeta.SelfLink = ""
//	return obj
//}
//
//func getContainerSourceUnaddressable() *sourcesv1alpha1.ContainerSource {
//	obj := &sourcesv1alpha1.ContainerSource{
//		TypeMeta:   containerSourceType(),
//		ObjectMeta: om(testNS, containerSourceName),
//		Spec: sourcesv1alpha1.ContainerSourceSpec{
//			Image: image,
//			Args:  []string{},
//			Sink: &corev1.ObjectReference{
//				Name:       unaddressableName,
//				Kind:       unaddressableKind,
//				APIVersion: unaddressableAPIVersion,
//			},
//		},
//	}
//	// selflink is not filled in when we create the object, so clear it
//	obj.ObjectMeta.SelfLink = ""
//	return obj
//}
//
//func getAddressable() *unstructured.Unstructured {
//	return &unstructured.Unstructured{
//		Object: map[string]interface{}{
//			"apiVersion": addressableAPIVersion,
//			"kind":       addressableKind,
//			"metadata": map[string]interface{}{
//				"namespace": testNS,
//				"name":      addressableName,
//			},
//			"status": map[string]interface{}{
//				"address": map[string]interface{}{
//					"hostname": addressableDNS,
//				},
//			},
//		},
//	}
//}
//
//func getAddressableNoStatus() *unstructured.Unstructured {
//	return &unstructured.Unstructured{
//		Object: map[string]interface{}{
//			"apiVersion": unaddressableAPIVersion,
//			"kind":       unaddressableKind,
//			"metadata": map[string]interface{}{
//				"namespace": testNS,
//				"name":      unaddressableName,
//			},
//		},
//	}
//}
//
//func getAddressableNilAddress() *unstructured.Unstructured {
//	return &unstructured.Unstructured{
//		Object: map[string]interface{}{
//			"apiVersion": addressableAPIVersion,
//			"kind":       addressableKind,
//			"metadata": map[string]interface{}{
//				"namespace": testNS,
//				"name":      addressableName,
//			},
//			"status": map[string]interface{}{
//				"address": map[string]interface{}(nil),
//			},
//		},
//	}
//}
//
func makeDeployment(source *sourcesv1alpha1.ContainerSource) *appsv1.Deployment {
	args := append(source.Spec.Args, fmt.Sprintf("--sink=%s", sinkURI))
	env := append(source.Spec.Env, corev1.EnvVar{Name: "SINK", Value: sinkURI})
	return &appsv1.Deployment{
		TypeMeta: deploymentType(),
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    fmt.Sprintf("%s-", source.Name),
			Namespace:       source.Namespace,
			OwnerReferences: getOwnerReferences(),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"eventing.knative.dev/source": source.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: map[string]string{
						"eventing.knative.dev/source": source.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "source",
						Image:           source.Spec.Image,
						Args:            args,
						Env:             env,
						ImagePullPolicy: corev1.PullIfNotPresent,
					}},
					ServiceAccountName: source.Spec.ServiceAccountName,
				},
			},
		},
	}
}

//
//func containerSourceType() metav1.TypeMeta {
//	return metav1.TypeMeta{
//		APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
//		Kind:       "ContainerSource",
//	}
//}
//
func deploymentType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: appsv1.SchemeGroupVersion.String(),
		Kind:       "Deployment",
	}
}

//
//func om(namespace, name string) metav1.ObjectMeta {
//	return metav1.ObjectMeta{
//		Namespace: namespace,
//		Name:      name,
//		SelfLink:  fmt.Sprintf("/apis/eventing/sources/v1alpha1/namespaces/%s/object/%s", namespace, name),
//	}
//}
//
func getOwnerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         sourcesv1alpha1.SchemeGroupVersion.String(),
		Kind:               "ContainerSource",
		Name:               sourceName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
		UID:                sourceUID,
	}}
}

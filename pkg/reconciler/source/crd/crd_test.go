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

package crd

import (
	"context"
	"testing"

	"knative.dev/eventing/pkg/reconciler/source/duck"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/apis/sources"
	crdreconciler "knative.dev/pkg/client/injection/apiextensions/reconciler/apiextensions/v1/customresourcedefinition"
	"knative.dev/pkg/client/injection/ducks/duck/v1/source"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	. "knative.dev/eventing/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"

	// Fake injection informers
	_ "knative.dev/eventing/pkg/client/injection/informers/eventing/v1beta1/eventtype/fake"
	fakeclient "knative.dev/pkg/client/injection/apiextensions/client/fake"
	_ "knative.dev/pkg/client/injection/apiextensions/informers/apiextensions/v1/customresourcedefinition/fake"
	_ "knative.dev/pkg/client/injection/ducks/duck/v1/source/fake"
)

const (
	crdName = "test-crd"

	crdGroup         = "testing.sources.knative.dev"
	crdKind          = "TestSource"
	crdPlural        = "testsources"
	crdVersionServed = "v1alpha1"
)

var crdGVR = schema.GroupVersionResource{
	Group:    crdGroup,
	Version:  crdVersionServed,
	Resource: crdPlural,
}

var crdGVK = schema.GroupVersionKind{
	Group:   crdGroup,
	Version: crdVersionServed,
	Kind:    crdKind,
}

func TestAllCases(t *testing.T) {
	ctx := context.Background()
	ctx, _ = injection.Fake.SetupInformers(ctx, &rest.Config{})
	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		}, {
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "not-found",
		}, {
			Name: "reconcile failed, cannot find GVR or GVK",
			Objects: []runtime.Object{
				NewCustomResourceDefinition(crdName,
					WithCustomResourceDefinitionLabels(map[string]string{
						sources.SourceDuckLabelKey: sources.SourceDuckLabelValue,
					})),
			},
			Key:     crdName,
			WantErr: true,
			WantEvents: []string{
				Eventf(corev1.EventTypeWarning, "InternalError", "unable to find GVR or GVK for %s", crdName),
			},
		},
		{

			Name: "reconcile succeeded",
			Objects: []runtime.Object{
				NewCustomResourceDefinition(crdName,
					WithCustomResourceDefinitionLabels(map[string]string{
						sources.SourceDuckLabelKey: sources.SourceDuckLabelValue,
					}),
					WithCustomResourceDefinitionGroup(crdGroup),
					WithCustomResourceDefinitionNames(apiextensionsv1.CustomResourceDefinitionNames{
						Kind:   crdKind,
						Plural: crdPlural,
					}),
					WithCustomResourceDefinitionVersions([]apiextensionsv1.CustomResourceDefinitionVersion{{
						Name:   crdVersionServed,
						Served: true,
					}})),
			},
			Key: crdName,
			Ctx: ctx,
		},
		{
			Name: "reconcile deleted",
			Objects: []runtime.Object{
				NewCustomResourceDefinition(crdName,
					WithCustomResourceDefinitionLabels(map[string]string{
						sources.SourceDuckLabelKey: sources.SourceDuckLabelValue,
					}),

					WithCustomResourceDefinitionGroup(crdGroup),
					WithCustomResourceDefinitionDeletionTimestamp(),
					WithCustomResourceDefinitionNames(apiextensionsv1.CustomResourceDefinitionNames{
						Kind:   crdKind,
						Plural: crdPlural,
					}),
					WithCustomResourceDefinitionVersions([]apiextensionsv1.CustomResourceDefinitionVersion{{
						Name:   crdVersionServed,
						Served: true,
					}})),
			},
			Key: crdName,
			Ctx: ctx,
		},
		{
			Name: "reconcile not Served",
			Objects: []runtime.Object{
				NewCustomResourceDefinition(crdName,
					WithCustomResourceDefinitionLabels(map[string]string{
						sources.SourceDuckLabelKey: sources.SourceDuckLabelValue,
					}),

					WithCustomResourceDefinitionGroup(crdGroup),
					WithCustomResourceDefinitionDeletionTimestamp(),
					WithCustomResourceDefinitionNames(apiextensionsv1.CustomResourceDefinitionNames{
						Kind:   crdKind,
						Plural: crdPlural,
					}),
					WithCustomResourceDefinitionVersions([]apiextensionsv1.CustomResourceDefinitionVersion{{
						Name:   crdVersionServed,
						Served: false,
					}})),
			},
			Key: crdName,
			Ctx: ctx,
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = source.WithDuck(ctx)
		r := &Reconciler{
			ogctx:       ctx,
			ogcmw:       cmw,
			controllers: make(map[schema.GroupVersionResource]runningController),
		}

		return crdreconciler.NewReconciler(ctx, logger,
			fakeclient.Get(ctx), listers.GetCustomResourceDefinitionLister(),
			controller.GetEventRecorder(ctx), r)
	}, false, logger))
}

func TestControllerRunning(t *testing.T) {
	ctx := context.Background()
	ctx, _ = injection.Fake.SetupInformers(ctx, &rest.Config{})
	table := TableTest{
		{

			Name: "reconcile succeeded",
			Objects: []runtime.Object{
				NewCustomResourceDefinition(crdName,
					WithCustomResourceDefinitionLabels(map[string]string{
						sources.SourceDuckLabelKey: sources.SourceDuckLabelValue,
					}),
					WithCustomResourceDefinitionGroup(crdGroup),
					WithCustomResourceDefinitionNames(apiextensionsv1.CustomResourceDefinitionNames{
						Kind:   crdKind,
						Plural: crdPlural,
					}),
					WithCustomResourceDefinitionVersions([]apiextensionsv1.CustomResourceDefinitionVersion{{
						Name:   crdVersionServed,
						Served: true,
					}})),
			},
			Key: crdName,
			Ctx: ctx,
		},
		{
			Name: "reconcile deleted",
			Objects: []runtime.Object{
				NewCustomResourceDefinition(crdName,
					WithCustomResourceDefinitionLabels(map[string]string{
						sources.SourceDuckLabelKey: sources.SourceDuckLabelValue,
					}),

					WithCustomResourceDefinitionGroup(crdGroup),
					WithCustomResourceDefinitionDeletionTimestamp(),
					WithCustomResourceDefinitionNames(apiextensionsv1.CustomResourceDefinitionNames{
						Kind:   crdKind,
						Plural: crdPlural,
					}),
					WithCustomResourceDefinitionVersions([]apiextensionsv1.CustomResourceDefinitionVersion{{
						Name:   crdVersionServed,
						Served: true,
					}})),
			},
			Key: crdName,
			Ctx: ctx,
		},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = source.WithDuck(ctx)
		r := &Reconciler{
			ogctx:       ctx,
			ogcmw:       cmw,
			controllers: make(map[schema.GroupVersionResource]runningController),
		}
		sdc := duck.NewController(crdName, crdGVR, crdGVK)
		// Source Duck controller context
		sdctx, cancel := context.WithCancel(r.ogctx)
		// Source Duck controller instantiation
		sd := sdc(sdctx, r.ogcmw)
		rc := runningController{
			controller: sd,
			cancel:     cancel,
		}
		r.controllers[crdGVR] = rc

		return crdreconciler.NewReconciler(ctx, logger,
			fakeclient.Get(ctx), listers.GetCustomResourceDefinitionLister(),
			controller.GetEventRecorder(ctx), r)
	}, false, logger))
}

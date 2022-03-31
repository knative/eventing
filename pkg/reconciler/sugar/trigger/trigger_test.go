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

package trigger

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	sugarconfig "knative.dev/eventing/pkg/apis/sugar"
	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"
	"knative.dev/eventing/pkg/client/injection/reconciler/eventing/v1/trigger"
	"knative.dev/eventing/pkg/reconciler/sugar/resources"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"

	. "knative.dev/eventing/pkg/reconciler/testing/v1"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	testNS      = "test-namespace"
	triggerName = "test-trigger"
	brokerName  = "default"

	SomeLabelKey        = "eventing.knative.dev/somekey"
	SomeLabelValue      = "someValue"
	SomeOtherLabelValue = "someOtherValue"

	LegacyInjectionLabelKey           = "eventing.knative.dev/injection"
	LegacyInjectionEnabledLabelValue  = "enabled"
	LegacyInjectionDisabledLabelValue = "disabled"
)

type key int

var (
	sugarConfigContextKey key
)

type testConfigStore struct {
	config *sugarconfig.Config
}

func (t *testConfigStore) ToContext(ctx context.Context) context.Context {
	return sugarconfig.ToContext(ctx, t.config)
}

func TestEnabled(t *testing.T) {
	// Events
	brokerEvent := Eventf(corev1.EventTypeNormal, "BrokerCreated", "Default eventing.knative.dev Broker %q created.", "default")

	// Objects
	broker := resources.MakeBroker(testNS, resources.DefaultBrokerName)

	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "Enabled for all triggers",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS, brokerName),
		},
		Key:                     testNS + "/" + triggerName,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			brokerEvent,
		},
		WantCreates: []runtime.Object{
			broker,
		},
		Ctx: context.WithValue(context.Background(), sugarConfigContextKey,
			&metav1.LabelSelector{}),
	}, {
		Name: "Labelled namespace with expected `key` and `value`",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS, brokerName, WithLabel(SomeLabelKey, SomeLabelValue)),
		},
		Key:                     testNS + "/" + triggerName,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			brokerEvent,
		},
		WantCreates: []runtime.Object{
			broker,
		},
		Ctx: context.WithValue(context.Background(), sugarConfigContextKey,
			&metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      SomeLabelKey,
					Operator: "In",
					Values:   []string{SomeLabelValue},
				}}}),
	}, {
		Name: "Labelled namespace with expected LegacyInjectionKey and LegacyInjectionEnabledValue",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS, brokerName, WithLabel(LegacyInjectionLabelKey, LegacyInjectionEnabledLabelValue)),
		},
		Key:                     testNS + "/" + triggerName,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		WantEvents: []string{
			brokerEvent,
		},
		WantCreates: []runtime.Object{
			broker,
		},
		Ctx: context.WithValue(context.Background(), sugarConfigContextKey,
			&metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      LegacyInjectionLabelKey,
					Operator: "In",
					Values:   []string{LegacyInjectionEnabledLabelValue},
				}}}),
	}, {
		Name: "Trigger is deleted no resources",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS, brokerName,
				WithTriggerDeleted),
		},
		Key: testNS + "/" + triggerName,
		Ctx: context.WithValue(context.Background(), sugarConfigContextKey,
			&metav1.LabelSelector{}),
	}, {
		Name: "Trigger enabled, broker exists",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS, brokerName),
			resources.MakeBroker(testNS, resources.DefaultBrokerName),
		},
		Key:                     testNS + "/" + triggerName,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		Ctx: context.WithValue(context.Background(), sugarConfigContextKey,
			&metav1.LabelSelector{}),
	}, {
		Name: "Trigger enabled, broker exists with no label",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS, brokerName),
			&v1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNS,
					Name:      resources.DefaultBrokerName,
				},
			},
		},
		Key:                     testNS + "/" + triggerName,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		Ctx: context.WithValue(context.Background(), sugarConfigContextKey,
			&metav1.LabelSelector{}),
	},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			eventingClientSet: fakeeventingclient.Get(ctx),
			brokerLister:      listers.GetBrokerLister(),
		}

		sugarCfg := &sugarconfig.Config{}
		if ls, ok := ctx.Value(sugarConfigContextKey).(*metav1.LabelSelector); ok && ls != nil {
			sugarCfg.TriggerSelector = ls
		}

		return trigger.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetTriggerLister(),
			controller.GetEventRecorder(ctx), r, controller.Options{
				SkipStatusUpdates: true,
				ConfigStore: &testConfigStore{
					config: sugarCfg,
				},
			})
	}, false, logger))
}

func TestDisabled(t *testing.T) {
	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "foo/not-found",
	}, {
		Name: "Disabled by default",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS, brokerName),
		},
		Key:                     testNS + "/" + triggerName,
		SkipNamespaceValidation: true,
		WantErr:                 false,
	}, {
		Name: "Labelled trigger with expected `key` but different `value`",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS, brokerName, WithLabel(SomeLabelKey, SomeOtherLabelValue)),
		},
		Key:                     testNS + "/" + triggerName,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		Ctx: context.WithValue(context.Background(), sugarConfigContextKey,
			&metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      SomeLabelKey,
					Operator: "In",
					Values:   []string{SomeLabelValue},
				}}}),
	}, {
		Name: "Labelled trigger with expected LegacyInjectionKey and LegacyInjectionDisabledLabelValue",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS, brokerName, WithLabel(LegacyInjectionLabelKey, LegacyInjectionDisabledLabelValue)),
		},
		Key:                     testNS + "/" + triggerName,
		SkipNamespaceValidation: true,
		WantErr:                 false,
		Ctx: context.WithValue(context.Background(), sugarConfigContextKey,
			&metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      LegacyInjectionLabelKey,
					Operator: "In",
					Values:   []string{LegacyInjectionEnabledLabelValue},
				}}}),
	}, {
		Name: "Trigger is deleted no resources",
		Objects: []runtime.Object{
			NewTrigger(triggerName, testNS, brokerName,
				WithTriggerDeleted),
		},
		Key: testNS + "/" + triggerName,
	},
	}

	logger := logtesting.TestLogger(t)
	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			eventingClientSet: fakeeventingclient.Get(ctx),
			brokerLister:      listers.GetBrokerLister(),
		}

		sugarCfg := &sugarconfig.Config{}
		if ls, ok := ctx.Value(sugarConfigContextKey).(*metav1.LabelSelector); ok && ls != nil {
			sugarCfg.TriggerSelector = ls
		}

		return trigger.NewReconciler(ctx, logger,
			fakeeventingclient.Get(ctx), listers.GetTriggerLister(),
			controller.GetEventRecorder(ctx), r, controller.Options{
				SkipStatusUpdates: true,
				ConfigStore: &testConfigStore{
					config: sugarCfg,
				},
			})
	}, false, logger))
}

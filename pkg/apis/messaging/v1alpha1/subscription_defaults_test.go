/*
Copyright 2018 The Knative Authors

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

package v1alpha1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	testNS = "testnamespace"
)

// No-op test because method does nothing.
func TestSubscriptionDefaultsEmpty(t *testing.T) {
	s := Subscription{}
	s.SetDefaults(context.Background())
}

func TestDefaultSubscriber(t *testing.T) {
	s := Subscription{ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
		Spec: SubscriptionSpec{
			Subscriber: &duckv1.Destination{Ref: &duckv1.KReference{}},
		},
	}
	s.SetDefaults(context.Background())
	if s.Spec.Subscriber.Ref.Namespace != testNS {
		t.Errorf("Got: %s wanted %s", s.Spec.Subscriber.Ref.Namespace, testNS)
	}
}

func TestDefaultReply(t *testing.T) {
	s := Subscription{ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
		Spec: SubscriptionSpec{
			Reply: &duckv1.Destination{Ref: &duckv1.KReference{}},
		},
	}
	s.SetDefaults(context.Background())
	if s.Spec.Reply.Ref.Namespace != testNS {
		t.Errorf("Got: %s wanted %s", s.Spec.Reply.Ref.Namespace, testNS)
	}
}

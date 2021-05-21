/*
Copyright 2021 The Knative Authors

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

package prober_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	fakeeventingclientset "knative.dev/eventing/pkg/client/clientset/versioned/fake"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/upgrade/prober"
	pkgtest "knative.dev/pkg/test"
)

func TestCustomConfigTemplate(t *testing.T) {
	config := prober.NewConfig("prober-unittest")
	ctx := context.TODO()
	log := zap.NewNop().Sugar()
	client := newTestClient(config, t)
	p := prober.RunEventProber(ctx, log, client, config)
	assert.NotNil(t, p)
}

func newTestClient(c *prober.Config, t *testing.T) *testlib.Client {
	fc := fakekubeclientset.NewSimpleClientset()
	eventing := fakeeventingclientset.NewSimpleClientset()
	sc := runtime.NewScheme()
	_ = corev1.AddToScheme(sc)
	dynamic := fakedynamic.NewSimpleDynamicClient(sc)
	cl := &testlib.Client{
		Namespace: c.Namespace,
		Kube: &pkgtest.KubeClient{
			Interface: fc,
		},
		Eventing: eventing,
		Dynamic:  dynamic,
		T:        t,
		Tracker:  testlib.NewTracker(t, dynamic),
	}
	return cl
}

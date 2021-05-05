// +build e2e

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

package sut_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/upgrade/prober/sut"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestNewDefaultE2E(t *testing.T) {
	client := testlib.Setup(t, false)
	defer testlib.TearDown(client)
	s := sut.NewDefault(client.Namespace)
	ctx := sut.Context{
		Ctx:    context.Background(),
		Log:    log(t),
		Client: client,
	}
	// create event logger pod and service as the subscriber
	pod := recordevents.DeployEventRecordOrFail(ctx.Ctx, client, "record")
	url, err := s.Deploy(ctx, duckv1.Destination{
		Ref: &duckv1.KReference{
			Kind:       pod.Kind,
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			APIVersion: pod.APIVersion,
		},
	})
	defer func() {
		err := s.Teardown(ctx)
		assert.NoError(t, err)
	}()
	assert.NoError(t, err)
	assert.NotEmpty(t, url)
}

func log(t *testing.T) *zap.SugaredLogger {
	l, err := zap.NewDevelopment()
	if err != nil {
		t.Fatal(err)
	}
	return l.Sugar()
}

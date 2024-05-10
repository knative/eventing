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
package apiserver

import (
	"testing"

	"go.uber.org/zap"
	adaptertest "knative.dev/eventing/pkg/adapter/v2/test"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/sources"
	brokerfilter "knative.dev/eventing/pkg/broker/filter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
)

func TestResourceAddEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Add(simplePod("unit", "test"))
	validateSent(t, ce, sources.ApiServerSourceAddEventType)
}

func TestResourceUpdateEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Update(simplePod("unit", "test"))
	validateSent(t, ce, sources.ApiServerSourceUpdateEventType)
}

func TestResourceDeleteEvent(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Delete(simplePod("unit", "test"))
	validateSent(t, ce, sources.ApiServerSourceDeleteEventType)
}

func TestResourceAddEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Add(nil)
	validateNotSent(t, ce, sources.ApiServerSourceAddEventType)
}

func TestResourceUpdateEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Update(nil)
	validateNotSent(t, ce, sources.ApiServerSourceUpdateEventType)
}

func TestResourceDeleteEventNil(t *testing.T) {
	d, ce := makeResourceAndTestingClient()
	d.Delete(nil)
	validateNotSent(t, ce, sources.ApiServerSourceDeleteEventType)
}

// HACKHACKHACK For test coverage.
func TestResourceStub(t *testing.T) {
	d, _ := makeResourceAndTestingClient()

	d.List()
	d.ListKeys()
	d.Get(nil)
	d.GetByKey("")
	d.Replace(nil, "")
	d.Resync()
}

func TestFilterFails(t *testing.T) {
	ce := adaptertest.NewTestClient()
	filters := []eventingv1.SubscriptionsAPIFilter{{
		Exact: map[string]string{
			"type": "dev.knative.apiserver.resource.add",
		},
	}}
	logger := zap.NewExample().Sugar()
	delegate := &resourceDelegate{
		ce:                  ce,
		source:              "unit-test",
		apiServerSourceName: apiServerSourceNameTest,
		logger:              logger,
		filter:              subscriptionsapi.NewAllFilter(brokerfilter.MaterializeFiltersList(logger.Desugar(), filters)...),
	}

	delegate.Update(simplePod("unit", "test"))
	validateNotSent(t, ce, sources.ApiServerSourceUpdateEventType)
}

func TestEmptyFiltersList(t *testing.T) {
	ce := adaptertest.NewTestClient()
	filters := []eventingv1.SubscriptionsAPIFilter{}

	logger := zap.NewExample().Sugar()
	delegate := &resourceDelegate{
		ce:                  ce,
		source:              "unit-test",
		apiServerSourceName: apiServerSourceNameTest,
		logger:              logger,
		filter:              subscriptionsapi.NewAllFilter(brokerfilter.MaterializeFiltersList(logger.Desugar(), filters)...),
	}

	delegate.Update(simplePod("unit", "test"))
	validateSent(t, ce, sources.ApiServerSourceUpdateEventType)
}

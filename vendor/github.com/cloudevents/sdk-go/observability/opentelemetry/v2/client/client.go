/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package client

import (
	obshttp "github.com/cloudevents/sdk-go/observability/opentelemetry/v2/http"
	"github.com/cloudevents/sdk-go/v2/client"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
)

// NewClientHTTP produces a new client instrumented with OpenTelemetry.
func NewClientHTTP(topt []cehttp.Option, copt []client.Option, obsOpts ...OTelObservabilityServiceOption) (client.Client, error) {
	t, err := obshttp.NewObservedHTTP(topt...)
	if err != nil {
		return nil, err
	}

	copt = append(
		copt,
		client.WithTimeNow(),
		client.WithUUIDs(),
		client.WithObservabilityService(NewOTelObservabilityService(obsOpts...)),
	)

	c, err := client.New(t, copt...)
	if err != nil {
		return nil, err
	}

	return c, nil
}

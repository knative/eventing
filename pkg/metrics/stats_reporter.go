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

package metrics

import (
	"context"
	"log"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics"
)

type MetricArgs interface {
	GenerateTag(tags ...tag.Mutator) (context.Context, error)
}

func init() {
	Register([]stats.Measure{}, nil)
}

// StatsReporter defines the interface for sending filter metrics.
type StatsReporter interface {
}

func Register(customMetrics []stats.Measure, customViews []*view.View, customTagKeys ...tag.Key) {
	allTagKeys := append(customTagKeys)
	allViews := append(customViews)

	// Add custom views for custom metrics
	for _, metric := range customMetrics {
		allViews = append(allViews, &view.View{
			Description: metric.Description(),
			Name:        metric.Name(),
			Measure:     metric,
			Aggregation: view.LastValue(),
			TagKeys:     allTagKeys,
		})
	}

	// Append custom views
	if err := metrics.RegisterResourceView(allViews...); err != nil {
		log.Printf("Failed to register opencensus views: %v", err)
	}
}

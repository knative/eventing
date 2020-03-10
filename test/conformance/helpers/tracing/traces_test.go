/*
Copyright 2020 Google LLC.

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

package tracing

import (
	"testing"

	"github.com/openzipkin/zipkin-go/model"
)

type SpanCase struct {
	Span        *model.SpanModel
	ShouldMatch bool
}

func TestSpanMatcher(t *testing.T) {
	serverKind := model.Server
	tcs := []struct {
		Name    string
		Matcher *SpanMatcher
		Spans   []SpanCase
	}{
		{
			Name:    "empty matcher",
			Matcher: &SpanMatcher{},
			Spans: []SpanCase{{
				Span:        &model.SpanModel{},
				ShouldMatch: true,
			}, {
				Span: &model.SpanModel{
					Kind: model.Server,
					LocalEndpoint: &model.Endpoint{
						ServiceName: "test-service-name",
					},
					Tags: map[string]string{
						"test-tag":  "test-tag-value",
						"other-tag": "other-tag-value",
					},
				},
				ShouldMatch: true,
			}},
		},
		{
			Name: "kind matcher",
			Matcher: &SpanMatcher{
				Kind: &serverKind,
			},
			Spans: []SpanCase{{
				Span:        &model.SpanModel{},
				ShouldMatch: false,
			}, {
				Span: &model.SpanModel{
					Kind: model.Server,
				},
				ShouldMatch: true,
			}, {
				Span: &model.SpanModel{
					Kind: model.Client,
				},
				ShouldMatch: false,
			}, {
				Span: &model.SpanModel{
					Kind: model.Server,
					LocalEndpoint: &model.Endpoint{
						ServiceName: "test-service-name",
					},
					Tags: map[string]string{
						"test-tag":  "test-tag-value",
						"other-tag": "other-tag-value",
					},
				},
				ShouldMatch: true,
			}},
		},
		{
			Name: "local endpoint service name matcher",
			Matcher: &SpanMatcher{
				LocalEndpointServiceName: "test-service-name",
			},
			Spans: []SpanCase{{
				Span:        &model.SpanModel{},
				ShouldMatch: false,
			}, {
				Span: &model.SpanModel{
					LocalEndpoint: &model.Endpoint{
						ServiceName: "test-service-name",
					},
				},
				ShouldMatch: true,
			}, {
				Span: &model.SpanModel{
					LocalEndpoint: &model.Endpoint{
						ServiceName: "other-service-name",
					},
				},
				ShouldMatch: false,
			}, {
				Span: &model.SpanModel{
					Kind: model.Server,
					LocalEndpoint: &model.Endpoint{
						ServiceName: "test-service-name",
					},
					Tags: map[string]string{
						"test-tag":  "test-tag-value",
						"other-tag": "other-tag-value",
					},
				},
				ShouldMatch: true,
			}},
		},
		{
			Name: "tag matcher",
			Matcher: &SpanMatcher{
				Tags: map[string]string{
					"test-tag": "test-tag-value",
				},
			},
			Spans: []SpanCase{{
				Span:        &model.SpanModel{},
				ShouldMatch: false,
			}, {
				Span: &model.SpanModel{
					Tags: map[string]string{
						"test-tag": "test-tag-value",
					},
				},
				ShouldMatch: true,
			}, {
				Span: &model.SpanModel{
					Tags: map[string]string{
						"test-tag": "other-tag-value",
					},
				},
				ShouldMatch: false,
			}, {
				Span: &model.SpanModel{
					Tags: map[string]string{
						"other-tag": "test-tag-value",
					},
				},
				ShouldMatch: false,
			}, {
				Span: &model.SpanModel{
					Kind: model.Server,
					LocalEndpoint: &model.Endpoint{
						ServiceName: "test-service-name",
					},
					Tags: map[string]string{
						"test-tag":  "test-tag-value",
						"other-tag": "other-tag-value",
					},
				},
				ShouldMatch: true,
			}},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			for _, sc := range tc.Spans {
				if err := tc.Matcher.MatchesSpan(sc.Span); err != nil && sc.ShouldMatch {
					t.Errorf("expected matcher %v to match span %v but got %v", tc.Matcher, sc.Span, err)
				} else if err == nil && !sc.ShouldMatch {
					t.Errorf("expected matcher %v not to match span %v but got match", tc.Matcher, sc.Span)
				}

			}
		})
	}
}

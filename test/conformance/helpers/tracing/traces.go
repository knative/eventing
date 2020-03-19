/*
Copyright 2019 The Knative Authors

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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/openzipkin/zipkin-go/model"
)

// PrettyPrintTrace pretty prints a Trace.
func PrettyPrintTrace(trace []model.SpanModel) string {
	b, _ := json.Marshal(trace)
	return string(b)
}

// SpanTree is the tree of Spans representation of a Trace.
type SpanTree struct {
	Root     bool
	Span     model.SpanModel
	Children []SpanTree
}

func (t SpanTree) String() string {
	b, _ := json.MarshalIndent(t, "", "  ")
	return string(b)
}

type SpanMatcher struct {
	Kind                     *model.Kind       `json:"a_Kind,omitempty"`
	LocalEndpointServiceName string            `json:"b_Name,omitempty"`
	Tags                     map[string]string `json:"c_Tags,omitempty"`
}

func (m *SpanMatcher) Cmp(m2 *SpanMatcher) int {
	if m == nil {
		if m2 == nil {
			return 0
		}
		return -1
	}
	if m2 == nil {
		return 1
	}

	if *m.Kind < *m2.Kind {
		return -1
	} else if *m.Kind > *m2.Kind {
		return 1
	}

	t1 := m.Tags
	t2 := m2.Tags
	for _, key := range []string{"http.url", "http.host", "http.path"} {
		if t1[key] < t2[key] {
			return -1
		} else if t1[key] > t2[key] {
			return 1
		}
	}
	return 0
}

type SpanMatcherOption func(*SpanMatcher)

func WithLocalEndpointServiceName(s string) SpanMatcherOption {
	return func(m *SpanMatcher) {
		m.LocalEndpointServiceName = s
	}
}

func (m *SpanMatcher) MatchesSpan(span *model.SpanModel) error {
	if m == nil {
		return nil
	}
	if m.Kind != nil {
		if *m.Kind != span.Kind {
			return fmt.Errorf("mismatched kind: got %q, want %q", span.Kind, *m.Kind)
		}
	}
	if m.LocalEndpointServiceName != "" {
		if span.LocalEndpoint == nil {
			return errors.New("missing local endpoint")
		}
		if m.LocalEndpointServiceName != span.LocalEndpoint.ServiceName {
			return fmt.Errorf("mismatched LocalEndpoint ServiceName: got %q, want %q", span.LocalEndpoint.ServiceName, m.LocalEndpointServiceName)
		}
	}
	for k, v := range m.Tags {
		if t := span.Tags[k]; t != v {
			return fmt.Errorf("unexpected tag %s: got %q, want %q", k, t, v)
		}
	}
	return nil
}

func MatchHTTPClientSpanWithCode(host string, path string, statusCode int, opts ...SpanMatcherOption) *SpanMatcher {
	kind := model.Client
	m := &SpanMatcher{
		Kind: &kind,
		Tags: map[string]string{
			"http.method":      http.MethodPost,
			"http.status_code": strconv.Itoa(statusCode),
			"http.url":         fmt.Sprintf("http://%s%s", host, path),
		},
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func MatchHTTPServerSpanWithCode(host string, path string, statusCode int, opts ...SpanMatcherOption) *SpanMatcher {
	kind := model.Server
	m := &SpanMatcher{
		Kind: &kind,
		Tags: map[string]string{
			"http.method":      http.MethodPost,
			"http.status_code": strconv.Itoa(statusCode),
			"http.host":        host,
			"http.path":        path,
		},
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func MatchHTTPClientSpanNoReply(host string, path string, opts ...SpanMatcherOption) *SpanMatcher {
	return MatchHTTPClientSpanWithCode(host, path, 202, opts...)
}

func MatchHTTPServerSpanNoReply(host string, path string, opts ...SpanMatcherOption) *SpanMatcher {
	return MatchHTTPServerSpanWithCode(host, path, 202, opts...)
}

func MatchHTTPClientSpanWithReply(host string, path string, opts ...SpanMatcherOption) *SpanMatcher {
	return MatchHTTPClientSpanWithCode(host, path, 200, opts...)
}

func MatchHTTPServerSpanWithReply(host string, path string, opts ...SpanMatcherOption) *SpanMatcher {
	return MatchHTTPServerSpanWithCode(host, path, 200, opts...)
}

// TestSpanTree is the expected version of SpanTree used for assertions in testing.
//
// The JSON names of the fields are weird because we want a specific order when pretty printing
// JSON. The JSON will be printed in alphabetical order, so we are imposing a certain order by
// prefixing the keys with a specific letter. The letter has no mean other than ordering.
type TestSpanTree struct {
	Note     string         `json:"a_Note,omitempty"`
	Root     bool           `json:"b_root"`
	Span     *SpanMatcher   `json:"c_Span"`
	Children []TestSpanTree `json:"z_Children,omitempty"`
}

func (t TestSpanTree) String() string {
	b, _ := json.MarshalIndent(t, "", "  ")
	return string(b)
}

// GetTraceTree converts a set slice of spans into a SpanTree.
func GetTraceTree(trace []model.SpanModel) (*SpanTree, error) {
	var roots []model.SpanModel
	parents := map[model.ID][]model.SpanModel{}
	for _, span := range trace {
		if span.ParentID != nil {
			parents[*span.ParentID] = append(parents[*span.ParentID], span)
		} else {
			roots = append(roots, span)
		}
	}

	children, err := getChildren(parents, roots)
	if err != nil {
		return nil, fmt.Errorf("Could not create span tree for %v: %v", PrettyPrintTrace(trace), err)
	}

	tree := SpanTree{
		Root:     true,
		Children: children,
	}
	if len(parents) != 0 {
		return nil, fmt.Errorf("Left over spans after generating the SpanTree: %v. Original: %v", parents, PrettyPrintTrace(trace))
	}
	return &tree, nil
}

func getChildren(parents map[model.ID][]model.SpanModel, current []model.SpanModel) ([]SpanTree, error) {
	var children []SpanTree
	for _, span := range current {
		grandchildren, err := getChildren(parents, parents[span.ID])
		if err != nil {
			return children, err
		}
		children = append(children, SpanTree{
			Span:     span,
			Children: grandchildren,
		})
		delete(parents, span.ID)
	}

	return children, nil
}

// SpanCount gets the count of spans in this tree.
func (t TestSpanTree) SpanCount() int {
	spans := 1
	if t.Root {
		// The root span is artificial. It exits solely so we can easily pass around the tree.
		spans = 0
	}
	for _, child := range t.Children {
		spans += child.SpanCount()
	}
	return spans
}

// MatchesSubtree checks to see if this TestSpanTree matches a subtree
// of the actual SpanTree. It is intended to be used for assertions
// while testing. Returns the set of possible subtree matches with the
// corresponding set of unmatched siblings.
func (tt TestSpanTree) MatchesSubtree(t *testing.T, actual *SpanTree) (matches [][]SpanTree) {
	if t != nil {
		t.Helper()
		t.Logf("attempting to match test tree %v against %v", tt, actual)
	}
	if err := tt.Span.MatchesSpan(&actual.Span); err == nil {
		if t != nil {
			t.Logf("%v matches span %v, matching children", tt.Span, actual.Span)
		}
		// Tree roots match; check children.
		if err := matchesSubtrees(t, tt.Children, actual.Children); err == nil {
			// A matching root leaves no unmatched siblings.
			matches = append(matches, nil)
		}
	}
	// Recursively match children.
	for i, child := range actual.Children {
		for _, childMatch := range tt.MatchesSubtree(t, &child) {
			// Append unmatched children to child results.
			childMatch = append(childMatch, actual.Children[:i]...)
			childMatch = append(childMatch, actual.Children[i+1:]...)
			matches = append(matches, childMatch)
		}
	}
	return
}

// matchesSubtrees checks for a match of each TestSpanTree with a
// subtree of a distrinct actual SpanTree.
func matchesSubtrees(t *testing.T, ts []TestSpanTree, as []SpanTree) error {
	if t != nil {
		t.Helper()
		t.Logf("attempting to match test trees %v against %v", ts, as)
	}
	if len(ts) == 0 {
		return nil
	}
	tt := ts[0]
	for j, a := range as {
		// If there is no error, then it matched successfully.
		for _, match := range tt.MatchesSubtree(t, &a) {
			asNew := make([]SpanTree, 0, len(as)-1+len(match))
			asNew = append(asNew, as[:j]...)
			asNew = append(asNew, as[j+1:]...)
			asNew = append(asNew, match...)
			if err := matchesSubtrees(t, ts[1:], asNew); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("unmatched span trees. want: %s got %s", ts, as)
}

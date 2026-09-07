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
	"regexp"
	"strconv"
	"testing"

	oteltrace "go.opentelemetry.io/otel/trace"
)

// hostSuffix is an optional suffix that might appear at the end of hostnames.
// We supplement matches with this to allow matches for:
//
//	foo.bar
//
// to match all of:
//
//	foo.bar
//	foo.bar.svc
//	foo.bar.svc.cluster.local
//
// It's hardly perfect, but requires the suffix to start with the delimiter '.'
// and then match anything prior to the path starting, e.g. '/'
const HostSuffix = "[.][^/]+"

// SpanData is a lightweight representation of an OTel span containing just the
// fields needed for test assertions. This avoids depending on the full
// ReadOnlySpan interface which is cumbersome to construct in tests.
type SpanData struct {
	SpanContext       oteltrace.SpanContext
	Parent            oteltrace.SpanContext
	Name              string
	Kind              oteltrace.SpanKind
	ServiceName       string
	Attributes        map[string]string
}

// PrettyPrintTrace pretty prints a Trace.
func PrettyPrintTrace(trace []SpanData) string {
	b, _ := json.Marshal(trace)
	return string(b)
}

// SpanTree is the tree of Spans representation of a Trace.
type SpanTree struct {
	Root     bool
	Span     SpanData
	Children []SpanTree
}

func (t SpanTree) String() string {
	b, _ := json.MarshalIndent(t, "", "  ")
	return string(b)
}

type SpanMatcher struct {
	Kind        *oteltrace.SpanKind       `json:"a_Kind,omitempty"`
	ServiceName string                    `json:"b_Name,omitempty"`
	Tags        map[string]*regexp.Regexp `json:"c_Tags,omitempty"`
}

type SpanMatcherOption func(*SpanMatcher)

func WithLocalEndpointServiceName(s string) SpanMatcherOption {
	return func(m *SpanMatcher) {
		m.ServiceName = s
	}
}

func WithHTTPHostAndPath(host, path string) SpanMatcherOption {
	return func(m *SpanMatcher) {
		m.Tags["http.host"] = regexp.MustCompile("^" + regexp.QuoteMeta(host) + HostSuffix + "$")
		m.Tags["http.path"] = regexp.MustCompile("^" + regexp.QuoteMeta(path) + "$")
	}
}

func WithHTTPURL(host, path string) SpanMatcherOption {
	return func(m *SpanMatcher) {
		m.Tags["http.url"] = regexp.MustCompile("^http://" + regexp.QuoteMeta(host) + HostSuffix + regexp.QuoteMeta(path) + "$")
	}
}

func (m *SpanMatcher) MatchesSpan(span *SpanData) error {
	if m == nil {
		return nil
	}
	if m.Kind != nil {
		if *m.Kind != span.Kind {
			return fmt.Errorf("mismatched kind: got %q, want %q", span.Kind, *m.Kind)
		}
	}
	if m.ServiceName != "" {
		if span.ServiceName == "" {
			return errors.New("missing service name")
		}
		if m.ServiceName != span.ServiceName {
			return fmt.Errorf("mismatched ServiceName: got %q, want %q", span.ServiceName, m.ServiceName)
		}
	}
	for k, v := range m.Tags {
		if t := span.Attributes[k]; !v.MatchString(t) {
			return fmt.Errorf("unexpected tag %s: got %q, want %q", k, t, v)
		}
	}
	return nil
}

func MatchSpan(kind oteltrace.SpanKind, opts ...SpanMatcherOption) *SpanMatcher {
	m := &SpanMatcher{
		Kind: &kind,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func MatchHTTPSpanWithCode(kind oteltrace.SpanKind, statusCode int, opts ...SpanMatcherOption) *SpanMatcher {
	return MatchSpan(kind, WithCode(statusCode))
}

func WithCode(statusCode int) SpanMatcherOption {
	return func(m *SpanMatcher) {
		m.Tags = map[string]*regexp.Regexp{
			"http.method":      regexp.MustCompile("^" + http.MethodPost + "$"),
			"http.status_code": regexp.MustCompile("^" + strconv.Itoa(statusCode) + "$"),
		}
	}
}

func MatchHTTPSpanNoReply(kind oteltrace.SpanKind, opts ...SpanMatcherOption) *SpanMatcher {
	return MatchHTTPSpanWithCode(kind, 202, opts...)
}

func MatchHTTPSpanWithReply(kind oteltrace.SpanKind, opts ...SpanMatcherOption) *SpanMatcher {
	return MatchHTTPSpanWithCode(kind, 200, opts...)
}

// TestSpanTree is the expected version of SpanTree used for assertions in testing.
//
// The JSON names of the fields are weird because we want a specific order when pretty printing
// JSON. The JSON will be printed in alphabetical order, so we are imposing a certain order by
// prefixing the keys with a specific letter. The letter has no mean other than ordering.
type TestSpanTree struct {
	Note     string         `json:"a_Note,omitempty"`
	Span     *SpanMatcher   `json:"c_Span"`
	Children []TestSpanTree `json:"z_Children,omitempty"`
}

func (tt TestSpanTree) String() string {
	b, _ := json.MarshalIndent(tt, "", "  ")
	return string(b)
}

// GetTraceTree converts a slice of SpanData into a SpanTree.
func GetTraceTree(trace []SpanData) (*SpanTree, error) {
	var roots []SpanData
	parents := map[oteltrace.SpanID][]SpanData{}
	for _, span := range trace {
		if span.Parent.IsValid() && span.Parent.HasSpanID() {
			parentID := span.Parent.SpanID()
			parents[parentID] = append(parents[parentID], span)
		} else {
			roots = append(roots, span)
		}
	}

	if len(roots) == 0 {
		return nil, fmt.Errorf("no root spans found in the trace: Original: %v", PrettyPrintTrace(trace))
	}

	children, err := getChildren(parents, roots)
	if err != nil {
		return nil, fmt.Errorf("could not create span tree for %v: %v", PrettyPrintTrace(trace), err)
	}

	if len(parents) != 0 {
		return nil, fmt.Errorf("left over spans after generating the SpanTree: %v. Original: %v", parents, PrettyPrintTrace(trace))
	}
	tree := SpanTree{
		Root:     true,
		Children: children,
	}
	return &tree, nil
}

func getChildren(parents map[oteltrace.SpanID][]SpanData, current []SpanData) ([]SpanTree, error) {
	children := make([]SpanTree, 0, len(current))
	for _, span := range current {
		spanID := span.SpanContext.SpanID()
		grandchildren, err := getChildren(parents, parents[spanID])
		if err != nil {
			return children, err
		}
		children = append(children, SpanTree{
			Span:     span,
			Children: grandchildren,
		})
		delete(parents, spanID)
	}

	return children, nil
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
	} else if t != nil {
		t.Logf("%v does not match span %v: %v", tt.Span, actual.Span, err)
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
// subtree of a distinct actual SpanTree.
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

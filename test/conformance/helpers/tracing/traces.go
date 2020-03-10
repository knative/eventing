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
	"sort"
	"strconv"
	"testing"

	"github.com/openzipkin/zipkin-go/model"
	"k8s.io/apimachinery/pkg/util/sets"
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

func (t SpanTree) ToTestSpanTree() TestSpanTree {
	children := make([]TestSpanTree, len(t.Children))
	for i := range t.Children {
		children[i] = t.Children[i].toTestSpanTreeHelper()
	}
	return TestSpanTree{
		Root:     t.Root,
		Children: children,
	}
}

func (t SpanTree) toTestSpanTreeHelper() TestSpanTree {
	name := ""
	if t.Span.LocalEndpoint != nil {
		name = t.Span.LocalEndpoint.ServiceName
	}
	children := make([]TestSpanTree, len(t.Children))
	for i := range t.Children {
		children[i] = t.Children[i].toTestSpanTreeHelper()
	}
	tst := TestSpanTree{
		Span: &SpanMatcher{
			Kind:                     &t.Span.Kind,
			LocalEndpointServiceName: name,
			Tags:                     t.Span.Tags,
		},
		Children: children,
	}
	tst.SortChildren()
	return tst
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

// SortChildren attempts to sort the children of this TestSpanTree. The children are siblings, order
// does not actually matter. TestSpanTree.Matches() correctly handles this, by matching in any
// order. SortChildren() is most useful before JSON pretty printing the structure and comparing
// manually.
//
// The order it uses:
//   1. Shorter children first.
//   2. Span kind.
//   3. "http.url", "http.host", "http.path" tag presence and values.
// If all of those are equal, then arbitrarily choose the earlier index.
func (t *TestSpanTree) SortChildren() {
	for _, child := range t.Children {
		child.SortChildren()
	}
	sort.Slice(t.Children, func(i, j int) bool {
		ic := t.Children[i]
		jc := t.Children[j]

		if ic.height() != jc.height() {
			return ic.height() < jc.height()
		}

		if r := ic.Span.Cmp(jc.Span); r != 0 {
			return r < 0
		}

		// We don't have anything to reliably differentiate by. So this isn't going to really be
		// sorted, just leave the existing one first arbitrarily.
		return i < j
	})
}

func (t TestSpanTree) height() int {
	height := 0
	for _, child := range t.Children {
		if ch := child.height(); ch >= height {
			height = ch + 1
		}
	}
	return height
}

// GetTraceTree converts a set slice of spans into a SpanTree.
func GetTraceTree(t *testing.T, trace []model.SpanModel) SpanTree {
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
		t.Fatalf("Could not create span tree for %v: %v", PrettyPrintTrace(trace), err)
	}

	tree := SpanTree{
		Root:     true,
		Children: children,
	}
	if len(parents) != 0 {
		t.Fatalf("Left over spans after generating the SpanTree: %v. Original: %v", parents, PrettyPrintTrace(trace))
	}
	return tree
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

// Matches checks to see if this TestSpanTree matches an actual SpanTree. It is intended to be used
// for assertions while testing.
func (t TestSpanTree) Matches(actual SpanTree) error {
	if g, w := actual.ToTestSpanTree().SpanCount(), t.SpanCount(); g != w {
		return fmt.Errorf("unexpected number of spans. got %d want %d", g, w)
	}
	t.SortChildren()
	if err := traceTreeMatches(".", t, actual); err != nil {
		return err
	}
	return nil
}

func traceTreeMatches(pos string, want TestSpanTree, got SpanTree) error {
	if err := want.Span.MatchesSpan(&got.Span); err != nil {
		return fmt.Errorf("no match for span at %q: %w", pos, err)
	}
	return unorderedTraceTreesMatch(pos, want.Children, got.Children)
}

// unorderedTraceTreesMatch checks to see if for every TestSpanTree in want, there is a
// corresponding SpanTree in got. It's comparison is done unordered, but slowly. It should not be
// called with too many entries in either slice.
func unorderedTraceTreesMatch(pos string, want []TestSpanTree, got []SpanTree) error {
	if g, w := len(got), len(want); g != w {
		return fmt.Errorf("unexpected number of children at %q: got %v, want %v", pos, g, w)
	}
	unmatchedGot := sets.NewInt()
	for i := range got {
		unmatchedGot.Insert(i)
	}
	// This is an O(n^4) algorithm. It compares every item in want to every item in got, O(n^2).
	// Those comparisons do the same recursively O(n^2). We expect there to be not too many traces,
	// so n should be small (say 50 in the largest cases).
OuterLoop:
	for i, w := range want {
		var errs []error
		for ug := range unmatchedGot {
			// If there is no error, then it matched successfully.
			if err := w.Matches(got[ug]); err == nil {
				unmatchedGot.Delete(ug)
				continue OuterLoop
			} else {
				errs = append(errs, fmt.Errorf("no child match %v: %v", ug, err))
			}
		}
		// Nothing matched.
		return fmt.Errorf("unable to find child match %s[%d]: errors %v. Want: %s **** Got: %s", pos, i, errs, w.String(), got)
	}
	// Everything matched.
	return nil
}

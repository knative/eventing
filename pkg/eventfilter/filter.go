package eventfilter

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const (
	PassFilter FilterResult = "pass"
	FailFilter FilterResult = "fail"
	NoFilter   FilterResult = "no_filter"
)

// FilterResult has the result of the filtering operation.
type FilterResult string

func (x FilterResult) And(y FilterResult) FilterResult {
	if x == NoFilter {
		return y
	}
	if y == NoFilter {
		return x
	}
	if x == PassFilter && y == PassFilter {
		return PassFilter
	}
	return FailFilter
}

// Filter is an interface representing an event filter of the trigger filter
type Filter interface {
	Filter(ctx context.Context, event cloudevents.Event) FilterResult
}

// Filters is a wrapper that runs each filter and performs the and
type Filters []Filter

func (filters Filters) Filter(ctx context.Context, event cloudevents.Event) FilterResult {
	res := NoFilter
	for _, f := range filters {
		res = res.And(f.Filter(ctx, event))
	}
	return res
}

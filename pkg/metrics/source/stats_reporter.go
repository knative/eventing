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

package source

// ReportArgs defines the arguments for reporting metrics.
type ReportArgs struct {
	Namespace     string
	EventType     string
	EventScheme   string
	EventSource   string // source attr from the event , not the name of the knative source
	Name          string
	ResourceGroup string
	Error         string
	Timeout       bool
}

// StatsReporter defines the interface for sending source metrics.
type StatsReporter interface {
	// ReportEventCount captures the event count. It records one per call.
	ReportEventCount(args *ReportArgs, responseCode int) error
	ReportRetryEventCount(args *ReportArgs, responseCode int) error
}

var _ StatsReporter = (*reporter)(nil)

// reporter is a no-op and will be removed later
type reporter struct{}

// NewStatsReporter creates a reporter that collects and reports source metrics.
func NewStatsReporter() (StatsReporter, error) {
	return &reporter{}, nil
}

func (r *reporter) ReportEventCount(args *ReportArgs, responseCode int) error {
	return nil
}

func (r *reporter) ReportRetryEventCount(args *ReportArgs, responseCode int) error {
	return nil
}

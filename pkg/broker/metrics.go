/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package broker

const (
	// EventArrivalTime is used to access the metadata stored on a
	// CloudEvent to measure the time difference between when an event is
	// received on a broker and when it is dispatched to the trigger function.
	// Should be set using time.Now(), which returns the current local time.
	// The format is: 2019-08-26T23:38:17.834384404Z.
	EventArrivalTime = "knativearrivaltime"

	// TraceParent is a documented extension for CloudEvent to include traces.
	// https://github.com/cloudevents/spec/blob/v0.3/extensions/distributed-tracing.md#traceparent
	// The format is: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01,
	// which stands for version("00" is the current version)-traceID-spanID-trace options
	TraceParent = "traceparent"
)

// Buckets125 generates an array of buckets with approximate powers-of-two
// buckets that also aligns with powers of 10 on every 3rd step. This can
// be used to create a view.Distribution.
func Buckets125(low, high float64) []float64 {
	buckets := []float64{low}
	for last := low; last < high; last = last * 10 {
		buckets = append(buckets, 2*last, 5*last, 10*last)
	}
	return buckets
}

// Result converts an error to a result string (either "success" or "error").
func Result(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}

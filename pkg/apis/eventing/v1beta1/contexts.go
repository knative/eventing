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

package v1beta1

import "context"

// brokerDefaultKey is used as the key for associating information
// with a context.Context.
type brokerDefaultKey struct{}

// WithDefaultBrokerConfigs adds the default Broker configurations
// to the current context.
func WithDefaultBrokerConfigs(ctx context.Context) context.Context {
	return context.WithValue(ctx, brokerDefaultKey{}, struct{}{})
}

// HasDefaultBrokerConfigs checks to see whether the given context has
// been marked as having broker defaults.
func HasDefaultBrokerConfigs(ctx context.Context) bool {
	return ctx.Value(brokerDefaultKey{}) != nil
}

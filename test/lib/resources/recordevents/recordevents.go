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

package recordevents

import (
	"context"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/naming"
	"knative.dev/eventing/test/lib/recordevents"
)

// Install a new record events pod and service in the client namespace.
func Install(ctx context.Context, c *testlib.Client, options ...recordevents.EventRecordOption) *recordevents.EventInfoStore {
	name := naming.MakeRandomK8sName("recordevents")
	store, _ := recordevents.StartEventRecordOrFail(ctx, c, name, options...)
	return store
}

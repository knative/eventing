/*
Copyright 2025 The Knative Authors

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

package versioned

// IsWatchListSemanticsUnSupported returns true to indicate this client
// does not support WatchList semantics.
//
// This prevents the informers from using WatchList when the Kubernetes
// server version is < 1.35, which would cause cache sync issues on GKE 1.34.x
// See: https://github.com/kubernetes/kubernetes/issues/135895
func (c *Clientset) IsWatchListSemanticsUnSupported() bool {
	return true
}

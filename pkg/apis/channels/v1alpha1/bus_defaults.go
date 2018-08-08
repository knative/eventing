/*
Copyright 2018 The Knative Authors

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

package v1alpha1

// TODO(n3wscott): This is staging work, the plan is another pass to bring up
// the test coverage, then remove unused after each type is stubbed.
// This is all prep for new serving style webhook integration.

func (b *Bus) SetDefaults() {
	b.Spec.SetDefaults()
}

func (bs *BusSpec) SetDefaults() {
	bs.Parameters.SetDefaults()
}

func (bp *BusParameters) SetDefaults() {
	// TODO anything?
}

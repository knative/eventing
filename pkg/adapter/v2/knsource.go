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

package adapter

const (
	// KnSourceExtension is the CloudEvent knsource extension referencing the originating CR
	KnSourceExtension = "knsource"
)

// KnSourceExtensionValue gets the value for the CloudEvent knsource extension referencing the originating CR
func KnSourceExtensionValue(plural, group, name string) string {
	return name + "." + plural + "." + group
}

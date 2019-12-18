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

package resources

import (
	"reflect"
	"testing"
)

func TestOriginalLabels(t *testing.T) {
	label := OriginalLabels(map[string]string{})
	if !reflect.DeepEqual(label, map[string]string{
		PropagationLabelKey: PropagationLabelValueOriginal,
	}) {
		t.Errorf("Should have required labels")
	}
}

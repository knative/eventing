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

package path

import (
	"fmt"
	"strings"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	prefix = "triggers"
)

// Generate generates the Path portion of a URI to send events to the given Trigger.
func Generate(t *v1alpha1.Trigger) string {
	return fmt.Sprintf("/%s/%s/%s", prefix, t.Namespace, t.Name)
}

// Parse parses the Path portion of a URI to determine which Trigger the request corresponds to. It
// is expected to be in the form "/triggers/namespace/name".
func Parse(path string) (types.NamespacedName, error) {
	parts := strings.Split(path, "/")
	if len(parts) != 4 {
		return types.NamespacedName{}, fmt.Errorf("incorrect number of parts in the path, expected 4, actual %d, '%s'", len(parts), path)
	}
	if parts[0] != "" {
		return types.NamespacedName{}, fmt.Errorf("text before the first slash, actual '%s'", path)
	}
	if parts[1] != prefix {
		return types.NamespacedName{}, fmt.Errorf("incorrect prefix, expected '%s', actual '%s'", prefix, path)
	}
	return types.NamespacedName{
		Namespace: parts[2],
		Name:      parts[3],
	}, nil
}

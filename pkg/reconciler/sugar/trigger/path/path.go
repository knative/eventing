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
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

const (
	prefix      = "triggers"
	replySuffix = "reply"
	dlsSuffix   = "dls"
)

// Generate generates the Path portion of a URI to send events to the given Trigger.
func Generate(t *v1.Trigger) string {
	return fmt.Sprintf("/%s/%s/%s/%s", prefix, t.Namespace, t.Name, t.UID)
}

func GenerateReply(t *v1.Trigger) string {
	return path.Join(Generate(t), replySuffix)
}

func GenerateDLS(t *v1.Trigger) string {
	return path.Join(Generate(t), dlsSuffix)
}

type NamespacedNameUID struct {
	types.NamespacedName
	UID     types.UID
	IsReply bool
	IsDLS   bool
}

// Parse parses the Path portion of a URI to determine which Trigger the request corresponds to. It
// is expected to be in the form "/triggers/namespace/name/uid" and eventually a "/reply" or "/dls" suffix.
func Parse(path string) (NamespacedNameUID, error) {
	parts := strings.Split(path, "/")
	if len(parts) != 5 && len(parts) != 6 {
		return NamespacedNameUID{}, fmt.Errorf("incorrect number of parts in the path, expected 5 or 6, actual %d, '%s'", len(parts), path)
	}
	if parts[0] != "" {
		return NamespacedNameUID{}, fmt.Errorf("text before the first slash, actual '%s'", path)
	}
	if parts[1] != prefix {
		return NamespacedNameUID{}, fmt.Errorf("incorrect prefix, expected '%s', actual '%s'", prefix, path)
	}

	return NamespacedNameUID{
		NamespacedName: types.NamespacedName{
			Namespace: parts[2],
			Name:      parts[3],
		},
		UID:     types.UID(parts[4]),
		IsReply: len(parts) == 6 && parts[5] == replySuffix,
		IsDLS:   len(parts) == 6 && parts[5] == dlsSuffix,
	}, nil
}

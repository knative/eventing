/*
Copyright 2023 The Knative Authors

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
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    NamespacedNameUID
		wantErr bool
	}{
		{
			path: "/triggers/namespace/name/uid",
			want: NamespacedNameUID{
				NamespacedName: types.NamespacedName{
					Name:      "name",
					Namespace: "namespace",
				},
				UID:     "uid",
				IsReply: false,
				IsDLS:   false,
			},
		},
		{
			path: "/triggers/namespace/name/uid/reply",
			want: NamespacedNameUID{
				NamespacedName: types.NamespacedName{
					Name:      "name",
					Namespace: "namespace",
				},
				UID:     "uid",
				IsReply: true,
				IsDLS:   false,
			},
		},
		{
			path: "/triggers/namespace/name/uid/dls",
			want: NamespacedNameUID{
				NamespacedName: types.NamespacedName{
					Name:      "name",
					Namespace: "namespace",
				},
				UID:     "uid",
				IsReply: false,
				IsDLS:   true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Error("unexpected diff (-want, +got) =", diff)
			}
		})
	}
}

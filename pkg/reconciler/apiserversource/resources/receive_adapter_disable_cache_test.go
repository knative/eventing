package resources

import (
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/adapter/apiserver"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/reconciler/source"
)

func TestMakeReceiveAdapterWithDisableCache(t *testing.T) {
	name := "source-name"

	testCases := []struct {
		name         string
		disableCache bool
		want         bool
	}{{
		name:         "DisableCache true",
		disableCache: true,
		want:         true,
	}, {
		name:         "DisableCache false",
		disableCache: false,
		want:         false,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			src := &v1.ApiServerSource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "source-namespace",
					UID:       "1234",
				},
				Spec: v1.ApiServerSourceSpec{
					Resources: []v1.APIVersionKindSelector{{
						APIVersion: "v1",
						Kind:       "Pod",
					}},
					EventMode:          "Resource",
					ServiceAccountName: "source-svc-acct",
				},
			}

			args := &ReceiveAdapterArgs{
				Image:        "test-image",
				Source:       src,
				Labels:       Labels(src.Name),
				SinkURI:      "http://sink.example.com",
				Configs:      &source.EmptyVarsGenerator{},
				Namespaces:   []string{"default"},
				DisableCache: tc.disableCache,
			}

			deployment, err := MakeReceiveAdapter(args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Find K_SOURCE_CONFIG env var
			var config apiserver.Config
			for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
				if env.Name == "K_SOURCE_CONFIG" {
					if err := json.Unmarshal([]byte(env.Value), &config); err != nil {
						t.Fatalf("failed to unmarshal K_SOURCE_CONFIG: %v", err)
					}
					break
				}
			}

			if config.DisableCache != tc.want {
				t.Errorf("DisableCache mismatch: got %v, want %v", config.DisableCache, tc.want)
			}
		})
	}
}

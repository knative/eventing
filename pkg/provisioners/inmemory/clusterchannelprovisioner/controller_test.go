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

package clusterchannelprovisioner

import (
	"fmt"
	"strings"
	"testing"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

type FakeManager struct {
	AddThrowsError bool
}

func (f *FakeManager) Add(r manager.Runnable) error {
	if f.AddThrowsError {
		return fmt.Errorf("FakeManager.Add()")
	}
	if _, err := inject.InjectorInto(f.SetFields, r); err != nil {
		return err
	}
	return nil
}

func (f *FakeManager) SetFields(i interface{}) error {
	return nil
}

func (f *FakeManager) Start(<-chan struct{}) error {
	return nil
}

func (f *FakeManager) GetConfig() *rest.Config {
	return nil
}

func (f *FakeManager) GetScheme() *runtime.Scheme {
	return nil
}

func (f *FakeManager) GetAdmissionDecoder() types.Decoder {
	return nil
}

func (f *FakeManager) GetClient() client.Client {
	return nil
}

func (f *FakeManager) GetFieldIndexer() client.FieldIndexer {
	return nil
}

func (f *FakeManager) GetCache() cache.Cache {
	return nil
}

func (f *FakeManager) GetRecorder(name string) record.EventRecorder {
	return nil
}

func (f *FakeManager) GetRESTMapper() meta.RESTMapper {
	return nil
}

var _ manager.Manager = (*FakeManager)(nil)

func TestProvideController(t *testing.T) {
	testCases := map[string]struct {
		mgr     manager.Manager
		wantErr string
	}{
		"controller.New throws error": {
			mgr: &FakeManager{
				AddThrowsError: true,
			},
			wantErr: "FakeManager.Add()",
		},
		"c.Watch throws error": {
			mgr:     &FakeManager{},
			wantErr: "must call CacheInto on Kind before calling Start",
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			c, err := ProvideController(tc.mgr, zap.NewNop())

			// TODO: this is not a very good test. It is not clear if the test
			// will valuable in the end anyway, it just tests controller runtime
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("Expected error %q, got nil.", tc.wantErr)
				}
				got := err.Error()
				if !strings.Contains(got, tc.wantErr) {
					t.Fatalf("Expected error %q, got %q.", tc.wantErr, got)
				}
				return
			}
			_ = c
		})
	}

}

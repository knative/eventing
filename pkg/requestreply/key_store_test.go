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

package requestreply

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
)

func TestKeyStoreWatch(t *testing.T) {
	tt := map[string]struct {
		startFiles  map[string][]byte
		fileUpdates map[string]struct {
			action   string // one of create or delete
			contents []byte
		}
		expectedSecrets map[types.NamespacedName][][]byte
	}{
		"all files present at start": {
			startFiles: map[string][]byte{
				"default.request-reply.key-0": exampleKey,
				"default.request-reply.key-1": otherKey,
				"ns-1.request-reply-1.key-0":  exampleKey,
				"ns-1.request-reply-2.key-0":  exampleKey,
			},
			expectedSecrets: map[types.NamespacedName][][]byte{
				{
					Name:      "request-reply",
					Namespace: "default",
				}: {
					exampleKey,
					otherKey,
				},
				{
					Name:      "request-reply-1",
					Namespace: "ns-1",
				}: {
					exampleKey,
				},
				{
					Name:      "request-reply-2",
					Namespace: "ns-1",
				}: {
					exampleKey,
				},
			},
		},
		"file added": {
			startFiles: map[string][]byte{
				"default.request-reply.key-0": exampleKey,
				"ns-1.request-reply-1.key-0":  exampleKey,
				"ns-1.request-reply-2.key-0":  exampleKey,
			},
			fileUpdates: map[string]struct {
				action   string
				contents []byte
			}{
				"default.request-reply.key-1": {
					action:   "create",
					contents: otherKey,
				},
			},
			expectedSecrets: map[types.NamespacedName][][]byte{
				{
					Name:      "request-reply",
					Namespace: "default",
				}: {
					exampleKey,
					otherKey,
				},
				{
					Name:      "request-reply-1",
					Namespace: "ns-1",
				}: {
					exampleKey,
				},
				{
					Name:      "request-reply-2",
					Namespace: "ns-1",
				}: {
					exampleKey,
				},
			},
		},
		"file removed": {
			startFiles: map[string][]byte{
				"default.request-reply.key-0": exampleKey,
				"default.request-reply.key-1": otherKey,
				"ns-1.request-reply-1.key-0":  exampleKey,
				"ns-1.request-reply-2.key-0":  exampleKey,
			},
			fileUpdates: map[string]struct {
				action   string
				contents []byte
			}{
				"default.request-reply.key-1": {
					action: "delete",
				},
			},
			expectedSecrets: map[types.NamespacedName][][]byte{
				{
					Name:      "request-reply",
					Namespace: "default",
				}: {
					exampleKey,
				},
				{
					Name:      "request-reply-1",
					Namespace: "ns-1",
				}: {
					exampleKey,
				},
				{
					Name:      "request-reply-2",
					Namespace: "ns-1",
				}: {
					exampleKey,
				},
			},
		},
	}

	logger, err := zap.NewDevelopment()
	assert.NoError(t, err, "creating dev logger should succeed")

	for tn, tc := range tt {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			dname, err := os.MkdirTemp("", "test")
			assert.NoError(t, err, "creating temp dir for test should succeed")

			for name, contents := range tc.startFiles {
				os.WriteFile(filepath.Join(dname, name), contents, 0664)
			}

			ks := &AESKeyStore{
				Logger: logger.Named(tn).Sugar(),
			}

			ks.WatchPath(dname)
			defer ks.StopWatch()

			for name, action := range tc.fileUpdates {
				if action.action == "create" {
					os.WriteFile(filepath.Join(dname, name), action.contents, 0664)
					continue
				}
				os.Remove(filepath.Join(dname, name))
			}

			// give a bit of time for the watcher to see everything
			time.Sleep(500 * time.Millisecond)

			for rr, expected := range tc.expectedSecrets {
				got, ok := ks.GetAllKeys(rr)
				if len(expected) > 0 {
					assert.True(t, ok, "should have secrets in the store")
				} else {
					assert.False(t, ok)
				}

				assert.ElementsMatch(t, got, expected)
			}

			defer os.RemoveAll(dname)
		})
	}

}

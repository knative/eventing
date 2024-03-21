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

package subscriptionsapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

func TestFiltersMap(t *testing.T) {
	fm := NewFiltersMap()
	exact, _ := NewExactFilter(map[string]string{"type": "sample.event.type"})
	exactTrigger := &eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "exact",
			Namespace: "default",
		},
	}
	prefix, _ := NewPrefixFilter(map[string]string{"source": "github.com"})
	prefixTrigger := &eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prefix",
			Namespace: "default",
		},
	}
	fm.Set(exactTrigger, exact)
	fm.Set(prefixTrigger, prefix)

	newExact, ok := fm.Get(exactTrigger)
	assert.True(t, ok)
	assert.Equal(t, exact, newExact)

	newPrefix, ok := fm.Get(prefixTrigger)
	assert.True(t, ok)
	assert.Equal(t, prefix, newPrefix)

	fm.Delete(prefixTrigger)
	_, ok = fm.Get(prefixTrigger)
	assert.False(t, ok)
}

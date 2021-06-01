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

package experimental

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	logtesting "knative.dev/pkg/logging/testing"

	. "knative.dev/pkg/configmap/testing"
)

func TestStoreLoadWithContext(t *testing.T) {
	store := NewStore(logtesting.TestLogger(t))

	_, exampleConfig := ConfigMapsFromTestFile(t, FlagsConfigName)
	store.OnConfigChanged(exampleConfig)

	have := FromContextOrDefaults(store.ToContext(context.Background()))
	expected, _ := NewFlagsConfigFromConfigMap(exampleConfig)

	require.Equal(t, expected.IsEnabled("my-true-flag"), have.IsEnabled("my-true-flag"))
	require.Equal(t, expected.IsEnabled("my-false-flag"), have.IsEnabled("my-false-flag"))
	require.Equal(t, expected.IsEnabled("other-flag"), have.IsEnabled("other-flag"))

	require.Equal(t, expected.IsEnabled("my-true-flag"), store.IsEnabled("my-true-flag"))
	require.Equal(t, expected.IsEnabled("my-false-flag"), store.IsEnabled("my-false-flag"))
	require.Equal(t, expected.IsEnabled("other-flag"), store.IsEnabled("other-flag"))
}

func TestStoreLoadWithContextOrDefaults(t *testing.T) {
	have := FromContextOrDefaults(context.Background())

	require.False(t, have.IsEnabled("my-true-flag"))
	require.False(t, have.IsEnabled("my-false-flag"))
	require.False(t, have.IsEnabled("other-flag"))
}

/*
Copyright 2020 The Knative Authors

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

package kncloudevents

import (
	nethttp "net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opencensus.io/plugin/ochttp"
)

func TestConfigureConnectionArgsOldClient(t *testing.T) {
	// Set connection args
	configureConnectionArgsOldClient(&ConnectionArgs{
		MaxIdleConnsPerHost: 1000,
		MaxIdleConns:        1000,
	})
	client1 := getClient()

	require.Same(t, getClient(), client1)
	require.Equal(t, 1000, castToTransport(client1).MaxIdleConns)
	require.Equal(t, 1000, castToTransport(client1).MaxIdleConnsPerHost)

	// Set other connection args
	configureConnectionArgsOldClient(&ConnectionArgs{
		MaxIdleConnsPerHost: 2000,
		MaxIdleConns:        2000,
	})
	client2 := getClient()

	require.Same(t, getClient(), client2)
	require.Equal(t, 2000, castToTransport(client2).MaxIdleConns)
	require.Equal(t, 2000, castToTransport(client2).MaxIdleConnsPerHost)

	// Try to set the same value and client should not be cleaned up
	configureConnectionArgsOldClient(&ConnectionArgs{
		MaxIdleConnsPerHost: 2000,
		MaxIdleConns:        2000,
	})
	require.Same(t, getClient(), client2)

	// Set back to nil
	configureConnectionArgsOldClient(nil)
	client3 := getClient()

	require.Same(t, getClient(), client3)
	require.Equal(t, nethttp.DefaultTransport.(*nethttp.Transport).MaxIdleConns, castToTransport(client3).MaxIdleConns)
	require.Equal(t, nethttp.DefaultTransport.(*nethttp.Transport).MaxIdleConnsPerHost, castToTransport(client3).MaxIdleConnsPerHost)

	require.NotSame(t, client1, client2)
	require.NotSame(t, client1, client3)
	require.NotSame(t, client2, client3)
}

func castToTransport(client *nethttp.Client) *nethttp.Transport {
	return client.Transport.(*ochttp.Transport).Base.(*nethttp.Transport)
}

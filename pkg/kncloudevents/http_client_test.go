package kncloudevents

import (
	nethttp "net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opencensus.io/plugin/ochttp"
)

func TestConfigureConnectionArgs(t *testing.T) {
	// Set connection args
	ConfigureConnectionArgs(&ConnectionArgs{
		MaxIdleConnsPerHost: 1000,
		MaxIdleConns:        1000,
	})
	client1 := getClient()

	require.Same(t, getClient(), client1)
	require.Equal(t, 1000, castToTransport(client1).MaxIdleConns)
	require.Equal(t, 1000, castToTransport(client1).MaxIdleConnsPerHost)

	// Set other connection args
	ConfigureConnectionArgs(&ConnectionArgs{
		MaxIdleConnsPerHost: 2000,
		MaxIdleConns:        2000,
	})
	client2 := getClient()

	require.Same(t, getClient(), client2)
	require.Equal(t, 2000, castToTransport(client2).MaxIdleConns)
	require.Equal(t, 2000, castToTransport(client2).MaxIdleConnsPerHost)

	// Try to set the same value and client should not be cleaned up
	ConfigureConnectionArgs(&ConnectionArgs{
		MaxIdleConnsPerHost: 2000,
		MaxIdleConns:        2000,
	})
	require.Same(t, getClient(), client2)

	// Set back to nil
	ConfigureConnectionArgs(nil)
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

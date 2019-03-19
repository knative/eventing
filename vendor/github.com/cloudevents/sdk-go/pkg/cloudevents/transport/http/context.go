package http

import (
	"context"
)

// TransportContext allows a Receiver to understand the context of a request.
type TransportContext struct {
	URI    string
	Host   string
	Method string
}

// Opaque key type used to store TransportContext
type transportContextKeyType struct{}

var transportContextKey = transportContextKeyType{}

func WithTransportContext(ctx context.Context, tcxt TransportContext) context.Context {
	return context.WithValue(ctx, transportContextKey, tcxt)
}

func TransportContextFrom(ctx context.Context) TransportContext {
	tctx := ctx.Value(transportContextKey)
	if tctx != nil {
		if tx, ok := tctx.(TransportContext); ok {
			return tx
		}
		if tx, ok := tctx.(*TransportContext); ok {
			return *tx
		}
	}
	return TransportContext{}
}

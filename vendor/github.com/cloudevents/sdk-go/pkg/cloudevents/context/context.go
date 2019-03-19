package context

import (
	"context"
	"net/url"
)

// Opaque key type used to store target
type targetKeyType struct{}

var targetKey = targetKeyType{}

func WithTarget(ctx context.Context, target string) context.Context {
	return context.WithValue(ctx, targetKey, target)
}

func TargetFrom(ctx context.Context) *url.URL {
	c := ctx.Value(targetKey)
	if c != nil {
		if s, ok := c.(string); ok && s != "" {
			if target, err := url.Parse(s); err == nil {
				return target
			}
		}
	}
	return nil
}

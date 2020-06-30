package main

import (
	"context"
	"fmt"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
)

func injectionEnabled() context.Context {
	ctx := signals.NewContext()
	cfg := sharedmain.ParseAndGetConfigOrDie()
	ctx, informers := injection.Default.SetupInformers(ctx, cfg)

	// Start the injection clients and informers.
	go func(ctx context.Context) {
		if err := controller.StartInformers(ctx.Done(), informers...); err != nil {
			panic(fmt.Sprintf("Failed to start informers - %s", err))
		}
		<-ctx.Done()
	}(ctx)

	return ctx
}

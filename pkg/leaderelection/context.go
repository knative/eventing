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

package leaderelection

import (
	"context"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"

	"knative.dev/pkg/controller"
	pkgleaderelection "knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
)

// WithStandardLeaderElectorBuilder infuses a context with the ability to build
// LeaderElectors with the provided component configuration acquiring resource
// locks via the provided kubernetes client
func WithStandardLeaderElectorBuilder(ctx context.Context, kc kubernetes.Interface, cc pkgleaderelection.ComponentConfig) context.Context {
	return context.WithValue(ctx, builderKey{}, &standardBuilder{
		kc:  kc,
		lec: cc,
	})
}

// Elector is the interface for running a leader elector.
type Elector interface {
	Run(context.Context)
}

// Adapter is the interface for running adapter
type Adapter interface {
	Start(ctx context.Context) error
}

// BuildAdapterElector builds a leaderelection.LeaderElector for the given adapter
// using a builder added to the context via WithXXLeaderElectorBuilder.
func BuildAdapterElector(ctx context.Context, adapter Adapter) (Elector, error) {
	if val := ctx.Value(builderKey{}); val != nil {
		switch builder := val.(type) {
		case *standardBuilder:
			return builder.BuildElector(ctx, adapter)
		}
	}

	return &unopposedElector{
		adapter: adapter,
	}, nil
}

type builderKey struct{}

type standardBuilder struct {
	kc  kubernetes.Interface
	lec pkgleaderelection.ComponentConfig
}

func (b *standardBuilder) BuildElector(ctx context.Context, adapter Adapter) (Elector, error) {
	logger := logging.FromContext(ctx)
	recorder := controller.GetEventRecorder(ctx)
	if recorder == nil {
		// Create event broadcaster
		logger.Debug("Creating event broadcaster")
		eventBroadcaster := record.NewBroadcaster()
		watches := []watch.Interface{
			eventBroadcaster.StartLogging(logger.Named("event-broadcaster").Infof),
			eventBroadcaster.StartRecordingToSink(
				&typedcorev1.EventSinkImpl{Interface: b.kc.CoreV1().Events(system.Namespace())}),
		}
		recorder = eventBroadcaster.NewRecorder(
			scheme.Scheme, corev1.EventSource{Component: b.lec.Component})
		go func() {
			<-ctx.Done()
			for _, w := range watches {
				w.Stop()
			}
		}()
	}

	// Create a unique identifier so that two controllers on the same host don't
	// race.
	id, err := pkgleaderelection.UniqueID()
	if err != nil {
		logger.Fatalw("Failed to get unique ID for leader election", zap.Error(err))
	}
	logger.Infof("%v will run in leader-elected mode with id %v", b.lec.Component, id)

	// rl is the resource used to hold the leader election lock.
	rl, err := resourcelock.New(pkgleaderelection.KnativeResourceLock,
		system.Namespace(), // use namespace we are running in
		b.lec.Component,    // component is used as the resource name
		b.kc.CoreV1(),
		b.kc.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: recorder,
		})
	if err != nil {
		logger.Fatalw("Error creating lock", zap.Error(err))
	}

	return leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: b.lec.LeaseDuration,
		RenewDeadline: b.lec.RenewDeadline,
		RetryPeriod:   b.lec.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				logger.Infof("%q has started leading %q", rl.Identity(), b.lec.Component)
				err := adapter.Start(ctx)
				if err != nil {
					logger.Fatalw("The adapter failed to start", zap.Error(err))
				}
			},
			OnStoppedLeading: func() {
				logger.Infof("%q has stopped leading %q", rl.Identity(), b.lec.Component)
			},
		},
		ReleaseOnCancel: true,
		// TODO: use health check watchdog, knative/pkg#1048
		Name: b.lec.Component,
	})
}

// unopposedElector runs the adapter without needing to be elected.
type unopposedElector struct {
	adapter Adapter
}

// Run implements Elector
func (ue *unopposedElector) Run(ctx context.Context) {
	err := ue.adapter.Start(ctx)
	if err != nil {
		logging.FromContext(ctx).Warn("Start returned an error", zap.Error(err))
	}
}

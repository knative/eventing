/*
Copyright 2019 The Knative Authors.

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

package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	// Uncomment the following line to load the gcp plugin
	// (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"github.com/kelseyhightower/envconfig"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/source"

	"knative.dev/eventing/pkg/adapter"
	"knative.dev/eventing/pkg/adapter/apiserver"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/tracing"
)

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

type StringList []string

// Decode splits list of strings separated by '|',
// overriding the default comma separator which is
// a valid label selector character.
func (s *StringList) Decode(value string) error {
	*s = strings.Split(value, ";")
	return nil
}

type envConfig struct {
	adapter.EnvConfig

	Mode            string     `envconfig:"MODE"`
	ApiVersion      StringList `split_words:"true" required:"true"`
	Kind            StringList `required:"true"`
	Controller      []bool     `required:"true"`
	LabelSelector   StringList `envconfig:"SELECTOR" required:"true"`
	OwnerApiVersion StringList `envconfig:"OWNER_API_VERSION" required:"true"`
	OwnerKind       StringList `envconfig:"OWNER_KIND" required:"true"`
	Name            string     `envconfig:"NAME" required:"true"`
}

const (
	component = "apiserversource"
)

func main() {
	flag.Parse()

	ctx := signals.NewContext()

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		log.Fatalf("Error processing env var: %s", err)
	}

	// Convert json logging.Config to logging.Config.
	loggingConfig, err := logging.JsonToLoggingConfig(env.LoggingConfigJson)
	if err != nil {
		fmt.Printf("[ERROR] failed to process logging config: %s", err.Error())
		// Use default logging config.
		if loggingConfig, err = logging.NewConfigFromMap(map[string]string{}); err != nil {
			// If this fails, there is no recovering.
			panic(err)
		}
	}
	loggerSugared, _ := logging.NewLoggerFromConfig(loggingConfig, component)
	logger := loggerSugared.Desugar()
	defer flush(loggerSugared)
	ctx = logging.WithLogger(ctx, loggerSugared)

	// Report stats on Go memory usage every 30 seconds.
	msp := metrics.NewMemStatsAll()
	msp.Start(ctx, 30*time.Second)
	if err := view.Register(msp.DefaultViews()...); err != nil {
		logger.Fatal("Error exporting go memstats view: %v", zap.Error(err))
	}

	// Convert json metrics.ExporterOptions to metrics.ExporterOptions.
	metricsConfig, err := metrics.JsonToMetricsOptions(env.MetricsConfigJson)
	if err != nil {
		logger.Error("failed to process metrics options", zap.Error(err))
	}

	if err := metrics.UpdateExporter(*metricsConfig, loggerSugared); err != nil {
		logger.Error("failed to create the metrics exporter", zap.Error(err))
	}

	reporter, err := source.NewStatsReporter()
	if err != nil {
		logger.Error("error building statsreporter", zap.Error(err))
	}

	if err = tracing.SetupStaticPublishing(loggerSugared, "", tracing.OnePercentSampling); err != nil {
		// If tracing doesn't work, we will log an error, but allow the adapter
		// to continue to start.
		logger.Error("Error setting up trace publishing", zap.Error(err))
	}

	eventsClient, err := kncloudevents.NewDefaultClient(env.SinkURI)
	if err != nil {
		logger.Fatal("error building cloud event client", zap.Error(err))
	}

	// Configuring the adapter

	cfg, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		logger.Fatal("error building kubeconfig", zap.Error(err))
	}

	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		logger.Fatal("error building dynamic client", zap.Error(err))
	}

	gvrcs := []apiserver.GVRC(nil)

	for i, apiVersion := range env.ApiVersion {
		kind := env.Kind[i]
		controlled := env.Controller[i]
		selector := env.LabelSelector[i]
		ownerApiVersion := env.OwnerApiVersion[i]
		ownerKind := env.OwnerKind[i]

		gv, err := schema.ParseGroupVersion(apiVersion)
		if err != nil {
			logger.Fatal("error parsing APIVersion", zap.Error(err))
		}
		// TODO: pass down the resource and the kind so we do not have to guess.
		gvr, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{Kind: kind, Group: gv.Group, Version: gv.Version})
		gvrcs = append(gvrcs, apiserver.GVRC{
			GVR:             gvr,
			Controller:      controlled,
			LabelSelector:   selector,
			OwnerApiVersion: ownerApiVersion,
			OwnerKind:       ownerKind,
		})
	}

	opt := apiserver.Options{
		Namespace: env.Namespace,
		Mode:      env.Mode,
		GVRCs:     gvrcs,
	}

	adapter := apiserver.NewAdaptor(cfg.Host, client, eventsClient, loggerSugared, opt, reporter, env.Name)
	logger.Info("Starting Receive Adapter", zap.Any("adapter", adapter))

	if err := adapter.Start(ctx.Done()); err != nil {
		logger.Warn("start returned an error", zap.Error(err))
	}
}

func flush(logger *zap.SugaredLogger) {
	_ = logger.Sync()
	metrics.FlushExporter()
}

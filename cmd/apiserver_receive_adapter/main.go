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

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/kelseyhightower/envconfig"
	"github.com/knative/eventing/pkg/adapter/apiserver"
	"github.com/knative/eventing/pkg/kncloudevents"
	"github.com/knative/pkg/signals"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

type envConfig struct {
	Namespace  string   `envconfig:"SYSTEM_NAMESPACE" default:"default"`
	Mode       string   `envconfig:"MODE"`
	SinkURI    string   `split_words:"true" required:"true"`
	ApiVersion []string `split_words:"true" required:"true"`
	Kind       []string `required:"true"`
	Controller []bool   `required:"true"`
}

// TODO: the controller should take the list of GVR

func main() {
	flag.Parse()

	logCfg := zap.NewProductionConfig()
	logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	dlogger, err := logCfg.Build()
	logger := dlogger.Sugar()

	var env envConfig
	err = envconfig.Process("", &env)
	if err != nil {
		logger.Fatalw("Error processing environment", zap.Error(err))
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		logger.Fatalw("Error building kubeconfig", zap.Error(err))
	}

	logger = logger.With(zap.String("controller/apiserver", "adapter"))
	logger.Info("Starting the controller")

	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		logger.Fatalw("Error building dynamic client", zap.Error(err))
	}

	eventsClient, err := kncloudevents.NewDefaultClient(env.SinkURI)
	if err != nil {
		logger.Fatalw("Error building cloud event client", zap.Error(err))
	}

	gvrcs := []apiserver.GVRC(nil)

	for i, apiVersion := range env.ApiVersion {
		kind := env.Kind[i]
		controlled := env.Controller[i]

		gv, err := schema.ParseGroupVersion(apiVersion)
		if err != nil {
			logger.Fatalw("Error parsing APIVersion", zap.Error(err))
		}
		// TODO: pass down the resource and the kind so we do not have to guess.
		gvr, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{Kind: kind, Group: gv.Group, Version: gv.Version})
		gvrcs = append(gvrcs, apiserver.GVRC{
			GVR:        gvr,
			Controller: controlled,
		})
	}

	opt := apiserver.Options{
		Namespace: env.Namespace,
		Mode:      env.Mode,
		GVRCs:     gvrcs,
	}

	a := apiserver.NewAdaptor(cfg.Host, client, eventsClient, logger, opt)
	logger.Info("starting kubernetes api adapter")
	if err := a.Start(stopCh); err != nil {
		logger.Warn("start returned an error,", zap.Error(err))
	}
}

package inmemorychannel

import (
	"time"

	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/system"
	"go.uber.org/zap"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
)

// Options defines the common reconciler options.
// We define this to reduce the boilerplate argument list when
// creating our controllers.
type Options struct {
	KubeClientSet    kubernetes.Interface
	DynamicClientSet dynamic.Interface

	EventingClientSet      clientset.Interface
	ApiExtensionsClientSet apiextensionsclientset.Interface
	//CachingClientSet cachingclientset.Interface

	Recorder      record.EventRecorder
	StatsReporter reconciler.StatsReporter

	ConfigMapWatcher *configmap.InformedWatcher
	Logger           *zap.SugaredLogger

	ResyncPeriod time.Duration
	StopChannel  <-chan struct{}
}

func NewOptionsOrDie(cfg *rest.Config, logger *zap.SugaredLogger, stopCh <-chan struct{}) Options {
	kubeClient := kubernetes.NewForConfigOrDie(cfg)
	eventingClient := clientset.NewForConfigOrDie(cfg)
	dynamicClient := dynamic.NewForConfigOrDie(cfg)
	apiExtensionsClient := apiextensionsclientset.NewForConfigOrDie(cfg)

	configMapWatcher := configmap.NewInformedWatcher(kubeClient, system.Namespace())

	return Options{
		KubeClientSet:          kubeClient,
		DynamicClientSet:       dynamicClient,
		EventingClientSet:      eventingClient,
		ApiExtensionsClientSet: apiExtensionsClient,
		ConfigMapWatcher:       configMapWatcher,
		Logger:                 logger,
		ResyncPeriod:           10 * time.Hour, // Based on controller-runtime default.
		StopChannel:            stopCh,
	}
}

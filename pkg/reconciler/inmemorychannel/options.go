package inmemorychannel

import (
	"time"

	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/logging/logkey"
	"github.com/knative/pkg/system"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	"github.com/knative/eventing/pkg/reconciler"
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

// NewBase instantiates a new instance of Base implementing
// the common & boilerplate code between our reconcilers.
func NewBase(opt Options, controllerAgentName string) *reconciler.Base {
	// Enrich the logs with controller name
	logger := opt.Logger.Named(controllerAgentName).With(zap.String(logkey.ControllerType, controllerAgentName))

	recorder := opt.Recorder
	if recorder == nil {
		// Create event broadcaster
		logger.Debug("Creating event broadcaster")
		eventBroadcaster := record.NewBroadcaster()
		watches := []watch.Interface{
			eventBroadcaster.StartLogging(logger.Named("event-broadcaster").Infof),
			eventBroadcaster.StartRecordingToSink(
				&typedcorev1.EventSinkImpl{Interface: opt.KubeClientSet.CoreV1().Events("")}),
		}
		recorder = eventBroadcaster.NewRecorder(
			scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
		go func() {
			<-opt.StopChannel
			for _, w := range watches {
				w.Stop()
			}
		}()
	}

	statsReporter := opt.StatsReporter
	if statsReporter == nil {
		logger.Debug("Creating stats reporter")
		var err error
		statsReporter, err = reconciler.NewStatsReporter(controllerAgentName)
		if err != nil {
			logger.Fatal(err)
		}
	}

	base := &reconciler.Base{
		KubeClientSet:          opt.KubeClientSet,
		EventingClientSet:      opt.EventingClientSet,
		ApiExtensionsClientSet: opt.ApiExtensionsClientSet,
		DynamicClientSet:       opt.DynamicClientSet,
		ConfigMapWatcher:       opt.ConfigMapWatcher,
		Recorder:               recorder,
		StatsReporter:          statsReporter,
		Logger:                 logger,
	}

	return base
}

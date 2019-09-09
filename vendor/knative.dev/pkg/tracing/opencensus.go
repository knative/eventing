package tracing

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"contrib.go.opencensus.io/exporter/stackdriver"
	oczipkin "contrib.go.opencensus.io/exporter/zipkin"
	"github.com/openzipkin/zipkin-go"
	httpreporter "github.com/openzipkin/zipkin-go/reporter/http"
	"go.opencensus.io/trace"
	"go.uber.org/zap"

	"knative.dev/pkg/tracing/config"
)

// ConfigOption is the interface for adding additional exporters and configuring opencensus tracing.
type ConfigOption func(*config.Config) error

// OpenCensusTracer is responsible for managing and updating configuration of OpenCensus tracing
type OpenCensusTracer struct {
	curCfg        *config.Config
	configOptions []ConfigOption

	closer   io.Closer
	exporter trace.Exporter
}

// OpenCensus tracing keeps state in globals and therefore we can only run one OpenCensusTracer
var (
	octMutex  sync.Mutex
	globalOct *OpenCensusTracer
)

func NewOpenCensusTracer(configOptions ...ConfigOption) *OpenCensusTracer {
	return &OpenCensusTracer{
		configOptions: configOptions,
	}
}

func (oct *OpenCensusTracer) ApplyConfig(cfg *config.Config) error {
	logger, _ := zap.NewDevelopment()
	err := oct.acquireGlobal()
	defer octMutex.Unlock()
	if err != nil {
		logger.Info("ApplyConfig - 1 - early err")
		return err
	}

	// Short circuit if our config hasnt changed
	if oct.curCfg != nil && oct.curCfg.Equals(cfg) {
		logger.Info("ApplyConfig - 2 - no op")
		return nil
	}

	// Apply config options
	for _, configOpt := range oct.configOptions {
		if err = configOpt(cfg); err != nil {
			return err
		}
	}
	logger.Info(fmt.Sprintf("ApplyConfig - options applied - %v", len(oct.configOptions)))

	// Set config
	trace.ApplyConfig(*createOCTConfig(cfg))

	logger.Info("ApplyConfig - healthy exit")
	return nil
}

func (oct *OpenCensusTracer) Finish() error {
	err := oct.acquireGlobal()
	defer octMutex.Unlock()
	if err != nil {
		return errors.New("finish called on OpenTracer which is not the global OpenCensusTracer")
	}

	for _, configOpt := range oct.configOptions {
		if err = configOpt(nil); err != nil {
			return err
		}
	}
	globalOct = nil

	return nil
}

func (oct *OpenCensusTracer) acquireGlobal() error {
	octMutex.Lock()

	if globalOct == nil {
		globalOct = oct
	} else if globalOct != oct {
		return errors.New("an OpenCensusTracer already exists and only one can be run at a time")
	}

	return nil
}

func createOCTConfig(cfg *config.Config) *trace.Config {
	octCfg := trace.Config{}

	if cfg.Backend != config.None {
		if cfg.Debug {
			octCfg.DefaultSampler = trace.AlwaysSample()
		} else {
			octCfg.DefaultSampler = trace.ProbabilitySampler(cfg.SampleRate)
		}
	} else {
		octCfg.DefaultSampler = trace.NeverSample()
	}

	return &octCfg
}

// WithExporter returns a ConfigOption for use with NewOpenCensusTracer that configures
// it to export traces based on the configuration read from config-tracing.
func WithExporter(name string, logger *zap.SugaredLogger) ConfigOption {
	return func(cfg *config.Config) error {
		logger.Info("WithExporter internal function called")
		var (
			exporter trace.Exporter
			closer   io.Closer
		)
		switch cfg.Backend {
		case config.Stackdriver:
			exp, err := stackdriver.NewExporter(stackdriver.Options{
				ProjectID: cfg.StackdriverProjectID,
			})
			if err != nil {
				logger.Errorw("error reading project-id from metadata", zap.Error(err))
				return err
			}
			exporter = exp
		case config.Zipkin:
			logger.Info("Adding Zipkin tracing")
			hostPort := name + ":80"
			zipEP, err := zipkin.NewEndpoint(name, hostPort)
			if err != nil {
				logger.Errorw("error building zipkin endpoint", zap.Error(err))
				return err
			}
			reporter := httpreporter.NewReporter(cfg.ZipkinEndpoint)
			exporter = oczipkin.NewExporter(reporter, zipEP)
			closer = reporter
		default:
			// Disables tracing.
		}
		if exporter != nil {
			trace.RegisterExporter(exporter)
			logger.Infof("Registered exporter %T", exporter)
		}
		// We know this is set because we are called with acquireGlobal lock held
		if globalOct.exporter != nil {
			trace.UnregisterExporter(globalOct.exporter)
		}
		if globalOct.closer != nil {
			globalOct.closer.Close()
		}

		globalOct.exporter = exporter
		globalOct.closer = closer
		
		return nil
	}
}

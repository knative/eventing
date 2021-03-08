module knative.dev/eventing

go 1.15

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cloudevents/sdk-go/v2 v2.2.0
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.4
	github.com/google/gofuzz v1.1.0
	github.com/google/mako v0.0.0-20190821191249-122f8dcef9e3
	github.com/google/uuid v1.2.0
	github.com/hashicorp/go-retryablehttp v0.6.7
	github.com/influxdata/tdigest v0.0.0-20191024211133-5d87a7585faa // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/openzipkin/zipkin-go v0.2.5
	github.com/pelletier/go-toml v1.8.0
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pkg/errors v0.9.1
	github.com/rickb777/date v1.13.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/rogpeppe/fastuuid v1.2.0
	github.com/stretchr/testify v1.6.1
	github.com/tsenart/vegeta/v12 v12.8.4
	github.com/wavesoftware/go-ensure v1.0.0
	go.opencensus.io v0.22.6
	go.opentelemetry.io/otel v0.16.0
	go.uber.org/atomic v1.7.0
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	google.golang.org/grpc v1.35.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	k8s.io/api v0.19.7
	k8s.io/apiextensions-apiserver v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/apiserver v0.19.7
	k8s.io/client-go v0.19.7
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
	knative.dev/hack v0.0.0-20210305150220-f99a25560134
	knative.dev/pkg v0.0.0-20210308052421-737401c38b22
	knative.dev/reconciler-test v0.0.0-20210305172520-cdad89dfbd9d
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2

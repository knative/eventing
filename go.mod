module knative.dev/eventing

go 1.14

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.1 // indirect
	github.com/cloudevents/sdk-go/v2 v2.0.1-0.20200630063327-b91da81265fe
	github.com/ghodss/yaml v1.0.0
	github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp v0.4.0
	github.com/google/mako v0.0.0-20190821191249-122f8dcef9e3
	github.com/google/uuid v1.1.1
	github.com/influxdata/tdigest v0.0.0-20191024211133-5d87a7585faa // indirect
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/mailru/easyjson v0.7.1-0.20191009090205-6c0755d89d1e // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/openzipkin/zipkin-go v0.2.2
	github.com/pelletier/go-toml v1.8.0
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pkg/errors v0.9.1
	github.com/robfig/cron/v3 v3.0.1
	github.com/rogpeppe/fastuuid v1.2.0
	github.com/rogpeppe/go-internal v1.5.0 // indirect
	github.com/stretchr/testify v1.5.1
	github.com/tsenart/vegeta v12.7.1-0.20190725001342-b5f4fca92137+incompatible
	github.com/wavesoftware/go-ensure v1.0.0
	go.opencensus.io v0.22.4
	go.opentelemetry.io/otel v0.2.3
	go.uber.org/atomic v1.6.0
	go.uber.org/zap v1.14.1
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	google.golang.org/grpc v1.28.0
	gopkg.in/yaml.v3 v3.0.0-20191026110619-0b21df46bc1d // indirect
	k8s.io/api v0.17.6
	k8s.io/apiextensions-apiserver v0.17.6
	k8s.io/apimachinery v0.17.6
	k8s.io/apiserver v0.17.6
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/utils v0.0.0-20200124190032-861946025e34
	knative.dev/pkg v0.0.0-20200627192328-fe0740d31f07
	knative.dev/test-infra v0.0.0-20200626234928-7fb82ece3d02
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/code-generator => k8s.io/code-generator v0.17.6
)

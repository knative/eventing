module knative.dev/eventing

go 1.13

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.1 // indirect
	github.com/cloudevents/sdk-go v1.2.0
	github.com/cloudevents/sdk-go/v2 v2.0.0-RC2
	github.com/ghodss/yaml v1.0.0
	github.com/gobuffalo/envy v1.7.1 // indirect
	github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp v0.4.0
	github.com/google/mako v0.0.0-20190821191249-122f8dcef9e3
	github.com/google/uuid v1.1.1
	github.com/influxdata/tdigest v0.0.0-20191024211133-5d87a7585faa // indirect
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/mailru/easyjson v0.7.1-0.20191009090205-6c0755d89d1e // indirect
	github.com/openzipkin/zipkin-go v0.2.2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.1.0 // indirect
	github.com/robfig/cron/v3 v3.0.1
	github.com/rogpeppe/fastuuid v1.2.0
	github.com/rogpeppe/go-internal v1.5.0 // indirect
	github.com/stretchr/testify v1.5.1
	github.com/tsenart/vegeta v12.7.1-0.20190725001342-b5f4fca92137+incompatible
	go.opencensus.io v0.22.3
	go.opentelemetry.io/otel v0.2.3
	go.uber.org/atomic v1.4.0
	go.uber.org/multierr v1.2.0 // indirect
	go.uber.org/zap v1.10.0
	gomodules.xyz/jsonpatch/v2 v2.1.0 // indirect
	google.golang.org/grpc v1.28.0
	k8s.io/api v0.16.4
	k8s.io/apiextensions-apiserver v0.16.4
	k8s.io/apimachinery v0.16.5-beta.1
	k8s.io/apiserver v0.16.4
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/utils v0.0.0-20191010214722-8d271d903fe4
	knative.dev/pkg v0.0.0-20200429233442-1ebb4d56f726
	knative.dev/test-infra v0.0.0-20200429211942-f4c4853375cf
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2
	k8s.io/api => k8s.io/api v0.16.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.16.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.4
	k8s.io/client-go => k8s.io/client-go v0.16.4
	k8s.io/code-generator => k8s.io/code-generator v0.16.4
)

module knative.dev/eventing

go 1.17

require (
	github.com/ahmetb/gen-crd-api-reference-docs v0.3.1-0.20210420163308-c1402a70e2f1
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cloudevents/conformance v0.2.0
	github.com/cloudevents/sdk-go/observability/opencensus/v2 v2.4.1
	github.com/cloudevents/sdk-go/v2 v2.4.1
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.6
	github.com/google/gofuzz v1.2.0
	github.com/google/mako v0.0.0-20190821191249-122f8dcef9e3
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.4.2
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/hashicorp/go-retryablehttp v0.6.7
	github.com/hashicorp/golang-lru v0.5.4
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/onsi/ginkgo v1.16.5
	github.com/openzipkin/zipkin-go v0.3.0
	github.com/pelletier/go-toml/v2 v2.0.0-beta.2
	github.com/phayes/freeport v0.0.0-20180830031419-95f893ade6f2
	github.com/pkg/errors v0.9.1
	github.com/rickb777/date v1.13.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/rogpeppe/fastuuid v1.2.0
	github.com/stretchr/testify v1.7.0
	github.com/tsenart/vegeta/v12 v12.8.4
	github.com/wavesoftware/go-ensure v1.0.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/otel v0.16.0
	go.uber.org/atomic v1.9.0
	go.uber.org/zap v1.19.1
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	google.golang.org/grpc v1.42.0
	google.golang.org/protobuf v1.27.1
	k8s.io/api v0.21.4
	k8s.io/apiextensions-apiserver v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/apiserver v0.21.4
	k8s.io/client-go v0.21.4
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
	knative.dev/hack v0.0.0-20211102174540-b284d49386cc
	knative.dev/hack/schema v0.0.0-20211102174540-b284d49386cc
	knative.dev/pkg v0.0.0-20211103135647-464034912f7e
	knative.dev/reconciler-test v0.0.0-20211101213739-c5f938aa9974
	sigs.k8s.io/yaml v1.3.0
)

require (
	cloud.google.com/go v0.97.0 // indirect
	cloud.google.com/go/storage v1.18.2 // indirect
	contrib.go.opencensus.io/exporter/ocagent v0.7.1-0.20200907061046-05415f1de66d // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.4.0 // indirect
	contrib.go.opencensus.io/exporter/zipkin v0.1.2 // indirect
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/blendle/zapdriver v1.3.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/cncf/udpa/go v0.0.0-20210930031921-04548b0d99d4 // indirect
	github.com/cncf/xds/go v0.0.0-20211011173535-cb28da3451f1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful v2.9.5+incompatible // indirect
	github.com/envoyproxy/go-control-plane v0.9.10-0.20210907150352-cf90f659a021 // indirect
	github.com/envoyproxy/protoc-gen-validate v0.1.0 // indirect
	github.com/evanphx/json-patch v4.9.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/go-kit/log v0.1.0 // indirect
	github.com/go-logfmt/logfmt v0.5.0 // indirect
	github.com/go-logr/logr v0.4.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.19.5 // indirect
	github.com/go-openapi/spec v0.19.6 // indirect
	github.com/go-openapi/swag v0.19.15 // indirect
	github.com/gobuffalo/flect v0.2.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-github/v27 v27.0.6 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/googleapis/gax-go/v2 v2.1.1 // indirect
	github.com/googleapis/gnostic v0.4.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/imdario/mergo v0.3.9 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/influxdata/tdigest v0.0.0-20181121200506-bf2b5ad3c0a9 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.10 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.11.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.32.1 // indirect
	github.com/prometheus/procfs v0.2.0 // indirect
	github.com/prometheus/statsd_exporter v0.21.0 // indirect
	github.com/rickb777/plural v1.2.1 // indirect
	github.com/russross/blackfriday/v2 v2.0.1 // indirect
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/spf13/cobra v1.1.3 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	go.uber.org/automaxprocs v1.4.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/crypto v0.0.0-20210920023735-84f357641f63 // indirect
	golang.org/x/mod v0.4.2 // indirect
	golang.org/x/net v0.0.0-20211101193420-4a448f8816b3 // indirect
	golang.org/x/oauth2 v0.0.0-20211028175245-ba495a64dcb5 // indirect
	golang.org/x/sys v0.0.0-20211025201205-69cdffdb9359 // indirect
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/tools v0.1.7 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/api v0.60.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20211021150943-2b146023228c // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/code-generator v0.21.4 // indirect
	k8s.io/gengo v0.0.0-20210915205010-39e73c8a59cd // indirect
	k8s.io/klog v1.0.0 // indirect
	k8s.io/klog/v2 v2.8.0 // indirect
	k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
)

replace github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.2

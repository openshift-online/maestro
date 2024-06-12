module github.com/openshift-online/maestro

go 1.21

require (
	github.com/Masterminds/squirrel v1.5.3
	github.com/auth0/go-jwt-middleware v0.0.0-20190805220309-36081240882b
	github.com/buraksezer/consistent v0.10.0
	github.com/bwmarrin/snowflake v0.3.0
	github.com/bxcodec/faker/v3 v3.2.0
	github.com/cespare/xxhash v1.1.0
	github.com/cloudevents/sdk-go/v2 v2.15.3-0.20240329120647-e6a74efbacbf
	github.com/deckarep/golang-set/v2 v2.6.0
	github.com/docker/go-healthcheck v0.1.0
	github.com/getsentry/sentry-go v0.20.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-gormigrate/gormigrate/v2 v2.0.0
	github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/golang/glog v1.2.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/jinzhu/inflection v1.0.0
	github.com/lib/pq v1.10.7
	github.com/mendsley/gojwk v0.0.0-20141217222730-4d5ec6e58103
	github.com/onsi/ginkgo/v2 v2.17.1
	github.com/onsi/gomega v1.32.0
	github.com/openshift-online/ocm-sdk-go v0.1.334
	github.com/prometheus/client_golang v1.18.0
	github.com/segmentio/ksuid v1.0.2
	github.com/spf13/cobra v1.8.0
	github.com/spf13/pflag v1.0.5
	github.com/yaacov/tree-search-language v0.0.0-20190923184055-1c2dad2e354b
	google.golang.org/grpc v1.62.1
	google.golang.org/protobuf v1.33.0
	gopkg.in/resty.v1 v1.12.0
	gorm.io/datatypes v1.2.0
	gorm.io/driver/postgres v1.5.0
	gorm.io/gorm v1.24.7-0.20230306060331-85eaf9eeda11
	k8s.io/apimachinery v0.29.4
	k8s.io/client-go v0.29.4
	k8s.io/component-base v0.29.3
	k8s.io/klog/v2 v2.120.1
	open-cluster-management.io/api v0.13.1-0.20240605083248-f9e7f50520fc
	open-cluster-management.io/ocm v0.13.1-0.20240612012446-8e792c14d8f4
	open-cluster-management.io/sdk-go v0.13.1-0.20240606075054-7671bb086504
)

require (
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/antlr/antlr4 v0.0.0-20200712162734-eb1adaa8a7a6 // indirect
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230305170008-8188dc5388df // indirect
	github.com/asaskevich/govalidator v0.0.0-20200428143746-21a406dcc535 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cloudevents/sdk-go/protocol/kafka_confluent/v2 v2.0.0-20240413090539-7fef29478991 // indirect
	github.com/cloudevents/sdk-go/protocol/mqtt_paho/v2 v2.0.0-20231030012137-0836a524e995 // indirect
	github.com/confluentinc/confluent-kafka-go/v2 v2.3.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/eclipse/paho.golang v0.11.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/evanphx/json-patch v5.9.0+incompatible // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-sql-driver/mysql v1.7.0 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/cel-go v0.17.7 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20210720184732-4bb14d4b1be1 // indirect
	github.com/gorilla/css v1.0.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.16.0 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgx/v5 v5.3.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/microcosm-cc/bluemonday v1.0.23 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v0.0.0-20231218131639-7a5aa77cc72d // indirect
	github.com/openshift/client-go v0.0.0-20231218140158-47f6d749b9d9 // indirect
	github.com/openshift/library-go v0.0.0-20240116081341-964bcb3f545c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/profile v1.3.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/smartystreets/goconvey v1.8.1 // indirect
	github.com/stoewer/go-strcase v1.2.0 // indirect
	go.etcd.io/etcd/api/v3 v3.5.10 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.10 // indirect
	go.etcd.io/etcd/client/v3 v3.5.10 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.42.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.45.0 // indirect
	go.opentelemetry.io/otel v1.19.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.19.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.19.0 // indirect
	go.opentelemetry.io/otel/metric v1.19.0 // indirect
	go.opentelemetry.io/otel/sdk v1.19.0 // indirect
	go.opentelemetry.io/otel/trace v1.19.0 // indirect
	go.opentelemetry.io/proto/otlp v1.0.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1 // indirect
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/oauth2 v0.16.0 // indirect
	golang.org/x/sync v0.6.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/term v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.17.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto v0.0.0-20240123012728-ef4313101c80 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240123012728-ef4313101c80 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240123012728-ef4313101c80 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gorm.io/driver/mysql v1.4.7 // indirect
	k8s.io/api v0.29.4 // indirect
	k8s.io/apiextensions-apiserver v0.29.3 // indirect
	k8s.io/apiserver v0.29.3 // indirect
	k8s.io/kms v0.29.3 // indirect
	k8s.io/kube-aggregator v0.29.3 // indirect
	k8s.io/kube-openapi v0.0.0-20231010175941-2dd684a91f00 // indirect
	k8s.io/utils v0.0.0-20240310230437-4693a0247e57 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.28.0 // indirect
	sigs.k8s.io/controller-runtime v0.17.3 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/kube-storage-version-migrator v0.0.6-0.20230721195810-5c8923c5ff96 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

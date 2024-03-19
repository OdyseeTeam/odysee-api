module github.com/OdyseeTeam/odysee-api

go 1.20

require (
	github.com/ClickHouse/clickhouse-go v1.4.5
	github.com/OdyseeTeam/player-server v0.8.0
	github.com/Pallinder/go-randomdata v1.2.0
	github.com/alecthomas/kong v0.2.17
	github.com/aws/aws-sdk-go v1.44.247
	github.com/aws/aws-sdk-go-v2 v1.18.0
	github.com/aws/aws-sdk-go-v2/config v1.18.25
	github.com/aws/aws-sdk-go-v2/credentials v1.13.24
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.11.67
	github.com/aws/aws-sdk-go-v2/service/s3 v1.33.1
	github.com/bluele/factory-go v0.0.1
	github.com/btcsuite/btcd v0.22.0-beta
	github.com/coreos/go-oidc v2.2.1+incompatible
	github.com/dgraph-io/ristretto v0.1.0
	github.com/getsentry/sentry-go v0.13.0
	github.com/go-chi/chi/v5 v5.0.8
	github.com/go-chi/cors v1.2.1
	github.com/go-chi/jwtauth/v5 v5.1.0
	github.com/go-chi/render v1.0.3
	github.com/go-redsync/redsync/v4 v4.8.1
	github.com/gorilla/mux v1.8.0
	github.com/h2non/filetype v1.1.3
	github.com/hashicorp/go-cleanhttp v0.5.1
	github.com/hashicorp/go-retryablehttp v0.5.4
	github.com/hibiken/asynq v0.24.1
	github.com/lbryio/lbry.go/v2 v2.7.2-0.20220815204100-2adb8af5b68c
	github.com/lbryio/lbry.go/v3 v3.0.1-beta
	github.com/lbryio/reflector.go v1.1.3-0.20220730181028-f5d30b1a6e79
	github.com/lbryio/types v0.0.0-20220224142228-73610f6654a6
	github.com/lestrrat-go/jwx/v2 v2.0.21
	github.com/lib/pq v1.10.7
	github.com/mitchellh/mapstructure v1.5.0
	github.com/nsf/jsondiff v0.0.0-20190712045011-8443391ee9b6
	github.com/ory/dockertest/v3 v3.10.0
	github.com/oschwald/geoip2-golang v1.5.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.14.0
	github.com/prometheus/client_model v0.3.0
	github.com/redis/go-redis/v9 v9.0.5
	github.com/rs/cors v1.7.0
	github.com/rubenv/sql-migrate v1.1.1
	github.com/shopspring/decimal v1.3.1
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/cast v1.5.0
	github.com/spf13/cobra v1.4.0
	github.com/spf13/viper v1.9.0
	github.com/sqlc-dev/pqtype v0.2.0
	github.com/stretchr/testify v1.9.0
	github.com/tus/tusd v1.11.0
	github.com/volatiletech/null v8.0.0+incompatible
	github.com/volatiletech/sqlboiler v3.4.0+incompatible
	github.com/ybbus/jsonrpc v2.1.2+incompatible
	go.uber.org/zap v1.21.0
	goa.design/goa/v3 v3.5.2
	goa.design/plugins/v3 v3.4.3
	golang.org/x/oauth2 v0.6.0
	golang.org/x/sync v0.1.0
	gopkg.in/vansante/go-ffprobe.v2 v2.1.0
	gopkg.in/yaml.v2 v2.4.0
	logur.dev/logur v0.17.0
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/ajg/form v1.5.1 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.4.10 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.13.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.33 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.27 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.0.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.9.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.1.28 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.27 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.14.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.12.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.14.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.19.0 // indirect
	github.com/aws/smithy-go v1.13.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bluele/gcache v0.0.2 // indirect
	github.com/bmizerany/pat v0.0.0-20170815010413-6226ea591a40 // indirect
	github.com/brk0v/directio v0.0.0-20190225130936-69406e757cf7 // indirect
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f // indirect
	github.com/btcsuite/btcutil v1.0.3-0.20201208143702-a53e38424cce // indirect
	github.com/btcsuite/go-socks v0.0.0-20170105172521-4720035b7bfd // indirect
	github.com/btcsuite/websocket v0.0.0-20150119174127-31079b680792 // indirect
	github.com/bytedance/sonic v1.9.1 // indirect
	github.com/cenkalti/backoff/v4 v4.1.3 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/cloudflare/golz4 v0.0.0-20150217214814-ef862a3cdc58 // indirect
	github.com/cockroachdb/errors v1.9.0 // indirect
	github.com/cockroachdb/logtags v0.0.0-20211118104740-dabe8e521a4f // indirect
	github.com/cockroachdb/redact v1.1.3 // indirect
	github.com/containerd/continuity v0.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dimfeld/httppath v0.0.0-20170720192232-ee938bf73598 // indirect
	github.com/dimfeld/httptreemux/v5 v5.3.0 // indirect
	github.com/docker/cli v24.0.1+incompatible // indirect
	github.com/docker/docker v24.0.1+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/fatih/structs v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/gin-gonic/gin v1.9.1 // indirect
	github.com/go-errors/errors v1.4.2 // indirect
	github.com/go-gorp/gorp/v3 v3.1.0 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-openapi/swag v0.21.1 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.14.0 // indirect
	github.com/go-redis/redis/v8 v8.11.5 // indirect
	github.com/go-sql-driver/mysql v1.6.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/gomodule/redigo v1.8.8 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/karrick/godirwalk v1.17.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.4 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/lbryio/chainquery v1.9.0 // indirect
	github.com/lbryio/lbry.go v1.1.2 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/lestrrat-go/blackmagic v1.0.2 // indirect
	github.com/lestrrat-go/httpcc v1.0.1 // indirect
	github.com/lestrrat-go/httprc v1.0.5 // indirect
	github.com/lestrrat-go/iter v1.0.2 // indirect
	github.com/lestrrat-go/option v1.0.1 // indirect
	github.com/lyoshenka/bencode v0.0.0-20180323155644-b7abd7672df5 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/manveru/faker v0.0.0-20171103152722-9fbc68a78c4d // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/onsi/gomega v1.18.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/opencontainers/runc v1.1.5 // indirect
	github.com/oschwald/maxminddb-golang v1.8.0 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/pelletier/go-toml/v2 v2.0.8 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/pquerna/cachecontrol v0.1.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/rogpeppe/go-internal v1.9.0 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/sergi/go-diff v1.3.1 // indirect
	github.com/smartystreets/assertions v1.1.1 // indirect
	github.com/spf13/afero v1.9.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	github.com/volatiletech/inflect v0.0.1 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/zach-klippenstein/goregen v0.0.0-20160303162051-795b5e3961ea // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/mod v0.9.0 // indirect
	golang.org/x/net v0.21.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.7.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/DATA-DOG/go-sqlmock.v1 v1.3.0 // indirect
	gopkg.in/ini.v1 v1.66.4 // indirect
	gopkg.in/nullbio/null.v6 v6.0.0-20161116030900-40264a2e6b79 // indirect
	gopkg.in/square/go-jose.v2 v2.6.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/floostack/transcoder => github.com/anbsky/transcoder v1.2.0

replace github.com/lbryio/lbry.go/v3 => github.com/anbsky/lbry.go/v3 v3.0.6

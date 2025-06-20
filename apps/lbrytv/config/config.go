package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	cfg "github.com/OdyseeTeam/odysee-api/config"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

const (
	lbrynetServers           = "LbrynetServers"
	deprecatedLbrynetSetting = "Lbrynet"
	configName               = "oapi"
)

type LoggingOpts struct {
	level  string
	format string
}

var Config *cfg.ConfigWrapper

func ProjectRoot() string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(ex)
}

// IsProduction is true if we are running in a production environment.
func IsProduction() bool {
	return Config.IsProduction()
}

// GetInternalAPIHost returns the address of internal-api server.
func GetInternalAPIHost() string {
	return Config.Viper.GetString("InternalAPIHost")
}

// GetOauthProviderURL returns the address of OAuth provider.
func GetOauthProviderURL() string {
	return Config.Viper.GetStringMapString("oauth")["providerurl"]
}

// GetOauthClientID returns the address of OAuth client ID.
func GetOauthClientID() string {
	return Config.Viper.GetStringMapString("oauth")["clientid"]
}

// GetOauthTokenURL returns the address of OAuth token retrieval endpoint.
func GetOauthTokenURL() string {
	cfg := Config.Viper.GetStringMapString("oauth")
	return cfg["providerurl"] + cfg["tokenpath"]
}

// GetRedisLockerOpts returns Redis connection options in the official redis client format.
func GetRedisLockerOpts() (*redis.Options, error) {
	opts, err := redis.ParseURL(Config.Viper.GetString("RedisLocker"))
	if err != nil {
		return nil, err
	}
	return opts, nil
}

// GetRedisBusOpts returns Redis connection options in the Redis URL format.
func GetRedisBusOpts() (asynq.RedisConnOpt, error) {
	return asynq.ParseRedisURI(Config.Viper.GetString("RedisBus"))
}

// GetAsynqueryRequestsConnOpts returns Redis connection options for incoming asynquery queue.
func GetAsynqueryRequestsConnOpts() (asynq.RedisConnOpt, error) {
	return asynq.ParseRedisURI(Config.Viper.GetString("AsynqueryRequestsConnURL"))
}

func GetSturdyCacheMaster() string {
	return Config.Viper.GetString("sturdycache.master")
}

func GetSturdyCacheReplicas() []string {
	return Config.Viper.GetStringSlice("sturdycache.replicas")
}

func GetSturdyCachePassword() string {
	return Config.Viper.GetString("sturdycache.password")
}

// GetDatabase returns postgresql database server connection config.
func GetDatabase() cfg.DBConfig {
	return Config.GetDatabase()
}

// GetSentryDSN returns sentry.io service DSN.
func GetSentryDSN() string {
	return Config.Viper.GetString("SentryDSN")
}

// GetPublishSourceDir returns directory for storing published files before they're uploaded to lbrynet.
// The directory needs to be accessed by the running SDK instance.
func GetPublishSourceDir() string {
	return Config.Viper.GetString("PublishSourceDir")
}

// GetGeoPublishSourceDir returns directory for storing files created by publish v3 endpoint for all odysee-api instances.
// The directory needs to be accessed by the running SDK instance.
func GetGeoPublishSourceDir() string {
	return Config.Viper.GetString("GeoPublishSourceDir")
}

// GetGeoPublishConcurrency sets the number of simultaneously processed uploads per each API instance.
func GetGeoPublishConcurrency() int {
	Config.Viper.SetDefault("GeoPublishConcurrency", 3)
	return Config.Viper.GetInt("GeoPublishConcurrency")
}

// ShouldLogResponses enables or disables full SDK responses logging. Produces a lot of logging, use for debugging only.
func ShouldLogResponses() bool {
	return Config.Viper.GetBool("ShouldLogResponses")
}

// GetPaidTokenPrivKey returns absolute path to the private RSA key for generating paid tokens.
func GetPaidTokenPrivKey() string {
	return Config.Viper.GetString("PaidTokenPrivKey")
}

// GetUploadTokenPrivateKey returns absolute path to the private RSA key for generating paid tokens.
func GetUploadTokenPrivateKey() string {
	return strings.TrimSpace(Config.Viper.GetString("UploadTokenPrivateKey"))
}

// GetUploadServiceURL returns url to the v4 upload service.
func GetUploadServiceURL() string {
	return Config.Viper.GetString("UploadServiceURL")
}

// GetStreamsV5 returns config map for v5 streams endpoint.
func GetStreamsV5() map[string]string {
	return Config.Viper.GetStringMapString("StreamsV5")
}

// GetStreamsV6 returns config map for v6 streams endpoint.
func GetStreamsV6() map[string]string {
	return Config.Viper.GetStringMapString("StreamsV6")
}

func GetArfleetCDN() string {
	return Config.Viper.GetString("ArfleetCDN")
}

func GetArfleetEnabled() bool {
	return Config.Viper.GetBool("ArfleetEnabled")
}

// GetReflectorUpstream returns config map for publish reflector server.
func GetReflectorUpstream() *viper.Viper {
	return Config.Viper.Sub("ReflectorUpstream")
}

// GetAddress sets API HTTP binding address.
func GetAddress() string {
	return Config.Viper.GetString("Address")
}

func GetSimpleAdminToken() string {
	return Config.Viper.GetString("SimpleAdminToken")
}

// GetLbrynetServers returns the names/addresses of every SDK server.
func GetLbrynetServers() map[string]string {
	if Config.Viper.GetString(deprecatedLbrynetSetting) != "" &&
		len(Config.Viper.GetStringMapString(lbrynetServers)) > 0 {
		panic(fmt.Sprintf("only one of %s and %s can be set", deprecatedLbrynetSetting, lbrynetServers))
	}

	if len(Config.Viper.GetStringMapString(lbrynetServers)) > 0 {
		return Config.Viper.GetStringMapString(lbrynetServers)
	} else if Config.Viper.GetString(deprecatedLbrynetSetting) != "" {
		return map[string]string{"sdk": Config.Viper.GetString(deprecatedLbrynetSetting)}
	} else {
		servers, err := models.LbrynetServers().AllG()
		if err != nil {
			panic(fmt.Sprintf("Could not retrieve lbrynet server list from db: %s", err))
		}
		if len(servers) == 0 {
			panic("There are no servers listed in the db and config is not set.")
		}
		return nil
	}
}

func GetTokenCacheTimeout() time.Duration {
	return Config.Viper.GetDuration("TokenCacheTimeout")
}

func GetCacheGetterRetries() int {
	return Config.Viper.GetInt("CacheGetterRetries")
}

func GetCacheGetterInterval() time.Duration {
	return Config.Viper.GetDuration("CacheGetterInterval")
}

func GetCORSDomains() []string {
	return Config.Viper.GetStringSlice("CORSDomains")
}

func GetRPCTimeout(method string) *time.Duration {
	ts := Config.Viper.GetStringMapString("RPCTimeouts")
	if ts != nil {
		if t, ok := ts[method]; ok {
			d := cast.ToDuration(t)
			return &d
		}
	}
	return nil
}

func GetProfiling() bool {
	return Config.Viper.GetBool("Profiling")
}

func GetLoggingOpts() LoggingOpts {
	return LoggingOpts{
		level:  Config.Viper.GetString("logging.level"),
		format: Config.Viper.GetString("logging.format"),
	}
}

func Override(key string, value interface{}) {
	Config.Override(key, value)
}

func RestoreOverridden() {
	Config.RestoreOverridden()
}

func (o LoggingOpts) Level() string {
	return o.level
}

func (o LoggingOpts) Format() string {
	return o.format
}

func init() {
	Config = cfg.ReadConfig(configName)
	c := Config
	c.Viper.SetConfigName(configName)

	c.Viper.SetEnvPrefix("LW")
	c.Viper.SetDefault("Debug", false)

	c.Viper.BindEnv("Debug")
	c.Viper.BindEnv("Lbrynet")
	c.Viper.BindEnv("SentryDSN")
	c.Viper.BindEnv("DatabaseDSN")

	c.Viper.SetDefault("Address", ":8080")
	c.Viper.SetDefault("Host", "http://localhost:8080")
	c.Viper.SetDefault("Logging", map[string]string{"level": "debug", "format": "console"})
	c.Viper.SetDefault("CacheGetterRetries", 3)
	c.Viper.SetDefault("CacheGetterInterval", 1*time.Second)
}

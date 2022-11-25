package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	cfg "github.com/OdyseeTeam/odysee-api/config"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/go-redis/redis/v8"
	"github.com/hibiken/asynq"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

const (
	lbrynetServers           = "LbrynetServers"
	deprecatedLbrynetSetting = "Lbrynet"
	configName               = "oapi"
)

var Config *cfg.ConfigWrapper

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
}

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

// GetRedisOpts returns Redis connection options in the official redis client format.
func GetRedisOpts() (*redis.Options, error) {
	opts, err := redis.ParseURL(Config.Viper.GetString("redis"))
	if err != nil {
		return nil, err
	}
	return opts, nil
}

// GetAsynqRedisOpts rreturns Redis connection options ready for asynq package.
func GetAsynqRedisOpts() (asynq.RedisConnOpt, error) {
	redisOpts, err := asynq.ParseRedisURI(Config.Viper.GetString("redis"))
	if err != nil {
		return nil, err
	}
	return redisOpts, nil
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

// GetStreamsV5 returns config map for v5 streams endpoint.
func GetStreamsV5() map[string]string {
	return Config.Viper.GetStringMapString("StreamsV5")
}

// GetReflectorUpstream returns config map for publish reflector server.
func GetReflectorUpstream() map[string]string {
	return Config.Viper.GetStringMapString("ReflectorUpstream")
}

// GetAddress sets API HTTP binding address.
func GetAddress() string {
	return Config.Viper.GetString("Address")
}

// GetLbrynetServers returns the names/addresses of every SDK server.
func GetLbrynetServers() map[string]string {
	if Config.Viper.GetString(deprecatedLbrynetSetting) != "" &&
		len(Config.Viper.GetStringMapString(lbrynetServers)) > 0 {
		logrus.Panicf("Both %s and %s are set. This is a highlander situation...there can be only 1.", deprecatedLbrynetSetting, lbrynetServers)
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

func GetLbrynetXServer() string {
	return Config.Viper.GetString("LbrynetXServer")
}

func GetLbrynetXPercentage() int {
	return Config.Viper.GetInt("LbrynetXPercentage")
}

func GetTokenCacheTimeout() time.Duration {
	return Config.Viper.GetDuration("TokenCacheTimeout") * time.Second
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

func Override(key string, value interface{}) {
	Config.Override(key, value)
}

func RestoreOverridden() {
	Config.RestoreOverridden()
}

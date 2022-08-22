package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	cfg "github.com/OdyseeTeam/odysee-api/config"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/go-redis/redis/v8"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

const (
	lbrynetServers    = "LbrynetServers"
	deprecatedLbrynet = "Lbrynet"
	configName        = "lbrytv"
)

// overriddenValues stores overridden v values
// and is initialized as an empty map in the read method
var (
	overriddenValues map[string]interface{}
	once             sync.Once
	Config           *cfg.ConfigWrapper
)

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
	c.Viper.SetDefault("FreeContentURL", "http://localhost:8080/content/")
	c.Viper.SetDefault("ReflectorTimeout", int64(10))
	c.Viper.SetDefault("RefractorTimeout", int64(10))
}

func ProjectRoot() string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(ex)
}

// IsProduction is true if we are running in a production environment
func IsProduction() bool {
	return Config.IsProduction()
}

// GetInternalAPIHost returns the address of internal-api server
func GetInternalAPIHost() string {
	return Config.Viper.GetString("InternalAPIHost")
}

// GetOauthProviderURL returns the address of OAuth provider
func GetOauthProviderURL() string {
	return Config.Viper.GetStringMapString("oauth")["providerurl"]
}

// GetOauthClientID returns the address of OAuth client ID
func GetOauthClientID() string {
	return Config.Viper.GetStringMapString("oauth")["clientid"]
}

// GetOauthTokenURL returns the address of OAuth token retrieval endpoint
func GetOauthTokenURL() string {
	cfg := Config.Viper.GetStringMapString("oauth")
	return cfg["providerurl"] + cfg["tokenpath"]
}

// GetRedisOpts returns the address of OAuth client ID
func GetRedisOpts() (*redis.Options, error) {
	opts, err := redis.ParseURL(Config.Viper.GetString("redis"))
	if err != nil {
		return nil, err
	}
	return opts, nil
}

// GetDatabase returns postgresql database server connection config
func GetDatabase() cfg.DBConfig {
	return Config.GetDatabase()
}

// GetSentryDSN returns sentry.io service DSN
func GetSentryDSN() string {
	return Config.Viper.GetString("SentryDSN")
}

// GetPublishSourceDir returns directory for storing published files before they're uploaded to lbrynet.
// The directory needs to be accessed by the running SDK instance.
func GetPublishSourceDir() string {
	return Config.Viper.GetString("PublishSourceDir")
}

// GetBlobFilesDir returns directory where SDK instance stores blob files.
func GetBlobFilesDir() string {
	return Config.Viper.GetString("BlobFilesDir")
}

// GetReflectorAddress returns reflector address in the format of host:port.
func GetReflectorAddress() string {
	return Config.Viper.GetString("ReflectorAddress")
}

// ShouldLogResponses enables or disables full SDK responses logging
func ShouldLogResponses() bool {
	return Config.Viper.GetBool("ShouldLogResponses")
}

// GetPaidTokenPrivKey returns absolute path to the private RSA key for generating paid tokens
func GetPaidTokenPrivKey() string {
	return Config.Viper.GetString("PaidTokenPrivKey")
}

// GetStreamsV5 returns config map for streams endpoint v5
func GetStreamsV5() map[string]string {
	return Config.Viper.GetStringMapString("StreamsV5")
}

// GetAddress determines address to bind http API server to
func GetAddress() string {
	return Config.Viper.GetString("Address")
}

//GetLbrynetServers returns the names/addresses of every SDK server
func GetLbrynetServers() map[string]string {
	if Config.Viper.GetString(deprecatedLbrynet) != "" &&
		len(Config.Viper.GetStringMapString(lbrynetServers)) > 0 {
		logrus.Panicf("Both %s and %s are set. This is a highlander situation...there can be only 1.", deprecatedLbrynet, lbrynetServers)
	}

	if len(Config.Viper.GetStringMapString(lbrynetServers)) > 0 {
		return Config.Viper.GetStringMapString(lbrynetServers)
	} else if Config.Viper.GetString(deprecatedLbrynet) != "" {
		return map[string]string{"sdk": Config.Viper.GetString(deprecatedLbrynet)}
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

func Override(key string, value interface{}) {
	Config.Override(key, value)
}

func RestoreOverridden() {
	Config.RestoreOverridden()
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

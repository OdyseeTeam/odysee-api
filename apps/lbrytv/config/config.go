package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	cfg "github.com/OdyseeTeam/odysee-api/config"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/configng"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

const (
	lbrynetServers           = "LbrynetServers"
	deprecatedLbrynetSetting = "Lbrynet"
	configName               = "oapi"

	sdkHealthCheckEnabled       = "SDKHealthCheckEnabled"
	sdkReconnectEnabled         = "SDKReconnectEnabled"
	sdkHealthCheckInterval      = "SDKHealthCheckInterval"
	sdkHealthProbeTimeout       = "SDKHealthProbeTimeout"
	sdkHealthFailureThreshold   = "SDKHealthFailureThreshold"
	sdkReconnectCooldown        = "SDKReconnectCooldown"
	sdkReconnectLeaseTTL        = "SDKReconnectLeaseTTL"
	sdkReconnectObservation     = "SDKReconnectObservationWindow"
	sdkReconnectMaxConcurrent   = "SDKReconnectMaxConcurrent"
	sdkReconnectMaxAttempts     = "SDKReconnectMaxAttempts"
	sdkUnhealthyFleetFraction   = "SDKUnhealthyFleetFraction"
	sdkHealthSentinelURL        = "SDKHealthSentinelURL"
	defaultSDKHealthSentinelURL = "what#19b9c243bea0c45175e6a6027911abbad53e983e"
	sdkMinHealthCheckInterval   = 5 * time.Second
	sdkMinHealthProbeTimeout    = time.Second
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

func GetTranscoderS3Config() *configng.S3Config {
	if !Config.Viper.IsSet("TranscoderS3") {
		return nil
	}
	var s3cfg configng.S3Config
	if err := Config.Viper.UnmarshalKey("TranscoderS3", &s3cfg); err != nil {
		return nil
	}
	return &s3cfg
}

func GetTranscoderDBConfig() *configng.PostgresConfig {
	if !Config.Viper.IsSet("TranscoderDB") {
		return nil
	}
	var pcfg configng.PostgresConfig
	if err := Config.Viper.UnmarshalKey("TranscoderDB", &pcfg); err != nil {
		return nil
	}
	return &pcfg
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

func GetSDKHealthCheckEnabled() bool {
	return Config.Viper.GetBool(sdkHealthCheckEnabled)
}

func GetSDKReconnectEnabled() bool {
	return Config.Viper.GetBool(sdkReconnectEnabled)
}

func GetSDKHealthCheckInterval() time.Duration {
	return Config.Viper.GetDuration(sdkHealthCheckInterval)
}

func GetSDKHealthProbeTimeout() time.Duration {
	return Config.Viper.GetDuration(sdkHealthProbeTimeout)
}

func GetSDKHealthFailureThreshold() int {
	return Config.Viper.GetInt(sdkHealthFailureThreshold)
}

func GetSDKReconnectCooldown() time.Duration {
	return Config.Viper.GetDuration(sdkReconnectCooldown)
}

func GetSDKReconnectLeaseTTL() time.Duration {
	return Config.Viper.GetDuration(sdkReconnectLeaseTTL)
}

func GetSDKReconnectObservationWindow() time.Duration {
	return Config.Viper.GetDuration(sdkReconnectObservation)
}

func GetSDKReconnectMaxConcurrent() int {
	return Config.Viper.GetInt(sdkReconnectMaxConcurrent)
}

func GetSDKReconnectMaxAttempts() int {
	return Config.Viper.GetInt(sdkReconnectMaxAttempts)
}

func GetSDKUnhealthyFleetFraction() float64 {
	return Config.Viper.GetFloat64(sdkUnhealthyFleetFraction)
}

func GetSDKHealthSentinelURL() string {
	return Config.Viper.GetString(sdkHealthSentinelURL)
}

func ValidateSDKHealthConfig() error {
	if GetSDKHealthCheckInterval() < sdkMinHealthCheckInterval {
		return fmt.Errorf("%s must be at least %s", sdkHealthCheckInterval, sdkMinHealthCheckInterval)
	}
	if GetSDKHealthProbeTimeout() < sdkMinHealthProbeTimeout {
		return fmt.Errorf("%s must be at least %s", sdkHealthProbeTimeout, sdkMinHealthProbeTimeout)
	}
	if GetSDKHealthFailureThreshold() <= 0 {
		return fmt.Errorf("%s must be positive", sdkHealthFailureThreshold)
	}
	if GetSDKUnhealthyFleetFraction() <= 0 || GetSDKUnhealthyFleetFraction() > 1 {
		return fmt.Errorf("%s must be between 0 and 1", sdkUnhealthyFleetFraction)
	}
	if GetSDKHealthSentinelURL() == "" {
		return fmt.Errorf("%s must not be empty", sdkHealthSentinelURL)
	}
	if !GetSDKReconnectEnabled() {
		return nil
	}
	if GetSDKReconnectCooldown() <= 0 {
		return fmt.Errorf("%s must be positive", sdkReconnectCooldown)
	}
	if GetSDKReconnectMaxConcurrent() <= 0 {
		return fmt.Errorf("%s must be positive", sdkReconnectMaxConcurrent)
	}
	if GetSDKReconnectMaxAttempts() <= 0 {
		return fmt.Errorf("%s must be positive", sdkReconnectMaxAttempts)
	}
	if GetSDKReconnectLeaseTTL() <= GetSDKHealthCheckInterval() {
		return fmt.Errorf("%s must be greater than %s", sdkReconnectLeaseTTL, sdkHealthCheckInterval)
	}
	if GetSDKReconnectObservationWindow() <= GetSDKHealthCheckInterval() {
		return fmt.Errorf("%s must be greater than %s", sdkReconnectObservation, sdkHealthCheckInterval)
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
	c.Viper.SetDefault(sdkHealthCheckEnabled, false)
	c.Viper.SetDefault(sdkReconnectEnabled, false)
	c.Viper.SetDefault(sdkHealthCheckInterval, 45*time.Second)
	c.Viper.SetDefault(sdkHealthProbeTimeout, 4*time.Second)
	c.Viper.SetDefault(sdkHealthFailureThreshold, 3)
	c.Viper.SetDefault(sdkReconnectCooldown, 10*time.Minute)
	c.Viper.SetDefault(sdkReconnectLeaseTTL, 2*time.Minute)
	c.Viper.SetDefault(sdkReconnectObservation, 3*time.Minute)
	c.Viper.SetDefault(sdkReconnectMaxConcurrent, 1)
	c.Viper.SetDefault(sdkReconnectMaxAttempts, 3)
	c.Viper.SetDefault(sdkUnhealthyFleetFraction, 0.3)
	c.Viper.SetDefault(sdkHealthSentinelURL, defaultSDKHealthSentinelURL)

	bindSDKEnv(c.Viper, sdkHealthCheckEnabled, "SDK_HEALTH_CHECK_ENABLED")
	bindSDKEnv(c.Viper, sdkReconnectEnabled, "SDK_RECONNECT_ENABLED")
	bindSDKEnv(c.Viper, sdkHealthCheckInterval, "SDK_HEALTH_CHECK_INTERVAL")
	bindSDKEnv(c.Viper, sdkHealthProbeTimeout, "SDK_HEALTH_PROBE_TIMEOUT")
	bindSDKEnv(c.Viper, sdkHealthFailureThreshold, "SDK_HEALTH_FAILURE_THRESHOLD")
	bindSDKEnv(c.Viper, sdkReconnectCooldown, "SDK_RECONNECT_COOLDOWN")
	bindSDKEnv(c.Viper, sdkReconnectLeaseTTL, "SDK_RECONNECT_LEASE_TTL")
	bindSDKEnv(c.Viper, sdkReconnectObservation, "SDK_RECONNECT_OBSERVATION_WINDOW")
	bindSDKEnv(c.Viper, sdkReconnectMaxConcurrent, "SDK_RECONNECT_MAX_CONCURRENT")
	bindSDKEnv(c.Viper, sdkReconnectMaxAttempts, "SDK_RECONNECT_MAX_ATTEMPTS")
	bindSDKEnv(c.Viper, sdkUnhealthyFleetFraction, "SDK_UNHEALTHY_FLEET_FRACTION")
	bindSDKEnv(c.Viper, sdkHealthSentinelURL, "SDK_HEALTH_SENTINEL_URL")
}

func bindSDKEnv(v *viper.Viper, key string, snakeName string) {
	legacyName := "LW_" + strings.ToUpper(key)
	err := v.BindEnv(key, "LW_"+snakeName, legacyName)
	if err != nil {
		panic(err)
	}
}

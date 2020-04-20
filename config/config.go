package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/lbryio/lbrytv/models"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type configWrapper struct {
	Viper      *viper.Viper
	overridden map[string]interface{}
}

type DBConfig struct {
	Connection string
	DBName     string
	Options    string
}

const lbrynetServers = "LbrynetServers"
const deprecatedLbrynet = "Lbrynet"

var once sync.Once
var Config *configWrapper

// overriddenValues stores overridden v values
// and is initialized as an empty map in the read method
var overriddenValues map[string]interface{}

func init() {
	Config = GetConfig()
}

func GetConfig() *configWrapper {
	once.Do(func() {
		Config = NewConfig()
	})
	return Config
}

func NewConfig() *configWrapper {
	c := &configWrapper{}
	c.Init()
	c.Read()
	return c
}

func (c *configWrapper) Init() {
	c.overridden = make(map[string]interface{})
	c.Viper = viper.New()

	c.Viper.SetEnvPrefix("LW")
	c.Viper.SetDefault("Debug", false)

	c.Viper.BindEnv("Debug")
	c.Viper.BindEnv("Lbrynet")
	c.Viper.BindEnv("SentryDSN")
	c.Viper.BindEnv("DatabaseDSN")
	c.Viper.BindEnv(lbrynetServers)

	c.Viper.SetDefault("Address", ":8080")
	c.Viper.SetDefault("Host", "http://localhost:8080")
	c.Viper.SetDefault("BaseContentURL", "http://localhost:8080/content/")
	c.Viper.SetDefault("ReflectorTimeout", int64(10))
	c.Viper.SetDefault("RefractorTimeout", int64(10))

	c.Viper.SetConfigName("lbrytv") // name of config file (without extension)

	c.Viper.AddConfigPath(os.Getenv("LBRYTV_CONFIG_DIR"))
	c.Viper.AddConfigPath(ProjectRoot())
	c.Viper.AddConfigPath(".")
	c.Viper.AddConfigPath("..")
	c.Viper.AddConfigPath("../..")
	c.Viper.AddConfigPath("$HOME/.lbrytv")
}

func (c *configWrapper) Read() {
	err := c.Viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
}

// IsProduction is true if we are running in a production environment
func IsProduction() bool {
	return !Config.Viper.GetBool("Debug")
}

func ProjectRoot() string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(ex)
}

// Override sets a setting key value to whatever you supply.
// Useful in tests:
//	config.Override("Lbrynet", "http://www.google.com:8080/api/proxy")
//	defer config.RestoreOverridden()
//	...
func Override(key string, value interface{}) {
	Config.overridden[key] = Config.Viper.Get(key)
	Config.Viper.Set(key, value)
}

// RestoreOverridden restores original v values overridden by Override
func RestoreOverridden() {
	c := GetConfig()
	v := c.Viper
	if len(c.overridden) == 0 {
		return
	}
	for k, val := range c.overridden {
		v.Set(k, val)
	}
	c.overridden = make(map[string]interface{})
}

// Defined config variables go here

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
			panic("Could not retrieve lbrynet server list from db and config is not set.")
		}
		if len(servers) == 0 {
			panic("There are no servers listed in the db and config is not set.")
		}
		return nil
	}
}

// GetInternalAPIHost returns the address of internal-api server
func GetInternalAPIHost() string {
	return Config.Viper.GetString("InternalAPIHost")
}

// GetDatabase returns postgresql database server connection config
func GetDatabase() DBConfig {
	var config DBConfig
	Config.Viper.UnmarshalKey("Database", &config)
	config.Connection = Config.Viper.GetString("DatabaseDSN")
	return config
}

// GetSentryDSN returns sentry.io service DSN
func GetSentryDSN() string {
	return Config.Viper.GetString("SentryDSN")
}

// GetProjectURL returns publicly accessible URL for the project
func GetProjectURL() string {
	return Config.Viper.GetString("ProjectURL")
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

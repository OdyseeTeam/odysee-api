package config

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/spf13/viper"
)

type ConfigWrapper struct {
	Viper      *viper.Viper
	overridden map[string]interface{}
	ReadDone   bool
}

type DBConfig struct {
	Connection string
	DBName     string
	Options    string
}

var once sync.Once
var Config *ConfigWrapper

// overriddenValues stores overridden v values
// and is initialized as an empty map in the read method
var overriddenValues map[string]interface{}

func init() {
	Config = GetConfig()
}

func GetConfig() *ConfigWrapper {
	once.Do(func() {
		Config = NewConfig()
	})
	return Config
}

func NewConfig() *ConfigWrapper {
	c := &ConfigWrapper{}
	c.Init()
	c.Read()
	return c
}

func (c *ConfigWrapper) Init() {
	c.overridden = make(map[string]interface{})
	c.Viper = viper.New()

	c.Viper.SetEnvPrefix("LW")
	c.Viper.SetDefault("Debug", false)

	c.Viper.BindEnv("Debug")
	c.Viper.BindEnv("Lbrynet")
	c.Viper.SetDefault("Lbrynet", "http://localhost:5279/")

	c.Viper.SetDefault("Address", ":8080")
	c.Viper.SetDefault("Host", "http://localhost:8080")
	c.Viper.SetDefault("BaseContentURL", "http://localhost:8080/content/")

	c.Viper.SetDefault("AccountsEnabled", false)
	c.Viper.BindEnv("AccountsEnabled")

	c.Viper.SetConfigName("lbrytv") // name of config file (without extension)

	c.Viper.AddConfigPath(os.Getenv("LBRYTV_CONFIG_DIR"))
	c.Viper.AddConfigPath(ProjectRoot())
	c.Viper.AddConfigPath(".")
	c.Viper.AddConfigPath("..")
	c.Viper.AddConfigPath("../..")
	c.Viper.AddConfigPath("$HOME/.lbrytv")
}

func (c *ConfigWrapper) Read() {
	err := c.Viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
	c.ReadDone = true
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

// Concrete v variables go here

// AccountsEnabled enables or disables accounts subsystem
func AccountsEnabled() bool {
	return Config.Viper.GetBool("AccountsEnabled")
}

// GetAddress determines address to bind http API server to
func GetAddress() string {
	return Config.Viper.GetString("Address")
}

// MetricsAddress determines address to bind metrics HTTP server to
func MetricsAddress() string {
	return Config.Viper.GetString("MetricsAddress")
}

// MetricsPath determines the path to bind metrics HTTP server to
func MetricsPath() string {
	return Config.Viper.GetString("MetricsPath")
}

// GetLbrynet returns the address of SDK server to use
func GetLbrynet() string {
	return Config.Viper.GetString("Lbrynet")
}

// GetInternalAPIHost returns the address of internal-api server
func GetInternalAPIHost() string {
	return Config.Viper.GetString("InternalAPIHost")
}

// GetDatabase returns postgresql database server connection config
func GetDatabase() DBConfig {
	var config DBConfig
	Config.Viper.UnmarshalKey("Database", &config)
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

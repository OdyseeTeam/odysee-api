package config

import (
	"github.com/spf13/viper"
)

type ConfigWrapper struct {
	Viper      *viper.Viper
	configName string
	overridden map[string]interface{}
}

type DBConfig struct {
	Connection string
	DBName     string
	Options    string
}

func NewConfig() *ConfigWrapper {
	return &ConfigWrapper{
		overridden: map[string]interface{}{},
		Viper:      viper.New(),
	}
}

// ReadConfig initializes a ConfigWrapper and reads `configName`
func ReadConfig(configName string) *ConfigWrapper {
	c := NewConfig()
	c.configName = configName
	c.initPaths()
	c.read()
	return c
}

func (c *ConfigWrapper) initPaths() {
	c.Viper.SetConfigName(c.configName)
	c.Viper.AddConfigPath("./config/")
	c.Viper.AddConfigPath(".")
	c.Viper.AddConfigPath("..")
	c.Viper.AddConfigPath("../../")
	c.Viper.AddConfigPath("../../../")
}

func (c *ConfigWrapper) read() {
	err := c.Viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
}

// IsProduction is true if we are running in a production environment
func (c *ConfigWrapper) IsProduction() bool {
	return !c.Viper.GetBool("Debug")
}

// GetDatabase returns postgresql database server connection config
func (c *ConfigWrapper) GetDatabase() DBConfig {
	var dbc DBConfig
	c.Viper.UnmarshalKey("Database", &dbc)
	dbc.Connection = c.Viper.GetString("DatabaseDSN")
	return dbc
}

// Override sets a setting key value to whatever you supply.
// Useful in tests:
//	config.Override("Lbrynet", "http://www.google.com:8080/api/proxy")
//	defer config.RestoreOverridden()
//	...
func (c *ConfigWrapper) Override(key string, value interface{}) {
	c.overridden[key] = c.Viper.Get(key)
	c.Viper.Set(key, value)
}

// RestoreOverridden restores original v values overridden by Override
func (c *ConfigWrapper) RestoreOverridden() {
	v := c.Viper
	if len(c.overridden) == 0 {
		return
	}
	for k, val := range c.overridden {
		v.Set(k, val)
	}
	c.overridden = make(map[string]interface{})
}

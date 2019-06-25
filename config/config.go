package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Settings stores project settings such as download paths, host prefix for URLs and so on
var Settings = viper.New()

// overriddenValues stores overridden settings values
// and is initialized as an empty map in the read method
var overriddenValues map[string]interface{}

func init() {
	read()
}

// read parses `lbrytv.yml`
func read() {
	Settings.SetEnvPrefix("LW")
	Settings.BindEnv("Debug")
	Settings.SetDefault("Debug", false)
	Settings.BindEnv("Lbrynet")
	Settings.SetDefault("Lbrynet", "http://localhost:5279/")

	Settings.SetDefault("Address", ":8080")
	Settings.SetDefault("Host", "http://localhost:8080")
	Settings.SetDefault("BaseContentURL", "http://localhost:8080/content/")

	Settings.SetDefault("IsAccountV1Enabled", true)

	Settings.SetConfigName("lbrytv") // name of config file (without extension)
	Settings.AddConfigPath("./")
	Settings.AddConfigPath("../")
	Settings.AddConfigPath("$HOME/.lbrytv")
	err := Settings.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error reading config file: %s", err))
	}
	overriddenValues = make(map[string]interface{})
}

// IsProduction is true if we are running in a production environment
func IsProduction() bool {
	return !Settings.GetBool("Debug")
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
	overriddenValues[key] = Settings.Get(key)
	Settings.Set(key, value)
}

// RestoreOverridden restores original settings values overridden by Override
func RestoreOverridden() {
	if len(overriddenValues) == 0 {
		return
	}
	for k, v := range overriddenValues {
		Settings.Set(k, v)
	}
	overriddenValues = make(map[string]interface{})
}

// Concrete settings variables go here

// IsAccountV1Enabled enables or disables Account Subsystem V1 (database + plain auth_token)
func IsAccountV1Enabled() bool {
	return Settings.GetBool("IsAccountV1Enabled")
}

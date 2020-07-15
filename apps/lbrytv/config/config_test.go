package config

import (
	"os"
	"testing"

	cfg "github.com/lbryio/lbrytv/config"
	"github.com/stretchr/testify/assert"
)

func TestOverrideInEnv(t *testing.T) {
	os.Setenv("LW_LBRYNETSERVERS", `{"z": "http://abc:5279/"}`)
	oldConfig := Config
	Config = cfg.ReadConfig(configName)
	assert.Equal(t, map[string]string{"z": "http://abc:5279/"}, GetLbrynetServers())
	Config = oldConfig
}

func TestGetLbrynetServers(t *testing.T) {
	Config.Override("LbrynetServers", map[string]string{
		"sdk1": "http://lbrynet1:5279/",
		"sdk2": "http://lbrynet2:5279/",
		"sdk3": "http://lbrynet3:5279/",
	})
	defer Config.RestoreOverridden()
	assert.Equal(t, map[string]string{
		"sdk1": "http://lbrynet1:5279/",
		"sdk2": "http://lbrynet2:5279/",
		"sdk3": "http://lbrynet3:5279/",
	}, GetLbrynetServers())
}

func TestGetLbrynetServersNoDB(t *testing.T) {
	if Config.Viper.GetString(deprecatedLbrynet) != "" &&
		len(Config.Viper.GetStringMapString(lbrynetServers)) > 0 {
		t.Fatalf("Both %s and %s are set. This is a highlander situation...there can be only one.", deprecatedLbrynet, lbrynetServers)
	}
}

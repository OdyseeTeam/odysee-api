package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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

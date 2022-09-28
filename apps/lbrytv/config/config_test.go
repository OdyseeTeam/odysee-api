package config

import (
	"testing"
	"time"

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
	if Config.Viper.GetString(deprecatedLbrynetSetting) != "" &&
		len(Config.Viper.GetStringMapString(lbrynetServers)) > 0 {
		t.Fatalf("Both %s and %s are set. This is a highlander situation...there can be only one.", deprecatedLbrynetSetting, lbrynetServers)
	}
}

func TestGetTokenCacheTimeout(t *testing.T) {
	Config.Override("TokenCacheTimeout", 325)
	defer Config.RestoreOverridden()
	assert.Equal(t, 325*time.Second, GetTokenCacheTimeout())
}

func TestGetRPCTimeout(t *testing.T) {
	Config.Override("RPCTimeouts", map[string]string{
		"txo_list": "12s",
		"resolve":  "200ms",
	})
	defer Config.RestoreOverridden()

	assert.Equal(t, 12*time.Second, *GetRPCTimeout("txo_list"))
	assert.Equal(t, 200*time.Millisecond, *GetRPCTimeout("resolve"))
	assert.Nil(t, GetRPCTimeout("random_method"))
}

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
	Config.Override("TokenCacheTimeout", "325s")
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

func TestSDKHealthConfigSnakeCaseEnv(t *testing.T) {
	t.Setenv("LW_SDK_HEALTH_CHECK_ENABLED", "true")
	t.Setenv("LW_SDK_HEALTH_CHECK_INTERVAL", "30s")

	assert.True(t, GetSDKHealthCheckEnabled())
	assert.Equal(t, 30*time.Second, GetSDKHealthCheckInterval())
}

func TestSDKHealthConfigLegacyEnvStillWorks(t *testing.T) {
	t.Setenv("LW_SDKHEALTHCHECKENABLED", "true")

	assert.True(t, GetSDKHealthCheckEnabled())
}

func TestValidateSDKHealthConfig(t *testing.T) {
	assert.NoError(t, ValidateSDKHealthConfig())

	Config.Override(sdkHealthCheckInterval, 0)
	assert.Error(t, ValidateSDKHealthConfig())
	Config.RestoreOverridden()

	Config.Override(sdkHealthCheckInterval, 45)
	assert.Error(t, ValidateSDKHealthConfig())
	Config.RestoreOverridden()

	Config.Override(sdkReconnectEnabled, false)
	Config.Override(sdkReconnectMaxConcurrent, 0)
	Config.Override(sdkReconnectLeaseTTL, time.Nanosecond)
	assert.NoError(t, ValidateSDKHealthConfig())
	Config.RestoreOverridden()

	Config.Override(sdkReconnectEnabled, true)
	Config.Override(sdkHealthCheckInterval, 5*time.Second)
	Config.Override(sdkReconnectLeaseTTL, 5*time.Second)
	defer Config.RestoreOverridden()
	assert.Error(t, ValidateSDKHealthConfig())
}

func TestSDKHealthConfigDefaults(t *testing.T) {
	assert.False(t, GetSDKHealthCheckEnabled())
	assert.False(t, GetSDKReconnectEnabled())
	assert.Equal(t, 45*time.Second, GetSDKHealthCheckInterval())
	assert.Equal(t, 4*time.Second, GetSDKHealthProbeTimeout())
	assert.Equal(t, 3, GetSDKHealthFailureThreshold())
	assert.Equal(t, 10*time.Minute, GetSDKReconnectCooldown())
	assert.Equal(t, 2*time.Minute, GetSDKReconnectLeaseTTL())
	assert.Equal(t, 3*time.Minute, GetSDKReconnectObservationWindow())
	assert.Equal(t, 1, GetSDKReconnectMaxConcurrent())
	assert.Equal(t, 3, GetSDKReconnectMaxAttempts())
	assert.Equal(t, 0.3, GetSDKUnhealthyFleetFraction())
	assert.Equal(t, defaultSDKHealthSentinelURL, GetSDKHealthSentinelURL())
}

package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOverride(t *testing.T) {
	c := NewConfig()
	err := c.Viper.ReadConfig(strings.NewReader("Lbrynet: http://localhost:5279"))
	require.Nil(t, err)
	originalSetting := c.Viper.Get("Lbrynet")
	c.Override("Lbrynet", "http://www.google.com:8080/api/proxy")
	assert.Equal(t, "http://www.google.com:8080/api/proxy", c.Viper.Get("Lbrynet"))
	c.RestoreOverridden()
	assert.Equal(t, originalSetting, c.Viper.Get("Lbrynet"))
	assert.Empty(t, c.overridden)
}

func TestIsProduction(t *testing.T) {
	c := NewConfig()
	c.Override("Debug", false)
	assert.True(t, c.IsProduction())
	c.Override("Debug", true)
	assert.False(t, c.IsProduction())
}

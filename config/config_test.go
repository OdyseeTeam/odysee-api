package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOverride(t *testing.T) {
	originalSetting := Settings.Get("Lbrynet")
	Override("Lbrynet", "http://www.google.com:8080/api/proxy")
	assert.Equal(t, "http://www.google.com:8080/api/proxy", Settings.Get("Lbrynet"))
	RestoreOverridden()
	assert.Equal(t, originalSetting, Settings.Get("Lbrynet"))
	assert.Empty(t, overriddenValues)
}

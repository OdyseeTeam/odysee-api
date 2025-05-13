package forklift

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

const envReflectorConfig = "REFLECTOR_CONFIG"

var ErrMissingEnv = errors.New("REFLECTOR_CONFIG env var is not set")

type TestHelper struct {
	ReflectorConfig *viper.Viper
}

func NewTestHelper(t *testing.T) (*TestHelper, error) {
	th := &TestHelper{}
	os.Setenv("PATH", os.Getenv("PATH")+":/opt/homebrew/bin")
	envCfg := os.Getenv(envReflectorConfig)

	if envCfg == "" {
		return nil, ErrMissingEnv
	}

	th.ReflectorConfig = DecodeSecretViperConfig(t, envReflectorConfig)
	return th, nil
}

func DecodeSecretViperConfig(t *testing.T, secretEnvName string) *viper.Viper {
	require := require.New(t)
	secretValueEncoded := os.Getenv(secretEnvName)
	require.NotEmpty(secretValueEncoded)
	secretValue, err := base64.StdEncoding.DecodeString(secretValueEncoded)
	require.NoError(err)
	v := viper.New()
	v.SetConfigType("yaml")
	err = v.ReadConfig(bytes.NewBuffer(secretValue))
	require.NoError(err)
	return v
}

package forklift

import (
	"errors"
	"os"
	"testing"

	"gopkg.in/yaml.v2"
)

const envReflectorUplinkConfig = "REFLECTOR_UPLINK"

var ErrMissingEnv = errors.New("reflector uplink config env var is not set")

type TestHelper struct {
	ReflectorConfig map[string]string
}

func NewTestHelper(_ *testing.T) (*TestHelper, error) {
	th := &TestHelper{}
	os.Setenv("PATH", os.Getenv("PATH")+":/opt/homebrew/bin")
	parsedCfg := map[string]string{}
	envCfg := os.Getenv(envReflectorUplinkConfig)

	if envCfg == "" {
		return nil, ErrMissingEnv
	}

	err := yaml.Unmarshal([]byte(envCfg), &parsedCfg)
	if err != nil {
		return nil, err
	}
	th.ReflectorConfig = parsedCfg
	return th, nil
}

package forklift

import (
	"errors"
	"os"
	"testing"

	"gopkg.in/yaml.v2"
)

const envReflectorUplinkConfig = "REFLECTOR_UPLINK"

var ErrMissingEnv = errors.New("reflector uplink config env var is not set")

type ForkliftTestHelper struct {
	ReflectorConfig map[string]string
}

func (th *ForkliftTestHelper) Setup(t *testing.T) error {
	os.Setenv("PATH", os.Getenv("PATH")+":/opt/homebrew/bin")
	parsedCfg := map[string]string{}
	envCfg := os.Getenv(envReflectorUplinkConfig)

	if envCfg == "" {
		return ErrMissingEnv
	}

	err := yaml.Unmarshal([]byte(envCfg), &parsedCfg)
	if err != nil {
		return err
	}
	th.ReflectorConfig = parsedCfg
	return nil
}

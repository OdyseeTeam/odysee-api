package forklift

import (
	"errors"
	"os"

	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"

	"gopkg.in/yaml.v2"
)

const envReflectorUplinkConfig = "REFLECTOR_UPLINK"

var ErrMissingEnv = errors.New("reflector uplink config env var is not set")

type ForkliftTestHelper struct{}

func (s *ForkliftTestHelper) Setup() error {
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
	config.Override("ReflectorUpstream", parsedCfg)
	return nil
}

type StreamCreateResponse struct {
	Height int    `json:"height"`
	Hex    string `json:"hex"`
	Inputs []struct {
		Address       string `json:"address"`
		Amount        string `json:"amount"`
		Confirmations int    `json:"confirmations"`
		Height        int    `json:"height"`
		Nout          int    `json:"nout"`
		Timestamp     int    `json:"timestamp"`
		Txid          string `json:"txid"`
		Type          string `json:"type"`
	} `json:"inputs"`
	Outputs []struct {
		Address       string `json:"address"`
		Amount        string `json:"amount"`
		ClaimID       string `json:"claim_id,omitempty"`
		ClaimOp       string `json:"claim_op,omitempty"`
		Confirmations int    `json:"confirmations"`
		Height        int    `json:"height"`
		Meta          struct {
		} `json:"meta,omitempty"`
		Name           string      `json:"name,omitempty"`
		NormalizedName string      `json:"normalized_name,omitempty"`
		Nout           int         `json:"nout"`
		PermanentURL   string      `json:"permanent_url,omitempty"`
		Timestamp      interface{} `json:"timestamp"`
		Txid           string      `json:"txid"`
		Type           string      `json:"type"`
		Value          struct {
			Source struct {
				Hash      string `json:"hash"`
				MediaType string `json:"media_type"`
				Name      string `json:"name"`
				SdHash    string `json:"sd_hash"`
				Size      string `json:"size"`
			} `json:"source"`
			StreamType string `json:"stream_type"`
		} `json:"value,omitempty"`
		ValueType string `json:"value_type,omitempty"`
	} `json:"outputs"`
	TotalFee    string `json:"total_fee"`
	TotalInput  string `json:"total_input"`
	TotalOutput string `json:"total_output"`
	Txid        string `json:"txid"`
}

package wallet

import (
	"context"
	"fmt"
	"os"

	"github.com/lbryio/lbrytv/apps/lbrytv/config"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	envClientID     = "OAUTH_TEST_CLIENT_ID"
	envClientSecret = "OAUTH_TEST_CLIENT_SECRET"
)

var ErrTestCredentialsMissing = fmt.Errorf("test oauth client env vars %s and %s are missing", envClientID, envClientSecret)

// GetTestToken is for easily retrieving tokens that can be used in tests utilizing authentication subsystem.
func GetTestToken() (*oauth2.Token, error) {
	clientID := os.Getenv(envClientID)
	clientSecret := os.Getenv(envClientSecret)
	if clientID == "" || clientSecret == "" {
		return nil, ErrTestCredentialsMissing
	}

	ctx := context.Background()
	conf := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     config.GetOauthTokenURL(),
	}

	return conf.Token(ctx)
}

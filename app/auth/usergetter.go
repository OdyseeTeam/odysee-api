package auth

import (
	"net/http"

	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/ip"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/lbryio/transcoder/pkg/logging"
)

type universalUserGetter struct {
	logger   logging.KVLogger
	auther   Authenticator
	provider Provider
}

func NewUniversalUserGetter(auther Authenticator, provider Provider, logger logging.KVLogger) *universalUserGetter {
	return &universalUserGetter{
		auther:   auther,
		provider: provider,
		logger:   logger,
	}
}

func (g *universalUserGetter) GetFromRequest(r *http.Request) (*models.User, error) {
	log := g.logger
	token, err := g.auther.GetTokenFromRequest(r)
	// No oauth token present in request, try legacy method
	if errors.Is(err, wallet.ErrNoAuthInfo) {
		// TODO: Remove this pathway after legacy tokens go away.
		if token, ok := r.Header[wallet.LegacyTokenHeader]; ok {
			addr := ip.ForRequest(r)
			user, err := g.provider(token[0], addr)
			if err != nil {
				log.Info("user authentication failed", "err", err, "method", "token")
				return nil, err
			}
			if user == nil {
				err := wallet.ErrNoAuthInfo
				log.Info("unauthorized user", "err", err, "method", "token")
				return nil, err
			}
			log.Debug("user authenticated", "user", user.ID, "method", "token")
			return user, nil
		}
		return nil, errors.Err(wallet.ErrNoAuthInfo)
	} else if err != nil {
		return nil, err
	}

	user, err := g.auther.Authenticate(token, ip.ForRequest(r))
	if err != nil {
		log.Info("user authentication failed", "err", err, "method", "oauth")
		return nil, err
	}
	if user == nil {
		err := wallet.ErrNoAuthInfo
		log.Info("unauthorized user", "err", err, "method", "oauth")
		return nil, err
	}
	log.Debug("user authenticated", "user", user.ID, "method", "oauth")
	return user, nil
}

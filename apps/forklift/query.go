package forklift

import (
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
)

type AsyncQuery struct {
}

type AsyncQueryHandler struct {
}

func NewAsyncQueryHandler(logger logging.KVLogger) (*AsyncQueryHandler, error) {

	c := &AsyncQueryHandler{}
	return c, nil
}

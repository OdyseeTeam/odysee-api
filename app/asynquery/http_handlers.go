package asynquery

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"

	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/app/rpcerrors"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"

	"github.com/gorilla/mux"
)

type QueryHandler struct {
	m      CallManager
	logger logging.KVLogger
}

// Status returns status of the upload.
// Possible response HTTP codes:
// - 202: upload is currently being processed
// - 200: upload has been fully processed and is immediately available on the network. Normal JSON-RPC SDK response is provided in the body
// - 409: SDK returned an error and upload cannot be processed. Error details are provided in the response body
// - 404, 403: upload not found or does not belong to the user
func (h QueryHandler) Status(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	user, err := auth.FromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	q, err := h.m.get(context.Background(), id, user.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			rpcerrors.Write(w, err)
		}
		return
	}

	switch q.Status {
	case models.QueryStatusSucceeded:
		w.WriteHeader(http.StatusOK)
		w.Write(q.Response.JSON)
	case models.QueryStatusFailed:
		if !q.Response.IsZero() {
			w.Write(q.Response.JSON)
		} else {
			rpcerrors.Write(w, errors.Err(q.Error))
		}
	default:
		w.WriteHeader(http.StatusAccepted)
	}
}

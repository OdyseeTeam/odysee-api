package asynquery

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/responses"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/keybox"
	"github.com/OdyseeTeam/odysee-api/pkg/logging"
	"github.com/OdyseeTeam/odysee-api/pkg/rpcerrors"
	"github.com/mitchellh/mapstructure"
	"github.com/ybbus/jsonrpc/v2"

	"github.com/gorilla/mux"
)

const (
	StatusAuthError          = "auth_error"
	StatusUploadTokenCreated = "upload_token_created"
	StatusQueryCreated       = "query_created"
)

type QueryHandler struct {
	callManager      *CallManager
	logger           logging.KVLogger
	keyfob           *keybox.Keyfob
	uploadServiceURL string
}

func NewHandler(callManager *CallManager, logger logging.KVLogger, keyfob *keybox.Keyfob, uploadServiceURL string) QueryHandler {
	return QueryHandler{
		callManager:      callManager,
		logger:           logger,
		keyfob:           keyfob,
		uploadServiceURL: uploadServiceURL,
	}
}

type UploadTokenCreatedPayload struct {
	Token    string `json:"token"`
	Location string `json:"location"`
}

type QueryCreatedPayload struct {
	QueryID string `json:"query_id"`
}

type Response struct {
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

func (h QueryHandler) CreateUpload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uploadType, ok := vars["type"]
	if !ok {
		uploadType = "uploads"
	}
	targetURL := h.uploadServiceURL + uploadType + "/"
	responses.AddJSONContentType(w)
	u, err := auth.FromRequest(r)
	if err != nil {
		resp, jerr := json.Marshal(Response{
			Status: StatusAuthError,
			Error:  err.Error(),
		})
		if jerr != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(resp)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	token, err := h.keyfob.GenerateToken(int32(u.ID), time.Now().Add(48*time.Hour))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp, err := json.Marshal(Response{
		Status:  StatusUploadTokenCreated,
		Payload: UploadTokenCreatedPayload{Token: token, Location: targetURL},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("location", h.uploadServiceURL)
	w.Write(resp)
}

func (h QueryHandler) CreateQuery(w http.ResponseWriter, r *http.Request) {
	responses.AddJSONContentType(w)
	u, err := auth.FromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		responses.WriteJSON(w, Response{
			Status: StatusAuthError,
			Error:  err.Error(),
		})
		return
	}
	var rpcReq *jsonrpc.RPCRequest
	if err := json.NewDecoder(r.Body).Decode(&rpcReq); err != nil {
		rpcerrors.Write(w, rpcerrors.NewJSONParseError(err))
		return
	}
	if !isMethodAllowed(rpcReq.Method) {
		rpcerrors.Write(w, rpcerrors.NewMethodNotAllowedError(errors.Err("forbidden method")))
		return
	}
	aq, err := h.callManager.Call(u.ID, rpcReq)
	if err != nil {
		rpcerrors.Write(w, rpcerrors.NewInternalError(err))
		return
	}
	w.WriteHeader(http.StatusCreated)
	responses.WriteJSON(w, Response{
		Status: StatusQueryCreated,
		Payload: QueryCreatedPayload{
			QueryID: aq.ID,
		},
	})
}

// Get returns current details for the upload.
// Possible response HTTP codes:
// - 202: upload is currently being processed
// - 200: upload has been fully processed and is immediately available on the network. Normal JSON-RPC SDK response is provided in the body
// - 409: SDK returned an error and upload cannot be processed. Error details are provided in the response body
// - 404, 403: upload not found or does not belong to the user
func (h QueryHandler) Get(w http.ResponseWriter, r *http.Request) {
	queryID, ok := mux.Vars(r)["id"]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	log := h.logger.With("query_id", queryID)
	user, err := auth.FromRequest(r)
	if err != nil {
		log.Info("unauthorized request")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	aq, err := h.callManager.getQueryRecord(context.TODO(), queryParams{
		queryID: queryID, userID: user.ID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Info("query not found")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		log.Info("query retrieval error", "err", err)
		rpcerrors.Write(w, err)
		return
	}

	switch aq.Status {
	case models.AsynqueryStatusSucceeded:
		w.WriteHeader(http.StatusOK)
		w.Write(aq.Response.JSON)
	case models.AsynqueryStatusFailed:
		w.WriteHeader(http.StatusOK)
		if !aq.Response.IsZero() {
			w.Write(aq.Response.JSON)
		} else {
			rpcerrors.Write(w, errors.Err(aq.Error))
		}
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (r *Response) UnmarshalJSON(data []byte) error {
	type responseAlias Response // Alias to avoid recursion
	aux := &responseAlias{
		Payload: json.RawMessage{},
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*r = Response(*aux)
	var payload any
	switch r.Status {
	case StatusQueryCreated:
		payload = QueryCreatedPayload{}
	case StatusUploadTokenCreated:
		payload = UploadTokenCreatedPayload{}
	default:
		return errors.Err("unknown status")
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           &payload,
		TagName:          "json",
		WeaklyTypedInput: true,
	})
	if err != nil {
		return fmt.Errorf("error configuring payload decoder: %w", err)
	}

	if err := decoder.Decode(r.Payload); err != nil {
		return fmt.Errorf("error decoding payload: %w", err)
	}
	r.Payload = payload

	return nil
}

func isMethodAllowed(method string) bool {
	for _, m := range allowedSDKMethods() {
		if m == method {
			return true
		}
	}
	return false
}

func allowedSDKMethods() [2]string {
	return [...]string{
		query.MethodStreamCreate,
		query.MethodStreamUpdate,
	}
}

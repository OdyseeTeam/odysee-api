package status

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/auth"
	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/ip"
	"github.com/OdyseeTeam/odysee-api/internal/monitor"
	"github.com/OdyseeTeam/odysee-api/internal/responses"
	"github.com/OdyseeTeam/odysee-api/models"

	"github.com/gorilla/mux"
	"github.com/ybbus/jsonrpc/v2"
)

var logger = monitor.NewModuleLogger("status")

var PlayerServers = []string{
	"https://use-p1.odycdn.com",
	"https://use-p2.odycdn.com",
	"https://use-p3.odycdn.com",
	"https://player.odycdn.com",
	"https://eu-p1.odycdn.com",
}

var (
	cachedResponse *statusResponse
	lastUpdate     time.Time
	resolveURL     = "what#19b9c243bea0c45175e6a6027911abbad53e983e"
)

const (
	statusOK            = "ok"
	statusNotReady      = "not_ready"
	statusOffline       = "offline"
	statusFailing       = "failing"
	statusCacheValidity = 120 * time.Second
)

type serverItem struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type serverList []*serverItem

type userData struct {
	ID                    int    `json:"id"`
	AssignedLbrynetServer string `json:"assigned_lbrynet_server"`
}

type statusResponse struct {
	Timestamp    string                `json:"timestamp"`
	Services     map[string]serverList `json:"services"`
	GeneralState string                `json:"general_state"`
	User         *userData             `json:"user,omitempty"`
}

type whoAmIResponse struct {
	Timestamp      string            `json:"timestamp"`
	DetectedIP     string            `json:"detected_ip"`
	UserID         string            `json:"user_id"`
	SDK            string            `json:"sdk"`
	RequestHeaders map[string]string `json:"request_headers"`
	RemoteIP       string            `json:"remote_ip"`
}

func InstallRoutes(router *mux.Router) {
	router.HandleFunc("/status", StatusV2).Methods(http.MethodGet)
	router.HandleFunc("/whoami", WhoAmI).Methods(http.MethodGet)
	router.Methods(http.MethodOptions).HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})
}

func StatusV2(w http.ResponseWriter, r *http.Request) {
	respStatus := http.StatusOK

	response := statusResponse{
		Timestamp:    fmt.Sprintf("%v", time.Now().UTC()),
		Services:     nil,
		GeneralState: statusOK,
	}
	failureDetected := false

	var (
		userID        int
		lbrynetServer *models.LbrynetServer
	)
	rt := sdkrouter.New(config.GetLbrynetServers())
	user, err := auth.FromRequest(r)
	if err != nil || user == nil {
		lbrynetServer = rt.RandomServer()
	} else {
		lbrynetServer = sdkrouter.GetLbrynetServer(user)
		userID = user.ID
	}

	srv := serverItem{Name: lbrynetServer.Name, Status: statusOK}

	c := query.NewCaller(lbrynetServer.Address, userID)
	if query.HasCache(r) {
		c.Cache = query.CacheFromRequest(r)
	}

	rpcRes, err := c.Call(r.Context(), jsonrpc.NewRequest(query.MethodResolve, map[string]interface{}{"urls": resolveURL}))

	if err != nil {
		srv.Error = err.Error()
		srv.Status = statusOffline
		failureDetected = true
		logger.Log().Errorf("status call resolve is failing: %s", err)
	} else if rpcRes.Error != nil {
		srv.Error = rpcRes.Error.Message
		srv.Status = statusNotReady
		failureDetected = true
		logger.Log().Errorf("status call resolve is failing: %s", err)
	} else {
		if user != nil {
			response.User = &userData{
				ID:                    user.ID,
				AssignedLbrynetServer: lbrynetServer.Name,
			}
		}
	}

	response.Services = map[string]serverList{
		"lbrynet": []*serverItem{&srv},
	}

	if failureDetected {
		response.GeneralState = statusFailing
	}

	responses.AddJSONContentType(w)
	w.WriteHeader(respStatus)
	respByte, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		logger.Log().Error(err)
	}
	w.Write(respByte)
}

func WhoAmI(w http.ResponseWriter, r *http.Request) {
	res := whoAmIResponse{
		Timestamp:      time.Now().Format(time.RFC3339),
		DetectedIP:     ip.ForRequest(r),
		RequestHeaders: map[string]string{},
		RemoteIP:       r.RemoteAddr,
	}
	cu, err := auth.GetCurrentUserData(r.Context())
	if err == nil && cu.User() != nil {
		res.UserID = strconv.Itoa(cu.User().ID)
		res.SDK = cu.User().R.LbrynetServer.Name
	}

	for k := range r.Header {
		if k == wallet.AuthorizationHeader || k == wallet.LegacyTokenHeader || k == "Cookie" {
			continue
		}
		res.RequestHeaders[k] = r.Header.Get(k)
	}
	responses.WriteJSON(w, res)
}

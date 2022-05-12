package status

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/query"
	"github.com/lbryio/lbrytv/app/query/cache"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/responses"
	"github.com/lbryio/lbrytv/models"
	"github.com/ybbus/jsonrpc"

	"github.com/jinzhu/copier"
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

func GetStatus(w http.ResponseWriter, req *http.Request) {
	respStatus := http.StatusOK
	var response statusResponse

	if cachedResponse != nil && lastUpdate.After(time.Now().Add(statusCacheValidity)) {
		//response = *cachedResponse
		copier.Copy(&response, cachedResponse)
	} else {
		services := map[string]serverList{
			"lbrynet": {},
			"player":  {},
		}
		response = statusResponse{
			Timestamp:    fmt.Sprintf("%v", time.Now().UTC()),
			Services:     services,
			GeneralState: statusOK,
		}
		failureDetected := false

		sdks := sdkrouter.FromRequest(req).GetAll()
		for _, s := range sdks {
			services["lbrynet"] = append(services["lbrynet"], &serverItem{Name: s.Name, Status: statusOK})
		}

		for _, ps := range PlayerServers {
			r, err := http.Get(ps)
			srv := serverItem{Name: ps, Status: statusOK}
			if err != nil {
				srv.Error = fmt.Sprintf("%v", err)
				srv.Status = statusOffline
				failureDetected = true
			} else if r.StatusCode != http.StatusNotFound {
				srv.Status = statusNotReady
				srv.Error = fmt.Sprintf("http status %v", r.StatusCode)
				failureDetected = true
			}
			services["player"] = append(services["player"], &srv)
		}
		if failureDetected {
			response.GeneralState = statusFailing
		}
		cachedResponse = &response
		lastUpdate = time.Now()
	}

	responses.AddJSONContentType(w)
	w.WriteHeader(respStatus)
	respByte, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		logger.Log().Error(err)
	}
	w.Write(respByte)
}

func GetStatusV2(w http.ResponseWriter, r *http.Request) {
	respStatus := http.StatusOK
	var response statusResponse

	response = statusResponse{
		Timestamp:    fmt.Sprintf("%v", time.Now().UTC()),
		Services:     nil,
		GeneralState: statusOK,
	}
	failureDetected := false

	var (
		userID        int
		qCache        *cache.Cache
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

	if cache.IsOnRequest(r) {
		qCache = cache.FromRequest(r)
	}

	c := query.NewCaller(lbrynetServer.Address, userID)
	c.Cache = qCache
	rpcRes, err := c.Call(jsonrpc.NewRequest("resolve", map[string]interface{}{"urls": resolveURL}))

	if err != nil {
		srv.Error = err.Error()
		srv.Status = statusOffline
		failureDetected = true
		logger.Log().Error("we're failing: ", err)
	} else if rpcRes.Error != nil {
		srv.Error = rpcRes.Error.Message
		srv.Status = statusNotReady
		failureDetected = true
		logger.Log().Error("we're failing: ", err)
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

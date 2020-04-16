package status

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/sdkrouter"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/responses"

	"github.com/jinzhu/copier"
)

var logger = monitor.NewModuleLogger("status")

var PlayerServers = []string{
	"https://player1.lbry.tv",
	"https://player2.lbry.tv",
	"https://player3.lbry.tv",
	"https://player4.lbry.tv",
	"https://player5.lbry.tv",
}

var (
	cachedResponse *statusResponse
	lastUpdate     time.Time
)

const (
	statusOK            = "ok"
	statusNotReady      = "not_ready"
	statusOffline       = "offline"
	statusFailing       = "failing"
	statusCacheValidity = 120 * time.Second
)

type ServerItem struct {
	Address string `json:"address"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}
type ServerList []ServerItem
type statusResponse map[string]interface{}

func GetStatus(w http.ResponseWriter, req *http.Request) {
	respStatus := http.StatusOK
	var response statusResponse

	if cachedResponse != nil && lastUpdate.After(time.Now().Add(statusCacheValidity)) {
		//response = *cachedResponse
		copier.Copy(&response, cachedResponse)
	} else {
		services := map[string]ServerList{
			"lbrynet": {},
			"player":  {},
		}
		response = statusResponse{
			"timestamp":     fmt.Sprintf("%v", time.Now().UTC()),
			"services":      services,
			"general_state": statusOK,
		}
		failureDetected := false

		sdks := sdkrouter.FromRequest(req).GetAll()
		for _, s := range sdks {
			services["lbrynet"] = append(services["lbrynet"], ServerItem{Address: s.Address, Status: statusOK})
		}

		for _, ps := range PlayerServers {
			r, err := http.Get(ps)
			srv := ServerItem{Address: ps, Status: statusOK}
			if err != nil {
				srv.Error = fmt.Sprintf("%v", err)
				srv.Status = statusOffline
				respStatus = http.StatusServiceUnavailable
				failureDetected = true
			} else if r.StatusCode != http.StatusNotFound {
				srv.Status = statusNotReady
				srv.Error = fmt.Sprintf("http status %v", r.StatusCode)
				respStatus = http.StatusServiceUnavailable
				failureDetected = true
			}
			services["player"] = append(services["player"], srv)
		}
		if failureDetected {
			response["general_state"] = statusFailing
		}
		cachedResponse = &response
		lastUpdate = time.Now()
	}

	authResult := auth.FromRequest(req)
	if authResult.Authenticated() {
		response["user"] = map[string]interface{}{
			"user_id":      authResult.User().ID,
			"assigned_sdk": authResult.SDKAddress,
		}
	}

	responses.AddJSONContentType(w)
	w.WriteHeader(respStatus)
	respByte, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		logger.Log().Error(err)
	}
	w.Write(respByte)
}

func WhoAMI(w http.ResponseWriter, req *http.Request) {
	details := map[string]string{
		"ip":              fmt.Sprintf("%v", req.RemoteAddr),
		"X-Forwarded-For": req.Header.Get("X-Forwarded-For"),
		"X-Real-Ip":       req.Header.Get("X-Real-Ip"),
	}

	responses.AddJSONContentType(w)
	respByte, err := json.MarshalIndent(&details, "", "  ")
	if err != nil {
		logger.Log().Error(err)
	}
	w.Write(respByte)
}

package status

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lbryio/lbrytv/app/router"
	"github.com/lbryio/lbrytv/internal/monitor"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
)

var Logger = monitor.NewModuleLogger("status")

var PlayerServers = []string{
	"https://player1.lbry.tv",
	"https://player2.lbry.tv",
	"https://player3.lbry.tv",
	"https://player4.lbry.tv",
	"https://player5.lbry.tv",
}

var (
	cachedResponse *statusResponse = nil
	lastUpdate     time.Time
)

const (
	StatusOK            = "ok"
	StatusNotReady      = "not_ready"
	StatusOffline       = "offline"
	StatusFailing       = "failing"
	statusCacheValidity = 60 * time.Second
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
	var response *statusResponse

	if cachedResponse != nil && lastUpdate.After(time.Now().Add(statusCacheValidity)) {
		response = cachedResponse
	} else {
		services := map[string]ServerList{
			"lbrynet": ServerList{},
			"player":  ServerList{},
		}
		response = &statusResponse{
			"timestamp":     fmt.Sprintf("%v", time.Now().UTC()),
			"services":      services,
			"general_state": StatusOK,
		}
		failureDetected := false

		router := router.NewDefault()
		sdks := router.GetSDKServerList()
		for _, s := range sdks {
			c := ljsonrpc.NewClient(s.Address)
			c.SetRPCTimeout(30 * time.Second)
			status, err := c.Status()
			srv := ServerItem{Address: s.Address, Status: StatusOK}
			if err != nil {
				srv.Error = fmt.Sprintf("%v", err)
				srv.Status = StatusOffline
				respStatus = http.StatusServiceUnavailable
				failureDetected = true
			} else if !status.StartupStatus.Wallet {
				srv.Status = StatusNotReady
				respStatus = http.StatusServiceUnavailable
				failureDetected = true
			}
			services["lbrynet"] = append(services["lbrynet"], srv)
		}

		for _, ps := range PlayerServers {
			r, err := http.Get(ps)
			srv := ServerItem{Address: ps, Status: StatusOK}
			if err != nil {
				srv.Error = fmt.Sprintf("%v", err)
				srv.Status = StatusOffline
				respStatus = http.StatusServiceUnavailable
				failureDetected = true
			} else if r.StatusCode != http.StatusNotFound {
				srv.Status = StatusNotReady
				srv.Error = fmt.Sprintf("http status %v", r.StatusCode)
				respStatus = http.StatusServiceUnavailable
				failureDetected = true
			}
			services["player"] = append(services["player"], srv)
		}
		if failureDetected {
			(*response)["general_state"] = StatusFailing
		}
		cachedResponse = response
		lastUpdate = time.Now()
	}
	w.Header().Add("content-type", "application/json; charset=utf-8")
	w.WriteHeader(respStatus)
	respByte, _ := json.MarshalIndent(&response, "", "  ")
	w.Write(respByte)
}

func WhoAMI(w http.ResponseWriter, req *http.Request) {
	details := map[string]string{
		"ip":              fmt.Sprintf("%v", req.RemoteAddr),
		"X-Forwarded-For": req.Header.Get("X-Forwarded-For"),
		"X-Real-Ip":       req.Header.Get("X-Real-Ip"),
	}

	w.Header().Add("content-type", "application/json; charset=utf-8")
	respByte, _ := json.MarshalIndent(&details, "", "  ")
	w.Write(respByte)
}

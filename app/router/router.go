package router

import (
	"regexp"
	"sort"
	"strconv"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/monitor"
)

type SDKRouter struct {
	LbrynetServers map[string]string
	logger         monitor.ModuleLogger
	servers        []SDK
}

//SDK represents the Name and Address of an SDK server location
type SDK struct { //Remove this once we can store names in DB
	Name    string
	Address string
}

func SingleSDKSet(address string) map[string]string {
	return map[string]string{"default": address}
}

func New(sdkServers map[string]string) SDKRouter {
	return SDKRouter{
		LbrynetServers: sdkServers,
		logger:         monitor.NewModuleLogger("router"),
	}
}

func NewDefault() SDKRouter {
	return SDKRouter{LbrynetServers: map[string]string{
		"default": config.GetLbrynet(),
	},
		logger: monitor.NewModuleLogger("router"),
	}
}

func (r *SDKRouter) GetSDKServerList() []SDK {
	if len(r.servers) == 0 {
		r.sortServers()
	}
	return r.servers
}

func (r *SDKRouter) GetSDKServer(walletID string) string {
	sdk := r.balancedSDK()
	if walletID != "" {
		sdk = r.getSDKByWalletID(walletID)
	}

	if walletID != "" && sdk == "" {
		r.logger.Log().Info("walletID is set but there is no server associated with it.")
		sdk = r.defaultSDKServer() //"UNKOWN_SERVER"
	}
	r.logger.Log().Infof("From wallet id [%s] server [%s] is returned", walletID, sdk)
	return sdk
}

func (r *SDKRouter) getSDKByWalletID(walletID string) string {
	if len(r.servers) == 0 {
		r.sortServers()
	}
	digit := getLastDigit(walletID)
	server := int64(0)
	if len(r.servers) == 0 {
		r.logger.Log().Info("There are no servers listed. Something went terribly wrong.")
		return r.defaultSDKServer()
	} else {
		server = digit % int64(len(r.servers))
	}

	return r.servers[server].Address

}

func getLastDigit(walletID string) int64 {
	regex, err := regexp.Compile(`(\d+)`)
	if err != nil {
		return 0
	}
	useridStr := regex.FindAllString(walletID, 1)
	if len(useridStr) == 0 {
		return 0
	}
	userID, err := strconv.ParseInt(useridStr[0], 10, 64)
	if err != nil {
		return 0
	}

	return userID % 10
}

func (r *SDKRouter) sortServers() {
	var servers []SDK
	for name, address := range r.LbrynetServers {
		servers = append(servers, SDK{Name: name, Address: address})
	}
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})
	r.servers = servers
}

func (r *SDKRouter) balancedSDK() string {
	return r.defaultSDKServer()
}

func (r *SDKRouter) defaultSDKServer() string {
	server, _ := r.LbrynetServers["default"]
	return server
}

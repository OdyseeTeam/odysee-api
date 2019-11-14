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
	servers        []LbrynetServer
}

//LbrynetServer represents the Name and Address of an LbrynetServer server location
type LbrynetServer struct { //Remove this once we can store names in DB
	Name    string
	Address string
}

func SingleLbrynetServer(address string) map[string]string {
	return map[string]string{"default": address}
}

func New(lbrynetServers map[string]string) SDKRouter {

	sdkRouter := SDKRouter{
		LbrynetServers: lbrynetServers,
		logger:         monitor.NewModuleLogger("router"),
	}

	sdkRouter.sortServers()
	if _, ok := lbrynetServers["default"]; !ok {
		sdkRouter.logger.Log().Error(`There is no default lbrynet server defined with key "default"`)
	}
	return sdkRouter
}

func NewDefault() SDKRouter {
	return New(map[string]string{"default": config.Config.Viper.GetString("Lbrynet")})
}

func (r *SDKRouter) GetSDKServerList() []LbrynetServer {
	return r.servers
}

func (r *SDKRouter) GetSDKServer(walletID string) string {
	sdk := r.balancedSDK()
	if walletID != "" {
		sdk = r.getSDKByWalletID(walletID)
	}

	if walletID != "" && sdk == "" {
		r.logger.Log().Errorf("walletID [%s] is set but there is no server associated with it.", walletID)
		sdk = r.defaultSDKServer() //"UNKOWN_SERVER"
	}
	r.logger.Log().Debugf("From wallet id [%s] server [%s] is returned", walletID, sdk)
	return sdk
}

func (r *SDKRouter) getSDKByWalletID(walletID string) string {

	digit := getLastDigit(walletID)
	server := int64(0)
	if len(r.servers) == 0 {
		r.logger.Log().Error("There are no servers listed. Something went terribly wrong.")
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
	if len(r.LbrynetServers) == 0 {
		r.logger.Log().Error("Router created with NO servers!")
	}
	var servers []LbrynetServer
	for name, address := range r.LbrynetServers {
		servers = append(servers, LbrynetServer{Name: name, Address: address})
	}
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})
	r.servers = servers
}

func (r *SDKRouter) GetBalancedSDK() string {
	return r.balancedSDK()
}

func (r *SDKRouter) balancedSDK() string {
	return r.defaultSDKServer()
}

func (r *SDKRouter) defaultSDKServer() string {
	server, _ := r.LbrynetServers["default"]
	return server
}

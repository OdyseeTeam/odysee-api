package router

import (
	"database/sql"
	"errors"
	"regexp"
	"sort"
	"strconv"

	"github.com/volatiletech/sqlboiler/queries/qm"

	"github.com/lbryio/lbrytv/models"

	"github.com/lbryio/lbrytv/config"

	"github.com/lbryio/lbrytv/internal/monitor"
)

type SDKRouter struct {
	LbrynetServers map[string]string
	logger         monitor.ModuleLogger
	servers        []models.LbrynetServer
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

	sdkRouter.populateServers()
	if _, ok := lbrynetServers["default"]; !ok {
		sdkRouter.logger.Log().Fatal(`There is no default lbrynet server defined with key "default"`)
	}
	return sdkRouter
}

func NewDefault() SDKRouter {
	return New(map[string]string{"default": config.Config.Viper.GetString("Lbrynet")})
}

func (r *SDKRouter) GetSDKServerList() []models.LbrynetServer {
	r.populateServers()
	return r.servers
}

func (r *SDKRouter) GetSDKServer(walletID string) models.LbrynetServer {
	r.populateServers()
	sdk := r.balancedSDK()
	if walletID != "" {
		sdk = r.getSDKByWalletID(walletID)
	}

	if walletID != "" && sdk.Address == "" {
		r.logger.Log().Errorf("walletID [%s] is set but there is no server associated with it.", walletID)
		sdk = r.defaultSDKServer() //"UNKOWN_SERVER"
	}
	r.logger.Log().Tracef("From wallet id [%s] server [%s] is returned", walletID, sdk.Address)
	return sdk
}

func (r *SDKRouter) GetSDKServerAddress(walletID string) string {
	return r.GetSDKServer(walletID).Address
}

func (r *SDKRouter) getSDKByWalletID(walletID string) models.LbrynetServer {
	if len(r.servers) == 0 {
		r.logger.Log().Error("There are no servers listed. Something went terribly wrong.")
		return r.defaultSDKServer()
	}
	userID := getUserID(walletID)
	user, err := models.Users(qm.Load(models.UserRels.LbrynetServer), models.UserWhere.ID.EQ(userID)).OneG()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		r.logger.Log().Errorf("Error getting user %d from db: %s", userID, err.Error())
	}
	server := 0
	if user != nil {
		for i, s := range r.servers {
			if user.R.LbrynetServer != nil {
				if s.ID == user.R.LbrynetServer.ID {
					server = i
				}
			}
		}
	} else {
		server = userID % 10 % len(r.servers)
	}

	return r.servers[server]

}

func getUserID(walletID string) int {
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

	return int(userID)
}

func (r *SDKRouter) populateServers() {
	if len(r.LbrynetServers) == 0 {
		r.logger.Log().Fatal("Router created with NO servers!")
	}
	var servers []models.LbrynetServer
	if len(r.LbrynetServers) > 0 {
		for name, address := range r.LbrynetServers {
			servers = append(servers, models.LbrynetServer{Name: name, Address: address})
		}
	} else {
		serversDB, err := models.LbrynetServers().AllG()
		if err != nil {
			r.logger.Log().Error("Error retrieving lbrynet servers: ", err)
		}
		for _, s := range serversDB {
			if s != nil {
				servers = append(servers, *s)
			}
		}
	}

	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})
	r.servers = servers
}

func (r *SDKRouter) GetBalancedSDKAddress() string {
	return r.GetBalancedSDK().Address
}

func (r *SDKRouter) GetBalancedSDK() models.LbrynetServer {
	return r.balancedSDK()
}

func (r *SDKRouter) balancedSDK() models.LbrynetServer {
	return r.defaultSDKServer()
}

func (r *SDKRouter) defaultSDKServer() models.LbrynetServer {
	r.populateServers()
	var server models.LbrynetServer
	for _, s := range r.servers {
		if s.Name == "default" {
			server = s
		}
	}
	return server
}

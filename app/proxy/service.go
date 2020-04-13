package proxy

import (
	"net/http"
	"time"

	"github.com/lbryio/lbrytv/app/sdkrouter"

	"github.com/ybbus/jsonrpc"
)

const defaultRPCTimeout = 30 * time.Second

// Service generates Caller objects and keeps execution time metrics
// for all calls proxied through those objects.
type Service struct {
	SDKRouter  *sdkrouter.Router
	rpcTimeout time.Duration
}

// NewService is the entry point to proxy module.
// Normally only one instance of Service should be created per running server.
func NewService(router *sdkrouter.Router) *Service {
	return &Service{
		SDKRouter:  router,
		rpcTimeout: defaultRPCTimeout,
	}
}

func (ps *Service) SetRPCTimeout(timeout time.Duration) {
	ps.rpcTimeout = timeout
}

// NewCaller returns an instance of Caller ready to proxy requests.
// Note that `SetWalletID` needs to be called if an authenticated user is making this call.
func (ps *Service) NewCaller(walletID string) *Caller {
	endpoint := ps.SDKRouter.GetServer(sdkrouter.UserID(walletID)).Address
	return &Caller{
		endpoint: endpoint,
		walletID: walletID,
		client: jsonrpc.NewClientWithOpts(endpoint, &jsonrpc.RPCClientOpts{
			HTTPClient: &http.Client{Timeout: ps.rpcTimeout},
		}),
	}
}

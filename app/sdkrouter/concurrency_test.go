package sdkrouter

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/internal/test"
	"github.com/lbryio/lbrytv/models"

	"github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"
)

func TestRouterConcurrency(t *testing.T) {
	rpcServer, nextResp := test.MockJSONRPCServer(nil)
	defer rpcServer.Close()
	nextResp(`{"result": {"items": [], "page": 1, "page_size": 1, "total_pages": 10}}`) // mock WalletList response

	r := New(map[string]string{"srv": rpcServer.URL})
	servers := r.servers
	servers2 := []*models.LbrynetServer{
		{Name: "one", Address: rpcServer.URL},
		{Name: "two", Address: rpcServer.URL},
	}
	wg := sync.WaitGroup{}

	db := boil.GetDB()
	boil.SetDB(nil) // so we don't get errors about too many Postgres connections
	defer func() { boil.SetDB(db) }()

	logLvl := logrus.GetLevel()
	logrus.SetLevel(logrus.InfoLevel) // silence the jsonrpc debug messages
	defer func() { logrus.SetLevel(logLvl) }()

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			time.Sleep(time.Duration(rand.Intn(150)) * time.Millisecond)
			//t.Log("reads")
			switch rand.Intn(3) { // do reads in different orders
			case 0:
				r.RandomServer()
				r.GetAll()
				r.GetServer("yutwns.123.wallet")
			case 1:
				r.GetAll()
				r.GetServer("yutwns.123.wallet")
				r.RandomServer()
			case 2:
				r.GetServer("yutwns.123.wallet")
				r.RandomServer()
				r.GetAll()
			}
			wg.Done()
			//t.Log("reads done")
		}()
	}
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func(i int) {
			time.Sleep(time.Duration(rand.Intn(75)) * time.Millisecond)
			//t.Log("WRITE WRITE WRITE")
			if i%2 == 0 {
				r.setServers(servers)
			} else {
				r.setServers(servers2)
			}
			wg.Done()
		}(i)
	}
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
			//t.Log("update metrics")
			r.updateLoadAndMetrics()
			wg.Done()
		}()
	}

	wg.Wait()
}

package sdkrouter

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/models"
	"github.com/volatiletech/sqlboiler/boil"
)

func TestRouterConcurrency(t *testing.T) {
	r := New(config.GetLbrynetServers())
	servers := r.servers
	servers2 := []*models.LbrynetServer{
		{Name: "one", Address: "1.2.3.4"},
		{Name: "two", Address: "2.3.4.5"},
	}
	wg := sync.WaitGroup{}

	db := boil.GetDB()
	boil.SetDB(nil) // so we don't get errors about too many Postgres connections
	defer func() { boil.SetDB(db) }()

	for i := 0; i < 2000; i++ {
		wg.Add(1)
		go func() {
			time.Sleep(time.Duration(rand.Intn(150)) * time.Millisecond)
			//t.Log("reads")
			r.RandomServer()
			r.GetAll()
			r.GetServer("yutwns.123.wallet")
			wg.Done()
			//t.Log("reads done")
		}()
	}
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func(i int) {
			time.Sleep(time.Duration(rand.Intn(75)) * time.Millisecond)
			//t.Log("write")
			if i%2 == 0 {
				r.setServers(servers)
			} else {
				r.setServers(servers2)
			}
			wg.Done()
		}(i)
	}

	wg.Wait()
}

//func TestWatcherConcurrency(t *testing.T) {
//	r := New(config.GetLbrynetServers())
//	wg := sync.WaitGroup{}
//
//	db := boil.GetDB()
//	boil.SetDB(nil) // so we don't get errors about too many Postgres connections
//	defer func() { boil.SetDB(db) }()
//
//	for i := 0; i < 2000; i++ {
//		wg.Add(1)
//		go func() {
//			time.Sleep(time.Duration(rand.Intn(150)) * time.Millisecond)
//			r.updateLoadAndMetrics()
//			wg.Done()
//		}()
//	}
//
//	wg.Wait()
//}

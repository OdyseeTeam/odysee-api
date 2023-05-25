package asynquery

import (
	"testing"

	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/e2etest"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"

	"github.com/stretchr/testify/suite"
)

type asynquerySuite struct {
	suite.Suite

	manager    *CallManager
	userHelper *e2etest.UserTestHelper
}

func TestAsynquerySuite(t *testing.T) {
	suite.Run(t, new(asynquerySuite))
}

func (s *asynquerySuite) TestSuccessCallback() {

	// c := s.manager.NewCaller(s.userHelper.UserID())
	// r, err := c.A(context.Background(), jsonrpc.NewRequest(query.MethodWalletBalance))
	// s.Require().NoError(err)
	// s.Nil(r)

	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()
	// select {
	// case r := <-results:
	// 	s.NotEmpty(r.Response.Result.(map[string]any)["available"])
	// 	s.Equal(query.MethodWalletBalance, r.Query.Request.Method)
	// 	s.Equal(s.userHelper.UserID(), r.Query.UserID)
	// 	s.NotEmpty(r.Query.ID)
	// case <-ctx.Done():
	// 	s.T().Log("waiting too long")
	// 	s.T().FailNow()
	// }
}

func (s *asynquerySuite) TestErrorCallback() {
	// c := s.manager.NewCaller(s.userHelper.UserID())
	// r, err := c.Call(context.Background(), jsonrpc.NewRequest(query.MethodPublish))
	// s.Require().NoError(err)
	// s.Nil(r)

	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()
	// select {
	// case r := <-results:
	// 	s.Nil(r.Response.Result)
	// 	s.NotNil(r.Response.Error)
	// 	s.Equal(query.MethodPublish, r.Query.Request.Method)
	// 	s.Equal(s.userHelper.UserID(), r.Query.UserID)
	// 	s.NotEmpty(r.Query.ID)
	// case <-ctx.Done():
	// 	s.T().Log("waiting too long")
	// 	s.T().FailNow()
	// }
}

func (s *asynquerySuite) SetupSuite() {
	s.userHelper = &e2etest.UserTestHelper{}
	s.Require().NoError(s.userHelper.Setup(s.T()))

	ro, err := config.GetAsynqRedisOpts()
	s.Require().NoError(err)
	m, err := NewCallManager(ro, s.userHelper.DB, zapadapter.NewKV(nil))
	s.Require().NoError(err)
	s.manager = m
	go m.Start()
}

func (s *asynquerySuite) TearDownSuite() {
	config.RestoreOverridden()
	s.manager.Shutdown()
}

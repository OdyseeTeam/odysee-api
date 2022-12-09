package asynquery

import (
	"context"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/e2etest"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"

	"github.com/stretchr/testify/suite"
	"github.com/ybbus/jsonrpc"
)

type cleanupFunc func() error

type asynquerySuite struct {
	suite.Suite

	m          *CallManager
	userHelper *e2etest.UserTestHelper
}

func TestAsynquerySuite(t *testing.T) {
	suite.Run(t, new(asynquerySuite))
}

func (s *asynquerySuite) TestSuccessCallback() {
	results := make(chan AsyncQueryResult)
	s.m.SetResultChannel(query.MethodWalletBalance, results)

	c := s.m.NewCaller(s.userHelper.UserID())
	r, err := c.Call(context.Background(), jsonrpc.NewRequest(query.MethodWalletBalance))
	s.Require().NoError(err)
	s.Nil(r)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	select {
	case r := <-results:
		s.NotEmpty(r.Response.Result.(map[string]any)["available"])
		s.Equal(query.MethodWalletBalance, r.Query.Request.Method)
		s.Equal(s.userHelper.UserID(), r.Query.UserID)
		s.NotEmpty(r.Query.QueryID)
	case <-ctx.Done():
		s.T().Log("waiting too long")
		s.T().FailNow()
	}
}

func (s *asynquerySuite) TestErrorCallback() {
	results := make(chan AsyncQueryResult)
	s.m.SetResultChannel(query.MethodPublish, results)

	c := s.m.NewCaller(s.userHelper.UserID())
	r, err := c.Call(context.Background(), jsonrpc.NewRequest(query.MethodPublish))
	s.Require().NoError(err)
	s.Nil(r)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	select {
	case r := <-results:
		s.Nil(r.Response.Result)
		s.NotNil(r.Response.Error)
		s.Equal(query.MethodPublish, r.Query.Request.Method)
		s.Equal(s.userHelper.UserID(), r.Query.UserID)
		s.NotEmpty(r.Query.QueryID)
	case <-ctx.Done():
		s.T().Log("waiting too long")
		s.T().FailNow()
	}
}

func (s *asynquerySuite) SetupSuite() {
	s.userHelper = &e2etest.UserTestHelper{}
	s.Require().NoError(s.userHelper.Setup(s.T()))

	ro, err := config.GetAsynqRedisOpts()
	s.Require().NoError(err)
	m, err := NewCallManager(ro, zapadapter.NewKV(nil))
	s.Require().NoError(err)
	s.m = m
	go m.Start(s.userHelper.DB)
}

func (s *asynquerySuite) TearDownSuite() {
	config.RestoreOverridden()
	s.userHelper.Cleanup()
	s.m.Shutdown()
}

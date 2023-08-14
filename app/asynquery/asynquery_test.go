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

func (s *asynquerySuite) SetupSuite() {
	s.userHelper = &e2etest.UserTestHelper{}
	s.Require().NoError(s.userHelper.Setup(s.T()))

	ro, err := config.GetRedisBusOpts()
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

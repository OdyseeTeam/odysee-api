package users

import (
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/lbryio/lbryweb.go/db"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"
)

type StoreSuite struct {
	suite.Suite
	store *dbStore
	db    *gorm.DB
}

func (s *StoreSuite) SetupSuite() {
	s.db = db.Conn
	s.store = &dbStore{db: db.Conn}
	err := s.store.AutoMigrate()
	if err != nil {
		s.T().Fatal(err)
	}
}

func (s *StoreSuite) SetupTest() {
	db := s.db.Exec("DELETE FROM users;")
	if db.Error != nil {
		s.T().Fatal(db.Error)
	}
}

func (s *StoreSuite) TearDownSuite() {
	s.db.Exec("DELETE FROM users;")
	if s.db.Error != nil {
		s.T().Fatal(s.db.Error)
	}
}

func TestStoreSuite(t *testing.T) {
	s := new(StoreSuite)
	suite.Run(t, s)
}

func (s *StoreSuite) TestCreateRecord() {
	err := s.store.CreateRecord("acCID", "tOkEn")
	if err != nil {
		s.T().Fatal(err)
	}

	user, err := s.store.GetRecordByToken("tOkEn")
	if err != nil {
		s.T().Fatal(err)
	}
	assert.Equal(s.T(), "acCID", user.SDKAccountID)

	// Duplicate record should not go through
	err = s.store.CreateRecord("acCID", "tOkEn")
	if err == nil {
		s.T().Fatal("duplicate record created")
	}
}

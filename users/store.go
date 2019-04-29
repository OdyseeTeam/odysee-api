package users

import (
	"fmt"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres" // Dialect import
)

type Store interface {
	CreateRecord(accountID, token string) error
	GetRecordByToken() (User, error)
	AutoMigrate()
}

type dbStore struct {
	db *gorm.DB
}

// User is a thin model containing basic data about lbryweb user.
// The majority of user data is stored in internal-apis, referenced by AuthToken
type User struct {
	gorm.Model
	// CreatedAt    time.Time `json:"created_at"`
	AuthToken    string `json:"auth_token" gorm:"unique;not null" gorm:"index:auth_token"`
	SDKAccountID string `json:"sdk_account_id" gorm:"unique;not null"`
}

var store Store

func InitStore(s Store) {
	store = s
}

// AutoMigrate migrates user table
func (s *dbStore) AutoMigrate() error {
	db := s.db.AutoMigrate(&User{})
	if db.Error != nil {
		return db.Error
	}
	return nil
}

// GetRecordByToken retrieves user record by token
func (s *dbStore) GetRecordByToken(token string) (u User, err error) {
	db := s.db.First(&u, "auth_token = ?", token)
	return u, db.Error
}

// CreateRecord saves user record to the database
func (s *dbStore) CreateRecord(accountID, token string) error {
	_, err := s.GetRecordByToken(token)
	if err == nil {
		return fmt.Errorf("user %v already exists", token)
	}
	return s.db.Create(&User{AuthToken: token, SDKAccountID: accountID}).Error
}

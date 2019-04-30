package users

import (
	"testing"

	"github.com/lbryio/lbrytv/db"
	"gotest.tools/assert"
)

func TestCreateRecord(t *testing.T) {
	t.SkipNow()
	var store dbStore

	store = dbStore{db: db.Conn}
	err := store.AutoMigrate()

	db := db.Conn.Exec("DELETE FROM users;")
	if db.Error != nil {
		t.Fatal(err)
	}

	if err != nil {
		t.Fatal(err)
	}

	err = store.CreateRecord("acCID", "tOkEn")
	if err != nil {
		t.Fatal(err)
	}

	user, err := store.GetRecordByToken("tOkEn")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "acCID", user.SDKAccountID)

	// Duplicate record should not go through
	err = store.CreateRecord("acCID", "tOkEn")
	if err == nil {
		t.Fatal("duplicate record created")
	}
}

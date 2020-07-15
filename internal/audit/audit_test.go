package audit

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/lbryio/lbry.go/v2/extras/null"
	"github.com/lbryio/lbrytv/app/query"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/internal/test"
	"github.com/lbryio/lbrytv/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

func TestMain(m *testing.M) {
	dbConfig := config.GetDatabase()
	params := storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	}
	dbConn, connCleanup := storage.CreateTestConn(params)
	dbConn.SetDefaultConnection()

	code := m.Run()

	connCleanup()
	os.Exit(code)
}

func TestLogQuery(t *testing.T) {
	dummyUserID := 1234
	jReq := jsonrpc.NewRequest(
		query.MethodWalletSend,
		map[string]interface{}{"addresses": []string{"dgjkldfjgldkfjgkldfjg"}, "amount": "6.49999000"})
	q := test.ReqToStr(t, jReq)
	ql := LogQuery(dummyUserID, "8.8.8.8", query.MethodWalletSend, []byte(q))
	ql, err := models.QueryLogs(models.QueryLogWhere.ID.EQ(ql.ID)).OneG()
	require.NoError(t, err)
	assert.Equal(t, "8.8.8.8", ql.RemoteIP)
	assert.EqualValues(t, null.IntFrom(dummyUserID), ql.UserID)

	loggedReq := &jsonrpc.RPCRequest{}
	expReq := &jsonrpc.RPCRequest{}

	err = ql.Body.Unmarshal(&loggedReq)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(q), expReq)
	require.NoError(t, err)

	assert.Equal(t, expReq, loggedReq)
}

func TestLogQueryNoUserNoRemoteIP(t *testing.T) {
	var dummyUserID int
	jReq := jsonrpc.NewRequest(
		query.MethodWalletSend,
		map[string]interface{}{"addresses": []string{"dgjkldfjgldkfjgkldfjg"}, "amount": "6.49999000"})
	q := test.ReqToStr(t, jReq)
	ql := LogQuery(dummyUserID, "", query.MethodWalletSend, []byte(q))
	ql, err := models.QueryLogs(models.QueryLogWhere.ID.EQ(ql.ID)).OneG()
	require.NoError(t, err)
	assert.Equal(t, "", ql.RemoteIP)
	assert.EqualValues(t, null.IntFrom(dummyUserID), ql.UserID)

	loggedReq := &jsonrpc.RPCRequest{}
	expReq := &jsonrpc.RPCRequest{}

	err = ql.Body.Unmarshal(&loggedReq)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(q), expReq)
	require.NoError(t, err)

	assert.Equal(t, expReq, loggedReq)
}

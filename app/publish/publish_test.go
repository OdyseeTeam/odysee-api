package publish

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/app/proxy"
	"github.com/lbryio/lbrytv/app/users"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/lbrynet"
	"github.com/lbryio/lbrytv/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// func init() {
// 	flag.StringVar(&foo, "foo", "", "the foo bar bang")
// 	flag.Parse()
// }

func prettyPrint(i interface{}) {
	s, _ := json.MarshalIndent(i, "", "\t")
	fmt.Println(string(s))
}

func copyToDocker(t *testing.T, fileName string) {
	cmd := fmt.Sprintf(`docker cp %v lbrytv_lbrynet_1:/storage`, fileName)
	if _, err := exec.Command("bash", "-c", cmd).Output(); err != nil {
		t.Skipf("skipping TestLbrynetPublisher (cannot copy %v to docker container: %v)", fileName, err)
	}
}

func launchDummyAPIServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"success": true,
			"error": null,
			"data": {
			  "id": 751365,
			  "language": "en",
			  "given_name": null,
			  "family_name": null,
			  "created_at": "2019-01-17T12:13:06Z",
			  "updated_at": "2019-05-02T13:57:59Z",
			  "invited_by_id": null,
			  "invited_at": null,
			  "invites_remaining": 0,
			  "invite_reward_claimed": false,
			  "is_email_enabled": true,
			  "manual_approval_user_id": 837139,
			  "reward_status_change_trigger": "manual",
			  "primary_email": "andrey@lbry.com",
			  "has_verified_email": true,
			  "is_identity_verified": false,
			  "is_reward_approved": true,
			  "groups": []
			}
		}`))
	}))
}

func TestLbrynetPublisher(t *testing.T) {
	dummyUserID := 751365
	authToken := "zzz"

	dbConfig := config.GetDatabase()
	params := storage.ConnParams{
		Connection: dbConfig.Connection,
		DBName:     dbConfig.DBName,
		Options:    dbConfig.Options,
	}
	c, connCleanup := storage.CreateTestConn(params)
	c.SetDefaultConnection()
	defer connCleanup()
	defer lbrynet.RemoveAccount(dummyUserID)

	ts := launchDummyAPIServer()
	defer ts.Close()
	config.Override("InternalAPIHost", ts.URL)
	defer config.RestoreOverridden()

	p := &LbrynetPublisher{proxy.NewService(config.GetLbrynet())}

	userSvc := users.NewUserService()
	u, err := userSvc.Retrieve(authToken)
	require.Nil(t, err)
	// Required for the account to settle down in the SDK
	time.Sleep(500 * time.Millisecond)

	data := []byte("test file")
	f, err := ioutil.TempFile(os.TempDir(), "*")
	require.Nil(t, err)
	_, err = f.Write(data)
	require.Nil(t, err)
	err = f.Close()
	require.Nil(t, err)
	defer os.Remove(f.Name())

	copyToDocker(t, f.Name())

	query := []byte(`{
		"jsonrpc": "2.0",
		"method": "stream_create",
		"params": {
			"name": "test",
			"title": "test",
			"description": "test description",
			"bid": "0.000001",
			"languages": [
				"en"
			],
			"tags": [],
			"thumbnail_url": "http://smallmedia.com/thumbnail.jpg",
			"license": "None",
			"release_time": 1567580184,
			"file_path": "__POST_FILE__"
		},
		"id": 1567580184168
	}`)

	rawResp := p.Publish(path.Join("/storage", path.Base(f.Name())), u.SDKAccountID, query)

	// This is all we can check for now without running on testnet or crediting some funds to the test account
	assert.Regexp(t, "Not enough funds to cover this transaction", string(rawResp))
}

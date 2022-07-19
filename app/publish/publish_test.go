package publish

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"
	"github.com/Pallinder/go-randomdata"
	"github.com/shopspring/decimal"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

type WalletKeys struct{ PrivateKey, PublicKey string }

const (
	envPublicKey  = "REAL_WALLET_PUBLIC_KEY"
	envPrivateKey = "REAL_WALLET_PRIVATE_KEY"
)

func copyToContainer(t *testing.T, srcPath, dstPath string) error {
	t.Helper()
	// cmd := fmt.Sprintf(`docker cp %s %s`, srcPath, dstPath)
	os.Setenv("PATH", os.Getenv("PATH")+":/usr/local/bin")
	if out, err := exec.Command("docker", "cp", srcPath, dstPath).CombinedOutput(); err != nil {
		fmt.Println(os.Getenv("PATH"))
		// if _, err := exec.Command("docker", "cp", srcPath, dstPath).Output(); err != nil {
		return fmt.Errorf("cannot copy %s to %s: %w (%s)", srcPath, dstPath, err, string(out))
	}
	return nil
}

func createRealWallet(t *testing.T, keys WalletKeys, userID int) {
	t.Helper()
	absPath, _ := filepath.Abs("./testdata/wallet.template")
	wt, err := template.New("wallet.template").ParseFiles(absPath)
	require.NoError(t, err)
	wf, err := os.CreateTemp("", fmt.Sprintf("wallet.%v.*", userID))
	// defer os.RemoveAll(wf.Name())
	require.NoError(t, err)
	err = wt.Execute(wf, keys)
	require.NoError(t, err)
	err = wf.Close()
	require.NoError(t, err)
	err = copyToContainer(t, wf.Name(), fmt.Sprintf("lbrynet:/storage/lbryum/wallets/lbrytv-id.%v.wallet", userID))
	require.NoError(t, err)
}

func Test_createRealWallet(t *testing.T) {
	userID := randomdata.Number(10000, 90000)
	createRealWallet(t, WalletKeys{PrivateKey: os.Getenv(envPrivateKey), PublicKey: os.Getenv(envPublicKey)}, userID)

	c := query.NewCaller("http://localhost:5279", userID)
	res, err := c.Call(jsonrpc.NewRequest("account_balance"))
	require.NoError(t, err)
	require.Nil(t, res.Error)

	var bal ljsonrpc.AccountBalanceResponse
	err = ljsonrpc.Decode(res.Result, &bal)
	require.NoError(t, err)
	fmt.Printf("%+v", bal)
	assert.GreaterOrEqual(t, bal.Available.Cmp(decimal.NewFromInt(1)), 0)
}

func TestLbrynetPublisher(t *testing.T) {
	db, dbCleanup, err := migrator.CreateTestDB(migrator.DBConfigFromApp(config.GetDatabase()), storage.MigrationsFS)
	if err != nil {
		panic(err)
	}
	storage.SetDB(db)
	dbCleanup()

	data := []byte("test file")
	f, err := ioutil.TempFile(os.TempDir(), "*")
	require.NoError(t, err)
	_, err = f.Write(data)
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)

	err = copyToContainer(t, f.Name(), "lbrynet:/storage")
	if err != nil {
		t.Skipf("skipping (%s)", err)
	}

	req := test.StrToReq(t, `{
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

	userID := 751365
	server := test.RandServerAddress(t)
	err = wallet.Create(server, userID)
	require.NoError(t, err)

	res, err := getCaller(server, path.Join("/storage", path.Base(f.Name())), userID, nil).Call(req)
	require.NoError(t, err)

	// This is all we can check for now without running on testnet or crediting some funds to the test account
	assert.Regexp(t, "Not enough funds to cover this transaction", test.ResToStr(t, res))
}

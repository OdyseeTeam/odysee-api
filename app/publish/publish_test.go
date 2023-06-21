package publish

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/storage"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/odysee-api/pkg/migrator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type WalletKeys struct{ PrivateKey, PublicKey string }

const (
	envPublicKey  = "REAL_WALLET_PUBLIC_KEY"
	envPrivateKey = "REAL_WALLET_PRIVATE_KEY"
)

func copyToContainer(t *testing.T, srcPath, dstPath string) error {
	t.Helper()
	os.Setenv("PATH", os.Getenv("PATH")+":/usr/local/bin")
	if out, err := exec.Command("docker", "cp", srcPath, dstPath).CombinedOutput(); err != nil {
		fmt.Println(os.Getenv("PATH"))
		return fmt.Errorf("cannot copy %s to %s: %w (%s)", srcPath, dstPath, err, string(out))
	}
	return nil
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

	res, err := getCaller(server, path.Join("/storage", path.Base(f.Name())), userID, nil).Call(
		context.Background(), req)
	require.NoError(t, err)

	// This is all we can check for now without running on testnet or crediting some funds to the test account
	assert.Regexp(t, "Not enough funds to cover this transaction", test.ResToStr(t, res))
}

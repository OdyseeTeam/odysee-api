package paid

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"
	mrand "math/rand"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testStreamID = "bea4d30a1868a00e98297cfe8cdefc1be6c141b54bea3b7c95b34a66786c22ab4e9f35ae19aa453b3630e76afbd24fe2"
var testTxID = "e5a6a9ef4433b8868cebb770d1d2244eed410eb7503a9ffa82dce40aa62bbae7"

func generateKeyFile() (string, error) {
	keyFilename := fmt.Sprintf("test_private_key_%v.pem", time.Now().Unix())
	cmd := fmt.Sprintf("openssl genrsa -out %s 2048", keyFilename)
	if _, err := exec.Command("bash", "-c", cmd).Output(); err != nil {
		return "", fmt.Errorf("command %v failed: %v", cmd, err)
	}
	return keyFilename, nil
}

func TestMain(m *testing.M) {
	keyFile, err := generateKeyFile()
	if err != nil {
		panic(err)
	}

	rawKey, err := ioutil.ReadFile(keyFile)
	if err != nil {
		panic(err)
	}

	err = InitPrivateKey(rawKey)
	if err != nil {
		panic(err)
	}

	pubKM = &pubKeyManager{}
	err = pubKM.loadFromBytes(km.PublicKeyBytes())
	if err != nil {
		panic(err)
	}

	code := m.Run()
	os.Remove(keyFile) // defers do not run after os.Exit()

	os.Exit(code)
}

func TestCreateToken(t *testing.T) {
	size := mrand.Uint64()
	token, err := CreateToken(testStreamID, testTxID, size, ExpTenSecPer100MB)
	require.NoError(t, err)

	streamToken, err := km.PublicKeyManager().ValidateToken(token)
	require.NoError(t, err)
	assert.Equal(t, streamToken.StreamID, testStreamID)
	assert.Equal(t, streamToken.TxID, testTxID)
}

func TestPublicKeyBytes(t *testing.T) {
	b, r := pem.Decode(km.PublicKeyBytes())
	assert.NotNil(t, b)
	assert.Empty(t, r)
}

func BenchmarkCreateToken(b *testing.B) {
	logger.Disable()
	for i := 0; i < b.N; i++ {
		if _, err := CreateToken(testStreamID, testTxID, 100_000_000, ExpTenSecPer100MB); err != nil {
			b.Fatal(err)
		}
	}
}

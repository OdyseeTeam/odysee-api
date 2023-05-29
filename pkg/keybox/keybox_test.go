package keybox

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testPrivKey = "LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSUZZYWxQZkhySzNSQ1F2YmhRQ1h6cDZiWG9uODZWOGI5L3B0bjB3QTZxNkxvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFZjhyN3RlQWJwUlVldXZhVWRsNDQzVS9VZkpYZURDd051QkRrbmp5ZnRZaXZ2Tnl6cGt6ZgpYdDl3RE9rc1VZSmEzNVhvSndabjNHMmw2L2EvdVUvWmh3PT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo="
var testPubKey = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEsDn7Awhhaw5iZ0Q1GVpzYuZavxH5b/AJS2b3FPFF2/NcN+MMll9lzdtHVo1RGsskGqDy0vII8GK6xxSJl4n1Ig=="

func TestKeyfob(t *testing.T) {
	_, err := KeyfobFromString(testPrivKey)
	require.NoError(t, err)
}

func TestNewValidator(t *testing.T) {
	_, err := NewValidator("not a key")
	require.ErrorContains(t, err, "not an ECDSA public key")

	validator, err := ValidatorFromPublicKeyString(testPubKey)
	require.NoError(t, err)

	assert.NotNil(t, validator.publicKey)
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(validator.publicKey)
	require.NoError(t, err)
	assert.Equal(t, base64.StdEncoding.EncodeToString(publicKeyBytes), testPubKey)
}

func TestKeyfobFromStringGenerateToken(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	bkey, err := x509.MarshalECPrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "ECDSA PRIVATE KEY",
		Bytes: bkey,
	})

	km, err := KeyfobFromString(base64.StdEncoding.EncodeToString(privateKeyPEM))
	require.NoError(t, err)

	upid := randomdata.RandStringRunes(32)
	token, err := km.GenerateToken(123, time.Now().Add(24*time.Second), "upload_id", upid)
	require.NoError(t, err)

	pt, err := km.Validator().ParseToken(token)
	require.NoError(t, err)
	assert.Equal(t, upid, pt.PrivateClaims()["upload_id"])
}

func TestPublicKeyHandler(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	kf, err := KeyfobFromString(testPrivKey)
	require.NoError(err)

	ts := httptest.NewServer(http.HandlerFunc(PublicKeyHandler(kf.PublicKey())))
	defer ts.Close()

	pubKey, err := PublicKeyFromURL(ts.URL)
	require.NoError(err)

	assert.Equal(pubKey, kf.PublicKey(), "retrieved public key does not match parsed public key")
}

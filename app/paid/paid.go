package paid

// Key files should be generated as follows:
// $ openssl genrsa -out privateKey.pem 2048

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"

	"github.com/lbryio/lbrytv/internal/monitor"

	"github.com/dgrijalva/jwt-go"
)

var logger = monitor.NewModuleLogger("paid")

// Expfunc is a function type intended for CreateToken.
// Should take stream size in bytes and return validity as Unix time
type Expfunc func(uint64) int64

// StreamToken contains stream hash and transaction id of a stream that has been purchased
type StreamToken struct {
	SDHash string `json:"sd"`
	TxID   string `json:"txid"`
	jwt.StandardClaims
}

type keyManager struct {
	privKey         *rsa.PrivateKey
	pubKeyMarshaled []byte
}

var km *keyManager

// InitPrivateKey loads a private key from `[]bytes` for later token signing and derived pubkey distribution.
func InitPrivateKey(rawKey []byte) error {
	km = &keyManager{}
	err := km.loadFromBytes(rawKey)
	if err != nil {
		return err
	}
	return nil
}

// CreateToken takes stream hash, purchase transaction id and stream size to generate a JWS.
// In addition it accepts expiry function that takes streamSize and returns token expiry date as Unix time
func CreateToken(sdHash string, txid string, streamSize uint64, expfunc Expfunc) (string, error) {
	return km.createToken(sdHash, txid, streamSize, expfunc)
}

// GeneratePrivateKey generates an in-memory private key
func GeneratePrivateKey() error {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	k := &keyManager{privKey: privKey}
	k.pubKeyMarshaled, err = k.marshalPublicKey()
	if err != nil {
		return err
	}
	logger.Log().Infof("generated an in-memory private key")

	km = k
	return nil
}

func (k *keyManager) createToken(sdHash string, txid string, streamSize uint64, expfunc Expfunc) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, &StreamToken{
		sdHash,
		txid,
		jwt.StandardClaims{
			ExpiresAt: expfunc(streamSize),
			IssuedAt:  time.Now().UTC().Unix(),
		},
	})
	logger.Log().Debugf("created a token %v / %v", token.Header, token.Claims)
	return token.SignedString(k.privKey)
}

func (k *keyManager) loadFromBytes(b []byte) error {
	block, _ := pem.Decode(b)
	if block == nil {
		return errors.New("no PEM blob found")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	k.privKey = key
	logger.Log().Infof("loaded a private RSA key (%v bytes)", key.Size())

	k.pubKeyMarshaled, err = k.marshalPublicKey()
	if err != nil {
		return err
	}

	return nil
}

func (k *keyManager) PublicKeyBytes() []byte {
	return k.pubKeyMarshaled
}

func (k *keyManager) PublicKeyManager() *pubKeyManager {
	return &pubKeyManager{key: &k.privKey.PublicKey}
}

func (k *keyManager) marshalPublicKey() ([]byte, error) {
	pubKey, err := x509.MarshalPKIXPublicKey(&k.privKey.PublicKey)
	if err != nil {
		return nil, err
	}

	pubBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: pubKey,
	})

	return pubBytes, nil
}

// ExpTenSecPer100MB returns expiry date as calculated by 10 seconds times the stream size in MB
func ExpTenSecPer100MB(streamSize uint64) int64 {
	return time.Now().UTC().Add(time.Duration(streamSize/1024^2*10) * time.Second).Unix()
}

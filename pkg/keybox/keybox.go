package keybox

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type Keyfob struct {
	issuer     string
	publicKey  crypto.PublicKey
	privateKey crypto.PrivateKey
}

type Validator struct {
	publicKey crypto.PublicKey
}

// NewKeyfob creates a new Keyfob from an existing private key.
func NewKeyfob(privateKey crypto.PrivateKey) (*Keyfob, error) {
	if privateKey == nil {
		return nil, errors.New("empty private key supplied")
	}
	edpk, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an ecdsa private key")
	}
	kf := &Keyfob{
		privateKey: edpk,
		publicKey:  edpk.Public(),
	}
	return kf, nil
}

// GenerateKeyfob generates a new Keyfob containing a public and a private key.
func GenerateKeyfob() (*Keyfob, error) {
	pvk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("unable to generate private key: %e", err)
	}
	kf, err := NewKeyfob(pvk)
	if err != nil {
		return nil, err
	}
	return kf, nil
}

func NewValidator(publicKey crypto.PublicKey) (*Validator, error) {
	k, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("not an ECDSA public key")
	}
	v := &Validator{publicKey: k}
	return v, nil
}

// KeyfobFromString creates a Keyfob from an existing private key supplied as a base64 string.
func KeyfobFromString(privateKey string) (*Keyfob, error) {
	pvk, err := privateKeyFromString(privateKey)
	if err != nil {
		return nil, fmt.Errorf("unable to load private key from string: %e", err)
	}
	kf, err := NewKeyfob(pvk)
	if err != nil {
		return nil, fmt.Errorf("unable to create keyfob: %e", err)
	}
	return kf, nil
}

func ValidatorFromPublicKeyString(publicKey string) (*Validator, error) {
	v := &Validator{}
	var err error
	v.publicKey, err = publicKeyFromString(publicKey)
	if err != nil {
		return nil, fmt.Errorf("unable to load public key: %e", err)
	}
	return v, nil
}

func PublicKeyFromURL(keyURL string) (crypto.PublicKey, error) {
	r, err := http.Get(keyURL)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve public key: %w", err)
	}
	defer r.Body.Close()

	pemData, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read public key: %w", err)
	}

	block, _ := pem.Decode(pemData)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("invalid PEM-encoded public key")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse public key: %w", err)
	}

	return pubKey, nil
}

// GenerateToken generates a JWT token for a given user signed with the private key of the Keyfob.
func (kf Keyfob) GenerateToken(userID int32, expiry time.Time, fields ...string) (string, error) {
	b := jwt.NewBuilder().Issuer("api.odysee.com").Subject(strconv.Itoa(int(userID))).Expiration(expiry)
	for i := 0; i < len(fields); i += 2 {
		b = b.Claim(fields[i], fields[i+1])
	}
	t, err := b.Build()
	if err != nil {
		return "", fmt.Errorf("unable to build token: %w", err)
	}

	bt, err := jwt.Sign(t, jwt.WithKey(jwa.ES256, kf.privateKey))
	if err != nil {
		return "", fmt.Errorf("unable to sign token: %w", err)
	}

	return string(bt), nil
}

func (kf Keyfob) PublicKey() crypto.PublicKey {
	return kf.publicKey
}

func (kf Keyfob) PrivateKey() crypto.PrivateKey {
	return kf.privateKey
}

// NewValidator creates a new Validator from the public key.
func (kf Keyfob) Validator() *Validator {
	return &Validator{publicKey: kf.PublicKey()}
}

// ParseToken validates and parses a JWT token using the public key of the Validator structure,
// and returns the private claims as a map[string]interface{}.
func (v Validator) ParseToken(token string) (jwt.Token, error) {
	t, err := jwt.Parse([]byte(token), jwt.WithKey(jwa.ES256, v.publicKey))
	if t == nil {
		return nil, fmt.Errorf("unable to parse token: %w", err)
	}

	return t, err
}

// privateKeyFromString decodes a private key string encoded in Base64 and returns an ecdsa.PrivateKey.
func privateKeyFromString(key string) (any, error) {
	privateKeyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}

	privateKeyBlock, _ := pem.Decode(privateKeyBytes)
	privateKey, err := x509.ParseECPrivateKey(privateKeyBlock.Bytes)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

// publicKeyFromString decodes a public key string encoded in Base64 and returns an ecdsa.PublicKey.
func publicKeyFromString(key string) (any, error) {
	publicKeyBytes, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}

	var publicKey interface{}
	if publicKey, err = x509.ParsePKIXPublicKey(publicKeyBytes); err != nil {
		return nil, err
	}

	ecdsaPublicKey, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not an ECDSA key")
	}

	return ecdsaPublicKey, nil
}

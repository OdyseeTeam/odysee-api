package paid

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanPlayStream(t *testing.T) {
	noError := func(t *testing.T, err error) { assert.NoError(t, err) }
	type tokenMaker func() (string, error)
	type errChecker func(*testing.T, error)

	tests := []struct {
		name       string
		makeToken  tokenMaker
		want       bool
		checkError errChecker
	}{
		{
			name: "valid",
			makeToken: func() (string, error) {
				return CreateToken(testStreamID, testTxID, 120_000_000, ExpTenSecPer100MB)
			},
			want:       true,
			checkError: noError,
		},
		{
			name: "expired",
			makeToken: func() (string, error) {
				expFunc := func(uint64) int64 { return 1 } //  Returns the 1st second of Unix epoch
				return CreateToken(testStreamID, testTxID, 120_000_000, expFunc)
			},
			want:       false,
			checkError: func(t *testing.T, err error) { assert.Regexp(t, "token is expired by \\d+h\\d+m\\d+s", err) },
		},
		{
			name: "missigned",
			makeToken: func() (string, error) {
				otherPkey, _ := rsa.GenerateKey(rand.Reader, 2048)
				otherKM := &keyManager{privKey: otherPkey}
				return otherKM.createToken(testStreamID, testTxID, 120_000_000, ExpTenSecPer100MB)
			},
			want:       false,
			checkError: func(t *testing.T, err error) { assert.EqualError(t, err, "crypto/rsa: verification error") },
		},
		{
			name: "wrong_stream",
			makeToken: func() (string, error) {
				return CreateToken("wrOngHaSh", testTxID, 120_000_000, ExpTenSecPer100MB)
			},
			want: false,
			checkError: func(t *testing.T, err error) {
				assert.EqualError(t, err, "stream mismatch: requested bea4d30a1868a00e98297cfe8cdefc1be6c141b54bea3b7c95b34a66786c22ab4e9f35ae19aa453b3630e76afbd24fe2, token valid for wrOngHaSh")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := tt.makeToken()
			require.NoError(t, err)

			got, err := CanPlayStream(testStreamID, token)
			tt.checkError(t, err)
			if got != tt.want {
				t.Errorf("CanPlayStream() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkParseToken(b *testing.B) {
	token, err := CreateToken(testStreamID, testTxID, 100_000_000, ExpTenSecPer100MB)
	require.NoError(b, err)

	for i := 0; i < b.N; i++ {
		if _, err := CanPlayStream(testStreamID, token); err != nil {
			b.Fatal(err)
		}
	}
}

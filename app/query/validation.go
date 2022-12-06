package query

// this should really be in lbry.go
import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"math/big"

	"github.com/OdyseeTeam/odysee-api/internal/errors"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"
	"github.com/lbryio/lbry.go/v2/schema/keys"

	"github.com/btcsuite/btcd/btcec"
)

func ValidateSignatureFromClaim(channel *ljsonrpc.Claim, signature, signingTS, data string) error {
	if channel == nil {
		return errors.Err("no channel to validate")
	}
	if channel.SigningChannel.Value.GetChannel() == nil {
		return errors.Err("no channel for public key")
	}
	pubKey, err := keys.GetPublicKeyFromBytes(channel.SigningChannel.Value.GetChannel().GetPublicKey())
	if err != nil {
		return errors.Err(err)
	}

	return validateSignature(channel.SigningChannel.ClaimID, signature, signingTS, data, pubKey)

}

func validateSignature(channelClaimID, signature, signingTS, data string, publicKey *btcec.PublicKey) error {
	injest := sha256.Sum256(
		createDigest(
			[]byte(signingTS),
			unhelixifyAndReverse(channelClaimID),
			[]byte(data),
		))
	sig, err := hex.DecodeString(signature)
	if err != nil {
		return errors.Err(err)
	}
	signatureBytes := [64]byte{}
	copy(signatureBytes[:], sig)
	sigValid := isSignatureValid(signatureBytes, publicKey, injest[:])
	if !sigValid {
		return errors.Err("could not validate the signature")
	}
	return nil
}

func isSignatureValid(signature [64]byte, publicKey *btcec.PublicKey, injest []byte) bool {
	R := &big.Int{}
	S := &big.Int{}
	R.SetBytes(signature[:32])
	S.SetBytes(signature[32:])
	return ecdsa.Verify(publicKey.ToECDSA(), injest, R, S)
}

func createDigest(pieces ...[]byte) []byte {
	var digest []byte
	for _, p := range pieces {
		digest = append(digest, p...)
	}
	return digest
}

// rev reverses a byte slice. useful for switching endian-ness
func reverseBytes(b []byte) []byte {
	r := make([]byte, len(b))
	for left, right := 0, len(b)-1; left < right; left, right = left+1, right-1 {
		r[left], r[right] = b[right], b[left]
	}
	return r
}

func unhelixifyAndReverse(claimID string) []byte {
	b, err := hex.DecodeString(claimID)
	if err != nil {
		return nil
	}
	return reverseBytes(b)
}

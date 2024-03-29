package query

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"path"
	"strings"
)

func signStreamURL77(host, filePath, secureToken string, expiryTimestamp int64) (string, error) {
	strippedPath := path.Dir(filePath)

	hash := strippedPath + secureToken
	if expiryTimestamp > 0 {
		hash = fmt.Sprintf("%d%s", expiryTimestamp, hash)
	}

	finalHash := md5.Sum([]byte(hash))
	encodedFinalHash := base64.StdEncoding.EncodeToString(finalHash[:])
	encodedFinalHash = strings.NewReplacer("+", "-", "/", "_").Replace(encodedFinalHash)

	// signedURL := "https://" + fmt.Sprintf("%s/%s", host, encodedFinalHash)
	// if expiryTimestamp > 0 {
	// 	signedURL += fmt.Sprintf(",%d", expiryTimestamp)
	// }
	// signedURL += filePath

	if expiryTimestamp > 0 {
		return fmt.Sprintf("%s,%d", encodedFinalHash, expiryTimestamp), nil
	}

	return encodedFinalHash, nil
}

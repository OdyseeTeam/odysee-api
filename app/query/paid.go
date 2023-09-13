package query

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
)

type qkv [2]string

type urlQuery struct {
	basePath string
}

func (q urlQuery) render(kvs ...qkv) string {
	qs := ""
	for _, kv := range kvs {
		qs += fmt.Sprintf("%s=%s", kv[0], kv[1])
	}
	return qs
}

func (q urlQuery) hash(kvs ...qkv) string {
	h := md5.New()
	h.Write([]byte(fmt.Sprintf("%s?%s", q.basePath, q.render(kvs...))))
	return hex.EncodeToString(h.Sum(nil))
}

func signStreamURL(path, query string) string {
	h := md5.Sum([]byte(fmt.Sprintf("%s?%s", path, query)))
	s := hex.EncodeToString(h[:])
	logger.Log().Debugf("signing url: %s?%s, signed: %s", path, query, s)
	return s
}

func signStreamURL77(cdnResourceURL, filePath, secureToken string, expiryTimestamp int) (string, error) {
	strippedPath := path.Dir(filePath)

	hash := strippedPath + secureToken
	if expiryTimestamp > 0 {
		hash = fmt.Sprintf("%d%s", expiryTimestamp, hash)
	}

	finalHash := md5.Sum([]byte(hash))
	encodedFinalHash := hex.EncodeToString(finalHash[:])
	encodedFinalHash = strings.NewReplacer("+", "-", "/", "_").Replace(encodedFinalHash)

	signedURL := fmt.Sprintf("https://%s/%s", cdnResourceURL, encodedFinalHash)
	if expiryTimestamp > 0 {
		signedURL += fmt.Sprintf(",%d", expiryTimestamp)
	}
	signedURL += filePath

	return signedURL, nil
}

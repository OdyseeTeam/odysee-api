package query

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
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
	h := md5.New()
	h.Write([]byte(fmt.Sprintf("%s?%s", path, query)))
	s := hex.EncodeToString(h.Sum(nil))
	logger.Log().Debugf("signing url: %s?%s, signed: %s", path, query, s)
	return s
}

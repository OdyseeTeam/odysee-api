package olapdb

import (
	"net"
	"strings"

	"github.com/oschwald/maxminddb-golang"
)

var geodb *maxminddb.Reader

type record struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

func OpenGeoDB(file string) error {
	var err error
	geodb, err = maxminddb.Open(file)
	if err != nil {
		return err
	}
	return nil
}

func getClientArea(ip string) string {
	r := record{}
	err := geodb.Lookup(net.ParseIP(ip), &r)
	if err != nil {
		return ""
	}
	return strings.ToLower(r.Country.ISOCode)
}

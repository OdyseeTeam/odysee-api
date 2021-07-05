package olapdb

import (
	"net"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

var geodb *geoip2.Reader

type record struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

func OpenGeoDB(file string) error {
	var err error
	geodb, err = geoip2.Open(file)
	if err != nil {
		return err
	}
	return nil
}

func getArea(ip string) string {
	// r := record{}
	// err := geodb.Lookup(net.ParseIP(ip), &r)
	area := ""

	record, err := geodb.City(net.ParseIP(ip))
	if err != nil {
		return ""
	}

	area = record.Country.IsoCode
	if len(record.Subdivisions) >= 2 {
		area += "-" + record.Subdivisions[1].IsoCode
	}
	return strings.ToLower(area)
}

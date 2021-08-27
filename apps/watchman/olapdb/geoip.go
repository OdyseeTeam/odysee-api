package olapdb

import (
	"net"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

var geodb *geoip2.Reader

func OpenGeoDB(file string) error {
	var err error
	geodb, err = geoip2.Open(file)
	if err != nil {
		return err
	}
	return nil
}

func getArea(ip string) (string, string) {
	var area, subarea string

	record, err := geodb.City(net.ParseIP(ip))
	if err != nil {
		return "", ""
	}

	area = record.Country.IsoCode
	if len(record.Subdivisions) >= 2 {
		subarea = record.Subdivisions[1].IsoCode
	}
	return strings.ToLower(area), strings.ToLower(subarea)
}

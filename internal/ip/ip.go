package ip

import (
	"bytes"
	"net"
	"net/http"
	"strings"

	"github.com/lbryio/lbry.go/v2/extras/util"
)

// most of this is from https://husobee.github.io/golang/ip-address/2015/12/17/remote-ip-go.html

//ipRange holds the start and end of a range of ip addresses
type ipRange struct {
	start net.IP
	end   net.IP
}

// Contains returns true if the ipRange contains the ip address
func (r ipRange) Contains(ip net.IP) bool {
	return bytes.Compare(ip, r.start) >= 0 && bytes.Compare(ip, r.end) < 0
}

var privateRanges = []ipRange{
	{
		start: net.ParseIP("10.0.0.0"),
		end:   net.ParseIP("10.255.255.255"),
	},
	{
		start: net.ParseIP("100.64.0.0"),
		end:   net.ParseIP("100.127.255.255"),
	},
	{
		start: net.ParseIP("127.0.0.1"),
		end:   net.ParseIP("127.255.255.255"),
	},
	{
		start: net.ParseIP("172.16.0.0"),
		end:   net.ParseIP("172.31.255.255"),
	},
	{
		start: net.ParseIP("192.0.0.0"),
		end:   net.ParseIP("192.0.0.255"),
	},
	{
		start: net.ParseIP("192.168.0.0"),
		end:   net.ParseIP("192.168.255.255"),
	},
	{
		start: net.ParseIP("198.18.0.0"),
		end:   net.ParseIP("198.19.255.255"),
	},
}

// IsPrivateSubnet checks if this ip is in a private subnet
func IsPrivateSubnet(ipAddress net.IP) bool {
	// my use case is only concerned with ipv4 atm
	if ipCheck := ipAddress.To4(); ipCheck != nil {
		// iterate over all our ranges
		for _, r := range privateRanges {
			// check if this ip is in a private range
			if r.Contains(ipAddress) {
				return true
			}
		}
	}
	return false
}

// AddressForRequest returns the real IP address of the request
func AddressForRequest(r *http.Request) string {
	orgAddresses := []string{
		"34.231.101.5",
		"18.222.104.49",
		"18.191.94.135",
		"3.136.112.165",
		"18.223.116.236",
		"13.59.124.208",
		"3.19.54.244",
		"3.21.166.205",
		"3.16.131.26",
		"3.14.128.5",
		"3.133.116.6",
		"3.133.83.252",
		"3.19.143.233",
		"3.22.63.204",
		"3.12.147.187",
		"18.217.80.240",
		"13.59.197.247",
	}
	for _, h := range []string{"X-Forwarded-For", "X-Real-Ip"} {
		addresses := strings.Split(r.Header.Get(h), ",")
		// march from right to left until we get a public address
		// that will be the address right before our proxy.
		for i := len(addresses) - 1; i >= 0; i-- {
			addr := strings.TrimSpace(addresses[i])
			// header can contain spaces too, strip those out.
			realIP := net.ParseIP(addr)
			if !realIP.IsGlobalUnicast() || IsPrivateSubnet(realIP) {
				// bad address, go to next
				continue
			}
			return addr
		}
	}

	ipParts := strings.Split(r.RemoteAddr, ":")
	addr := strings.Join(ipParts[:len(ipParts)-1], ":")

	if addr == "[::1]" {
		return "127.0.0.1"
	}
	return addr
}

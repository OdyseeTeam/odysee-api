package ip

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

var expectedIPs = map[string]string{
	"127.0.0.1, 203.0.113.195":                         "203.0.113.195",
	"127.0.0.1":                                        "",
	"2001:db8:85a3:8d3:1319:8a2e:370:7348":             "2001:db8:85a3:8d3:1319:8a2e:370:7348",
	"127.0.0.1, 2001:db8:85a3:8d3:1319:8a2e:370:7348":  "2001:db8:85a3:8d3:1319:8a2e:370:7348",
	"127.0.0.1, 127.0.0.1, 127.0.0.1, 150.172.238.178": "150.172.238.178",
	"150.172.238.178, 127.0.0.1, 127.0.0.1, 127.0.0.1": "150.172.238.178",
	"150.172.238.178, 70.41.3.18, 127.0.0.1":           "70.41.3.18",
	"127.0.0.1, 192.168.0.1, 70.41.3.18, 127.0.0.1":    "70.41.3.18",
}

func TestForRequest(t *testing.T) {
	for val, exp := range expectedIPs {
		t.Run(val, func(t *testing.T) {
			r, _ := http.NewRequest(http.MethodGet, "", nil)
			r.Header.Add("X-Forwarded-For", val)
			assert.Equal(t, exp, ForRequest(r))
		})
	}
}

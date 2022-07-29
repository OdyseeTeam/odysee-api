package iapi

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/Pallinder/go-randomdata"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallCustomerList(t *testing.T) {
	oat, err := wallet.GetTestToken()
	require.NoError(t, err)

	type testCase struct {
		name          string
		genClient     func() *Client
		params        map[string]string
		errorCheck    func(err error)
		apiErrorCheck func(*string)
		exSuccess     bool
		dataCheck     func(testCase, *CustomerListResponse)
	}
	cases := []testCase{
		{
			name: "success with oauth token",
			genClient: func() *Client {
				c, err := NewClient(WithOAuthToken(oat.AccessToken))
				require.NoError(t, err)
				return c
			},
			params:        map[string]string{"claim_id_filter": "81b1749f773bad5b9b53d21508051560f2746cdc"},
			exSuccess:     true,
			apiErrorCheck: nil,
			dataCheck: func(c testCase, r *CustomerListResponse) {
				d := r.Data[0]
				assert.Equal(t, c.params["claim_id_filter"], d.TargetClaimID)
				assert.Equal(t, "confirmed", d.Status)
			},
		},
		{
			name: "non-existent purchase",
			genClient: func() *Client {
				c, err := NewClient(WithOAuthToken(oat.AccessToken))
				require.NoError(t, err)
				return c
			},
			params:        map[string]string{"claim_id_filter": "0590f924bbee6627a2e79f7f2ff7dfb50bf2877c"},
			apiErrorCheck: nil,
			exSuccess:     true,
			dataCheck: func(_ testCase, r *CustomerListResponse) {
				assert.Empty(t, r.Data)
			},
		},
		{
			name: "invalid token",
			genClient: func() *Client {
				c, err := NewClient(WithLegacyToken("invalidToken"))
				require.NoError(t, err)
				return c
			},
			params:        map[string]string{"claim_id_filter": "0590f924bbee6627a2e79f7f2ff7dfb50bf2877c"},
			exSuccess:     false,
			apiErrorCheck: func(s *string) { assert.Regexp(t, regexp.MustCompile("could not authenticate user"), *s) },
			dataCheck: func(_ testCase, r *CustomerListResponse) {
				assert.Empty(t, r.Data)
			},
		},
		{
			name: "invalid server",
			genClient: func() *Client {
				c, err := NewClient(
					WithLegacyToken("invalidToken"),
					WithServer(fmt.Sprintf("http://localhost:%v", randomdata.Number(10000, 65535))))
				require.NoError(t, err)
				return c
			},
			params:     map[string]string{"claim_id_filter": "0590f924bbee6627a2e79f7f2ff7dfb50bf2877c"},
			exSuccess:  false,
			errorCheck: func(e error) { assert.ErrorContains(t, e, "connection refused") },
		},
	}
	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			r := &CustomerListResponse{}
			c := cs.genClient()
			err = c.Call("customer/list", cs.params, r)
			if cs.errorCheck != nil {
				cs.errorCheck(err)
			} else {
				require.Nil(t, err)
			}
			assert.Equal(t, cs.exSuccess, r.Success)
			if cs.apiErrorCheck != nil {
				cs.apiErrorCheck(r.Error)
			} else {
				assert.Nil(t, r.Error)
			}
			if cs.dataCheck != nil {
				cs.dataCheck(cs, r)
			}
		})
	}
}

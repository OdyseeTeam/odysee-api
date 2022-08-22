package query

import (
	"context"
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/query/cache"
	"github.com/OdyseeTeam/odysee-api/app/rpcerrors"
	"github.com/OdyseeTeam/odysee-api/app/sdkrouter"
	"github.com/OdyseeTeam/odysee-api/app/wallet"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/errors"
	"github.com/OdyseeTeam/odysee-api/internal/test"
	"github.com/OdyseeTeam/player-server/pkg/paid"
	"github.com/Pallinder/go-randomdata"

	ljsonrpc "github.com/lbryio/lbry.go/v2/extras/jsonrpc"

	"github.com/sirupsen/logrus"
	logrusTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ybbus/jsonrpc"
)

var bgctx = func() context.Context { return context.Background() }

func parseRawResponse(t *testing.T, rawCallResponse []byte, v interface{}) {
	assert.NotNil(t, rawCallResponse)
	var res jsonrpc.RPCResponse
	err := json.Unmarshal(rawCallResponse, &res)
	require.NoError(t, err)
	err = res.GetObject(v)
	require.NoError(t, err)
}

func TestCaller_CallBlankEndpoint(t *testing.T) {
	c := NewCaller("", 0)
	_, err := c.Call(bgctx(), jsonrpc.NewRequest("status"))
	require.EqualError(t, err, "cannot call blank endpoint")
}

func TestCaller_CallRelaxedMethods(t *testing.T) {
	config.Override("LbrynetXPercentage", 0)
	defer config.RestoreOverridden()
	for _, m := range relaxedMethods {
		if m == MethodStatus || m == MethodGet {
			continue
		}
		t.Run(m, func(t *testing.T) {
			reqChan := test.ReqChan()
			srv := test.MockHTTPServer(reqChan)
			defer srv.Close()
			srv.NextResponse <- test.EmptyResponse()

			caller := NewCaller(srv.URL, 0)
			resp, err := caller.Call(bgctx(), jsonrpc.NewRequest(m))
			assert.Nil(t, resp)
			assert.Error(t, err)                                       // empty response should be an error
			assert.False(t, errors.Is(err, rpcerrors.ErrAuthRequired)) // but it should not be an auth error

			receivedRequest := <-reqChan
			expectedRequest := test.ReqToStr(t, &jsonrpc.RPCRequest{
				Method:  m,
				Params:  nil,
				JSONRPC: "2.0",
			})
			assert.EqualValues(t, expectedRequest, receivedRequest.Body)
		})
	}
}

func TestCaller_CallAmbivalentMethodsWithoutWallet(t *testing.T) {
	config.Override("LbrynetXPercentage", 0)
	defer config.RestoreOverridden()
	for _, m := range relaxedMethods {
		if !methodInList(m, walletSpecificMethods) || m == MethodGet {
			continue
		}
		t.Run(m, func(t *testing.T) {
			reqChan := test.ReqChan()
			srv := test.MockHTTPServer(reqChan)
			defer srv.Close()
			caller := NewCaller(srv.URL, 0)
			srv.NextResponse <- test.EmptyResponse()
			resp, err := caller.Call(bgctx(), jsonrpc.NewRequest(m))
			assert.Nil(t, resp)
			assert.Error(t, err) // empty response should be an error
			assert.False(t, errors.Is(err, rpcerrors.ErrAuthRequired))

			receivedRequest := <-reqChan
			expectedRequest := test.ReqToStr(t, &jsonrpc.RPCRequest{
				Method:  m,
				Params:  nil,
				JSONRPC: "2.0",
			})
			assert.EqualValues(t, expectedRequest, receivedRequest.Body)
		})
	}
}

func TestCaller_CallAmbivalentMethodsWithWallet(t *testing.T) {
	dummyUserID := 123321
	var methodsTested int

	for _, m := range relaxedMethods {
		if !methodInList(m, walletSpecificMethods) || m == MethodGet {
			continue
		}
		methodsTested++
		t.Run(m, func(t *testing.T) {
			reqChan := test.ReqChan()
			srv := test.MockHTTPServer(reqChan)
			defer srv.Close()
			srv.NextResponse <- test.EmptyResponse()
			authedCaller := NewCaller(srv.URL, dummyUserID)

			resp, err := authedCaller.Call(bgctx(), jsonrpc.NewRequest(m))
			assert.Nil(t, resp)
			assert.Error(t, err) // empty response should be an error
			assert.False(t, errors.Is(err, rpcerrors.ErrAuthRequired))

			receivedRequest := <-reqChan
			expectedRequest := test.ReqToStr(t, &jsonrpc.RPCRequest{
				Method: m,
				Params: map[string]interface{}{
					"wallet_id": sdkrouter.WalletID(dummyUserID),
				},
				JSONRPC: "2.0",
			})
			expectedRequestLbrynetX := test.ReqToStr(t, &jsonrpc.RPCRequest{
				Method: m,
				Params: map[string]interface{}{
					"wallet_id":      sdkrouter.WalletID(dummyUserID),
					"new_sdk_server": config.GetLbrynetXServer(),
				},
				JSONRPC: "2.0",
			})

			if expectedRequest != receivedRequest.Body {
				// TODO: Remove when new_sdk_server is not used anymore
				assert.EqualValues(t, expectedRequestLbrynetX, receivedRequest.Body)
			}
		})
	}

	if methodsTested == 0 {
		t.Fatal("no ambivalent methods found, that doesn't look right")
	}
}

func TestCaller_CallNonRelaxedMethods(t *testing.T) {
	for _, m := range walletSpecificMethods {
		if methodInList(m, relaxedMethods) {
			continue
		}
		t.Run(m, func(t *testing.T) {
			reqChan := test.ReqChan()
			srv := test.MockHTTPServer(reqChan)
			defer srv.Close()

			caller := NewCaller(srv.URL, 0)
			resp, err := caller.Call(bgctx(), jsonrpc.NewRequest(m))
			assert.Nil(t, resp)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, rpcerrors.ErrAuthRequired))
		})
	}
}

func TestCaller_CallForbiddenMethod(t *testing.T) {
	caller := NewCaller(test.RandServerAddress(t), 0)
	resp, err := caller.Call(bgctx(), jsonrpc.NewRequest("stop"))
	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Equal(t, "forbidden method", err.Error())
}

func TestCaller_CallAttachesWalletID(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	dummyUserID := 123321

	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()
	srv.NextResponse <- test.EmptyResponse()
	caller := NewCaller(srv.URL, dummyUserID)
	caller.Call(bgctx(), jsonrpc.NewRequest("channel_create", map[string]interface{}{"name": "test", "bid": "0.1"}))
	receivedRequest := <-reqChan

	expectedRequest := test.ReqToStr(t, &jsonrpc.RPCRequest{
		Method: "channel_create",
		Params: map[string]interface{}{
			"name":      "test",
			"bid":       "0.1",
			"wallet_id": sdkrouter.WalletID(dummyUserID),
		},
		JSONRPC: "2.0",
	})
	assert.EqualValues(t, expectedRequest, receivedRequest.Body)
}

func TestCaller_AddPreflightHookAmendingQueryParams(t *testing.T) {
	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()

	c := NewCaller(srv.URL, 0)

	c.AddPreflightHook(relaxedMethods[0], func(_ *Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
		params := GetQuery(ctx).ParamsAsMap()
		if params == nil {
			GetQuery(ctx).Request.Params = map[string]string{"param": "123"}
		} else {
			params["param"] = "123"
			GetQuery(ctx).Request.Params = params
		}
		return nil, nil
	}, "")

	srv.NextResponse <- test.EmptyResponse()

	c.Call(bgctx(), jsonrpc.NewRequest(relaxedMethods[0]))
	req := <-reqChan
	lastRequest := test.StrToReq(t, req.Body)

	p, ok := lastRequest.Params.(map[string]interface{})
	assert.True(t, ok, req.Body)
	assert.Equal(t, "123", p["param"], req.Body)
}

func TestCaller_AddPreflightHookReturningEarlyResponse(t *testing.T) {
	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()

	c := NewCaller(srv.URL, 0)

	c.AddPreflightHook(relaxedMethods[0], func(_ *Caller, _ context.Context) (*jsonrpc.RPCResponse, error) {
		return &jsonrpc.RPCResponse{Result: map[string]string{"ok": "ok"}}, nil
	}, "")

	srv.NextResponse <- test.EmptyResponse()

	res, err := c.Call(bgctx(), jsonrpc.NewRequest(relaxedMethods[0]))
	require.NoError(t, err)

	assert.Equal(t, map[string]string{"ok": "ok"}, res.Result)
}

func TestCaller_AddPreflightHookReturningError(t *testing.T) {
	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()

	c := NewCaller(srv.URL, 0)

	c.AddPreflightHook(relaxedMethods[0], func(_ *Caller, _ context.Context) (*jsonrpc.RPCResponse, error) {
		return &jsonrpc.RPCResponse{Result: map[string]string{"ok": "ok"}}, errors.Err("an error occured")
	}, "")

	srv.NextResponse <- test.EmptyResponse()

	res, err := c.Call(bgctx(), jsonrpc.NewRequest(relaxedMethods[0]))
	assert.EqualError(t, err, "an error occured")
	assert.Nil(t, res)
}

func TestCaller_AddPostflightHook_Response(t *testing.T) {
	dummyUserID := randomdata.Number(1, 99999)
	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()
	addr := test.RandServerAddress(t)
	err := wallet.Create(addr, dummyUserID)
	require.NoError(t, err)
	c := NewCaller(srv.URL, dummyUserID)

	srv.NextResponse <- `
	{
	"jsonrpc": "2.0",
	"result": {
		"available": "64.36180199",
		"reserved": "0.0",
		"reserved_subtotals": {
		"claims": "0.0",
		"supports": "0.0",
		"tips": "0.0"
		},
		"total": "64.36180199"
	},
	"id": 0
	}
	`

	c.AddPostflightHook("wallet_", func(c *Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
		r := GetResponse(ctx)
		r.Result = "0.0"
		return r, nil
	}, "")

	res, err := c.Call(bgctx(), jsonrpc.NewRequest(MethodWalletBalance))
	require.NoError(t, err)
	assert.Equal(t, "0.0", res.Result)
}

func TestCaller_AddPostflightHook_LogField(t *testing.T) {
	logHook := logrusTest.NewLocal(logger.Entry.Logger)
	logger.Entry.Logger.SetLevel(logrus.TraceLevel)
	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()

	c := NewCaller(srv.URL, 0)
	srv.NextResponse <- `
	{
		"jsonrpc": "2.0",
		"result": {
		  "what:19b9c243bea0c45175e6a6027911abbad53e983e": {
			"error": "what:19b9c243bea0c45175e6a6027911abbad53e983e is not a valid url"
		  }
		},
		"id": 0
	}
	`

	c.AddPostflightHook(MethodResolve, func(c *Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
		WithLogField(ctx, "remote_ip", "8.8.8.8")
		return nil, nil
	}, "")

	res, err := c.Call(bgctx(), jsonrpc.NewRequest(MethodResolve, map[string]interface{}{"urls": "what:19b9c243bea0c45175e6a6027911abbad53e983e"}))
	require.NoError(t, err)
	assert.Contains(t, res.Result.(map[string]interface{}), "what:19b9c243bea0c45175e6a6027911abbad53e983e")
	assert.Equal(t, "8.8.8.8", logHook.LastEntry().Data["remote_ip"])
}

func TestCaller_CloneWithoutHook(t *testing.T) {
	timesCalled := 0
	call := func() {
		timesCalled++
	}

	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()

	c := NewCaller(srv.URL, 0)
	srv.QueueResponses(resolveResponseWithoutPurchase, resolveResponseWithoutPurchase)

	c.AddPostflightHook(MethodResolve, func(c *Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
		call()
		return nil, nil
	}, "")

	c.AddPostflightHook(MethodResolve, func(c *Caller, ctx context.Context) (*jsonrpc.RPCResponse, error) {
		// This will be cloned without the current hook but the previous one should increment `timesCalled` once again
		cc := c.CloneWithoutHook(c.Endpoint(), MethodResolve, "lbrynext_resolve")
		q := GetQuery(ctx)
		_, err := cc.SendQuery(ctx, q)
		assert.NoError(t, err)
		return nil, nil
	}, "lbrynext_resolve")

	_, err := c.Call(bgctx(), jsonrpc.NewRequest(MethodResolve, map[string]interface{}{"urls": "what:19b9c243bea0c45175e6a6027911abbad53e983e"}))
	require.NoError(t, err)
	assert.Equal(t, timesCalled, 2)
}

func TestCaller_CallCachingResponses(t *testing.T) {
	var err error
	srv := test.MockHTTPServer(nil)
	defer srv.Close()
	srv.NextResponse <- `
	{
		"jsonrpc": "2.0",
		"result": {
		  "blocked": {
			"channels": [],
			"total": 0
		  },
		  "items": [
			{
			  "address": "bHz3LpVcuadmbK8g6VVUszF9jjH4pxG2Ct",
			  "amount": "0.5",
			  "canonical_url": "lbry://@lbry#3f/youtube-is-over-lbry-odysee-are-here#4"
			}
		  ]
		},
		"id": 0
	}
	`

	c := NewCaller(srv.URL, 0)
	c.Cache, err = cache.New(cache.DefaultConfig())
	require.NoError(t, err)
	rpcResponse, err := c.Call(bgctx(), jsonrpc.NewRequest("claim_search", map[string]interface{}{"urls": "what"}))
	require.NoError(t, err)
	assert.Nil(t, rpcResponse.Error)
	c.Cache.Wait()
	cResp, err := c.Cache.Retrieve("claim_search", map[string]interface{}{"urls": "what"}, nil)
	require.NoError(t, err)
	assert.NotNil(t, cResp.(*jsonrpc.RPCResponse).Result)
}

func TestCaller_CallNotCachingErrors(t *testing.T) {
	var err error
	srv := test.MockHTTPServer(nil)
	defer srv.Close()
	srv.NextResponse <- `
		{
			"jsonrpc": "2.0",
			"error": {
			  "code": -32000,
			  "message": "sqlite query timed out"
			},
			"id": 0
		}`

	c := NewCaller(srv.URL, 0)
	c.Cache, err = cache.New(cache.DefaultConfig())
	require.NoError(t, err)
	rpcResponse, err := c.Call(bgctx(), jsonrpc.NewRequest("claim_search", map[string]interface{}{"urls": "what"}))
	require.NoError(t, err)
	assert.Equal(t, rpcResponse.Error.Code, -32000)
	time.Sleep(500 * time.Millisecond)
	cResp, err := c.Cache.Retrieve(
		"claim_search",
		map[string]interface{}{"urls": "what"},
		func() (interface{}, error) { return nil, nil })
	require.NoError(t, err)
	assert.Nil(t, cResp)
}

func TestCaller_CallSDKError(t *testing.T) {
	srv := test.MockHTTPServer(nil)
	defer srv.Close()
	srv.NextResponse <- `
		{
			"jsonrpc": "2.0",
			"error": {
			  "code": -32500,
			  "message": "After successfully executing the command, failed to encode result for JSON RPC response.",
			  "data": [
				"Traceback (most recent call last):",
				"  File \"lbry/extras/daemon/Daemon.py\", line 482, in handle_old_jsonrpc",
				"  File \"lbry/extras/daemon/Daemon.py\", line 235, in jsonrpc_dumps_pretty",
				"  File \"json/__init__.py\", line 238, in dumps",
				"  File \"json/encoder.py\", line 201, in encode",
				"  File \"json/encoder.py\", line 431, in _iterencode",
				"  File \"json/encoder.py\", line 405, in _iterencode_dict",
				"  File \"json/encoder.py\", line 405, in _iterencode_dict",
				"  File \"json/encoder.py\", line 325, in _iterencode_list",
				"  File \"json/encoder.py\", line 438, in _iterencode",
				"  File \"lbry/extras/daemon/json_response_encoder.py\", line 118, in default",
				"  File \"lbry/extras/daemon/json_response_encoder.py\", line 151, in encode_output",
				"  File \"torba/client/baseheader.py\", line 75, in __getitem__",
				"  File \"lbry/wallet/header.py\", line 35, in deserialize",
				"struct.error: unpack requires a buffer of 4 bytes",
				""
			  ]
			},
			"id": 0
		}`

	c := NewCaller(srv.URL, 0)
	hook := logrusTest.NewLocal(logger.Entry.Logger)
	rpcResponse, err := c.Call(bgctx(), jsonrpc.NewRequest("resolve", map[string]interface{}{"urls": "what"}))
	require.NoError(t, err)
	assert.Equal(t, rpcResponse.Error.Code, -32500)
	assert.Equal(t, "query", hook.LastEntry().Data["module"])
	assert.Equal(t, "resolve", hook.LastEntry().Data["method"])
}

func TestCaller_ClientJSONError(t *testing.T) {
	ts := test.MockHTTPServer(nil)
	defer ts.Close()
	ts.NextResponse <- `{"method":"version}` // note the missing close quote after "version

	c := NewCaller(ts.URL, 0)
	_, err := c.Call(bgctx(), jsonrpc.NewRequest(MethodResolve, map[string]interface{}{"urls": "what"}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "could not decode body to rpc response")
}

func TestCaller_CallRaw(t *testing.T) {
	c := NewCaller(test.RandServerAddress(t), 0)
	for _, rawQ := range []string{`{}`, `{"method": " "}`} {
		t.Run(rawQ, func(t *testing.T) {
			_, err := c.Call(bgctx(), test.StrToReq(t, rawQ))
			assert.Error(t, err)
			assert.Equal(t, "no method in request", err.Error())
		})
	}
}

func TestCaller_Resolve(t *testing.T) {
	resolvedURL := "what#6769855a9aa43b67086f9ff3c1a5bacb5698a27a"
	resolvedClaimID := "6769855a9aa43b67086f9ff3c1a5bacb5698a27a"

	request := jsonrpc.NewRequest("resolve", map[string]interface{}{"urls": resolvedURL})
	rpcRes, err := NewCaller(test.RandServerAddress(t), 0).Call(bgctx(), request)
	require.NoError(t, err)
	require.Nil(t, rpcRes.Error)

	var resolveResponse ljsonrpc.ResolveResponse
	err = rpcRes.GetObject(&resolveResponse)
	require.NoError(t, err)
	assert.Equal(t, resolvedClaimID, resolveResponse[resolvedURL].ClaimID)
}

func TestCaller_WalletBalance(t *testing.T) {
	dummyUserID := randomdata.Number(1, 99999)

	request := jsonrpc.NewRequest("wallet_balance")

	rpcRes, err := NewCaller(test.RandServerAddress(t), 0).Call(bgctx(), request)
	assert.Nil(t, rpcRes)
	assert.True(t, errors.Is(err, rpcerrors.ErrAuthRequired))

	addr := test.RandServerAddress(t)
	err = wallet.Create(addr, dummyUserID)
	require.NoError(t, err, dummyUserID)

	hook := logrusTest.NewLocal(logger.Entry.Logger)
	rpcRes, err = NewCaller(addr, dummyUserID).Call(bgctx(), request)
	require.NoError(t, err)

	var accountBalanceResponse struct {
		Available string `json:"available"`
	}
	err = rpcRes.GetObject(&accountBalanceResponse)
	require.NoError(t, err)
	assert.EqualValues(t, "0.0", accountBalanceResponse.Available)
	assert.Equal(t, map[string]interface{}{"wallet_id": sdkrouter.WalletID(dummyUserID)}, hook.LastEntry().Data["params"])
	assert.Equal(t, "wallet_balance", hook.LastEntry().Data["method"])
}

func TestCaller_CallQueryWithRetry(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	dummyUserID := rand.Intn(100)
	addr := test.RandServerAddress(t)

	err := wallet.Create(addr, dummyUserID)
	require.NoError(t, err)
	err = wallet.UnloadWallet(addr, dummyUserID)
	require.NoError(t, err)

	q, err := NewQuery(jsonrpc.NewRequest("wallet_balance"), sdkrouter.WalletID(dummyUserID))
	require.NoError(t, err)

	// check that sdk loads the wallet and retries the query if the wallet was not initially loaded

	c := NewCaller(addr, dummyUserID)
	r, err := c.SendQuery(WithQuery(bgctx(), q), q)
	require.NoError(t, err)
	require.Nil(t, r.Error)
}

func TestCaller_timeouts(t *testing.T) {
	srv := test.MockHTTPServer(nil)
	defer srv.Close()

	config.Override("RPCTimeouts", map[string]string{
		"resolve": "300ms",
	})
	defer config.RestoreOverridden()

	c := NewCaller(srv.URL, 0)
	q, err := NewQuery(jsonrpc.NewRequest("resolve"), "")
	require.NoError(t, err)
	go func() {
		time.Sleep(200 * time.Millisecond)
		srv.NextResponse <- test.ResToStr(t, &jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Result:  `""`,
		})
		time.Sleep(700 * time.Millisecond)
		srv.NextResponse <- test.ResToStr(t, &jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Result:  `""`,
		})
	}()

	_, err = c.SendQuery(WithQuery(bgctx(), q), q)
	require.NoError(t, err)

	_, err = c.SendQuery(WithQuery(bgctx(), q), q)
	require.Error(t, err, `timeout awaiting response headers`)
}

func TestCaller_DontReloadWalletAfterOtherErrors(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	walletID := sdkrouter.WalletID(rand.Intn(100))

	srv := test.MockHTTPServer(nil)
	defer srv.Close()

	c := NewCaller(srv.URL, 0)
	q, err := NewQuery(jsonrpc.NewRequest("wallet_balance"), walletID)
	require.NoError(t, err)

	srv.QueueResponses(
		test.ResToStr(t, &jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Error: &jsonrpc.RPCError{
				Message: "Couldn't find wallet: //",
				Data: map[string]interface{}{
					"name": ljsonrpc.ErrorWalletNotFound,
				},
			},
		}),
		test.EmptyResponse(), // for the wallet_add call
		test.ResToStr(t, &jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Error: &jsonrpc.RPCError{
				Message: "Couldn't find wallet: //",
				Data: map[string]interface{}{
					"name": ljsonrpc.ErrorWalletNotFound,
				},
			},
		}),
	)

	r, err := c.SendQuery(WithQuery(bgctx(), q), q)
	require.NoError(t, err)
	require.Equal(t, "Couldn't find wallet: //", r.Error.Message)
}

func TestCaller_DontReloadWalletIfAlreadyLoaded(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	walletID := sdkrouter.WalletID(rand.Intn(100))

	srv := test.MockHTTPServer(nil)
	defer srv.Close()

	c := NewCaller(srv.URL, 0)
	q, err := NewQuery(jsonrpc.NewRequest("wallet_balance"), walletID)
	require.NoError(t, err)

	srv.QueueResponses(
		test.ResToStr(t, &jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Error: &jsonrpc.RPCError{
				Message: "Wallet // is not loaded",
				Data: map[string]interface{}{
					"name": ljsonrpc.ErrorWalletNotLoaded,
				},
			},
		}),
		test.EmptyResponse(), // for the wallet_add call
		test.ResToStr(t, &jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Error: &jsonrpc.RPCError{
				Message: "Wallet at path // is already loaded",
				Data: map[string]interface{}{
					"name": ljsonrpc.ErrorWalletAlreadyLoaded,
				},
			},
		}),
		test.ResToStr(t, &jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Result:  `"99999.00"`,
		}),
	)

	r, err := c.SendQuery(WithQuery(bgctx(), q), q)

	require.NoError(t, err)
	require.Nil(t, r.Error)
	require.Equal(t, `"99999.00"`, r.Result)
}

func TestCaller_Status(t *testing.T) {
	c := NewCaller(test.RandServerAddress(t), 0)
	rpcResponse, err := c.Call(bgctx(), jsonrpc.NewRequest("status"))
	require.NoError(t, err)
	assert.Equal(t,
		"692EAWhtoqDuAfQ6KHMXxFxt8tkhmt7sfprEMHWKjy5hf6PwZcHDV542VHqRnFnTCD",
		rpcResponse.Result.(map[string]interface{})["installation_id"].(string))
}

func TestCaller_GetFreeUnauthenticated(t *testing.T) {
	config.Override("FreeContentURL", "https://player.odycdn.com/api/v4/streams/free/")
	defer config.RestoreOverridden()

	srvAddress := test.RandServerAddress(t)
	uri := "what#19b9c243bea0c45175e6a6027911abbad53e983e"

	request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": uri})
	resp, err := NewCaller(srvAddress, 0).Call(bgctx(), request)
	require.NoError(t, err)
	require.Nil(t, resp.Error)

	getResponse := &ljsonrpc.GetResponse{}
	err = resp.GetObject(&getResponse)
	require.NoError(t, err)
	assert.Equal(t, "https://player.odycdn.com/api/v4/streams/free/what/19b9c243bea0c45175e6a6027911abbad53e983e/d51692", getResponse.StreamingURL)
}

func TestCaller_GetFreeAuthenticated(t *testing.T) {
	config.Override("FreeContentURL", "https://player.odycdn.com/api/v4/streams/free/")
	defer config.RestoreOverridden()

	uri := "what"

	dummyUserID := 123321
	srv := test.MockHTTPServer(nil)

	srv.QueueResponses(
		resolveResponseFree,
	)
	request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": uri})
	resp, err := NewCaller(srv.URL, dummyUserID).Call(bgctx(), request)
	require.NoError(t, err)
	require.Nil(t, resp.Error)

	getResponse := &ljsonrpc.GetResponse{}
	err = resp.GetObject(&getResponse)
	require.NoError(t, err)
	assert.Equal(t, "https://player.odycdn.com/api/v4/streams/free/what/19b9c243bea0c45175e6a6027911abbad53e983e/d51692", getResponse.StreamingURL)
}

func TestCaller_GetCouldntFindClaim(t *testing.T) {
	uri := "lbry://@whatever#b/whatever#4"

	dummyUserID := 123321
	srv := test.MockHTTPServer(nil)

	srv.QueueResponses(
		resolveResponseCouldntFind,
	)
	request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": uri})
	resp, err := NewCaller(srv.URL, dummyUserID).Call(bgctx(), request)
	assert.EqualError(t, err, "couldn't find claim")
	assert.Nil(t, resp)
}

func TestCaller_GetInvalidURLAuthenticated(t *testing.T) {
	config.Override("FreeContentURL", "https://player.odycdn.com/api")
	defer config.RestoreOverridden()

	uri := "what#@1||||"

	dummyUserID := 123321
	srv := test.MockHTTPServer(nil)

	srv.QueueResponses(
		resolveResponseFree,
	)
	request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": uri})
	resp, err := NewCaller(srv.URL, dummyUserID).Call(bgctx(), request)
	assert.EqualError(t, err, "could not find a corresponding entry in the resolve response")
	assert.Nil(t, resp)
}

func TestCaller_GetPaidCannotPurchase(t *testing.T) {
	dummyUserID := rand.Intn(99999)
	srvAddress := test.RandServerAddress(t)
	uri := "lbry://@specialoperationstest#3/iOS-13-AdobeXD#9"
	err := wallet.Create(srvAddress, dummyUserID)
	require.NoError(t, err)

	request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": uri})
	resp, err := NewCaller(srvAddress, dummyUserID).Call(bgctx(), request)
	assert.EqualError(t, err, "purchase error: Not enough funds to cover this transaction.")
	assert.Nil(t, resp)
}

func TestCaller_GetPaidUnauthenticated(t *testing.T) {
	srvAddress := test.RandServerAddress(t)
	uri := "lbry://@specialoperationstest#3/iOS-13-AdobeXD#9"

	request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": uri})
	resp, err := NewCaller(srvAddress, 0).Call(bgctx(), request)
	assert.EqualError(t, err, "authentication required")
	assert.Nil(t, resp)
}

func TestCaller_GetPaidPurchased(t *testing.T) {
	config.Override("PaidContentURL", "https://player.odycdn.com/api/v3/streams/paid/")
	defer config.RestoreOverridden()

	uri := "Body-Language---Robert-F.-Kennedy-Assassination---Hypnosis#d66f8ba85c85ca48daba9183bd349307fe30cb43"
	txid := "ff990688df370072f408e2db9d217d2cf331d92ac594d5e6e8391143e9d38160"
	claimName := "Body-Language---Robert-F.-Kennedy-Assassination---Hypnosis"
	sdHash := "51ee25"
	claimID := "d66f8ba85c85ca48daba9183bd349307fe30cb43"

	dummyUserID := 123321
	srv := test.MockHTTPServer(nil)
	defer srv.Close()

	srv.QueueResponses(
		resolveResponseWithPurchase,
		purchaseCreateExistingResponse,
		resolveResponseWithPurchase,
	)

	err := paid.GeneratePrivateKey()
	require.NoError(t, err)

	token, err := paid.CreateToken(claimName+"/"+claimID, txid, 585600621, paid.ExpTenSecPer100MB)
	require.NoError(t, err)

	request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": uri})
	resp, err := NewCaller(srv.URL, dummyUserID).Call(bgctx(), request)
	require.NoError(t, err)
	require.Nil(t, resp.Error)

	getResponse := &ljsonrpc.GetResponse{}
	err = resp.GetObject(&getResponse)
	require.NoError(t, err)
	assert.Equal(t, "https://player.odycdn.com/api/v3/streams/paid/"+claimName+"/"+claimID+"/"+sdHash+"/"+token, getResponse.StreamingURL)
	assert.NotNil(t, getResponse.PurchaseReceipt)
}

func TestCaller_GetPaidResolveLag(t *testing.T) {
	config.Override("PaidContentURL", "https://player.odycdn.com/api/v3/streams/paid/")
	defer config.RestoreOverridden()

	uri := "Body-Language---Robert-F.-Kennedy-Assassination---Hypnosis#d66f8ba85c85ca48daba9183bd349307fe30cb43"

	dummyUserID := 123321
	srv := test.MockHTTPServer(nil)
	defer srv.Close()

	srv.QueueResponses(
		resolveResponseWithoutPurchase,
		purchaseCreateResponse,
		resolveResponseWithoutPurchase,
	)

	request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": uri})
	_, err := NewCaller(srv.URL, dummyUserID).Call(bgctx(), request)
	require.EqualError(t, err, "couldn't find purchase receipt for paid stream")
}

func TestCaller_GetPaidPurchasedMissingPurchase(t *testing.T) {
	config.Override("PaidContentURL", "https://player.odycdn.com/api/v3/streams/paid/")
	defer config.RestoreOverridden()

	uri := "Body-Language---Robert-F.-Kennedy-Assassination---Hypnosis#d66f8ba85c85ca48daba9183bd349307fe30cb43"
	txid := "ff990688df370072f408e2db9d217d2cf331d92ac594d5e6e8391143e9d38160"
	claimName := "Body-Language---Robert-F.-Kennedy-Assassination---Hypnosis"
	sdHash := "51ee25"
	claimID := "d66f8ba85c85ca48daba9183bd349307fe30cb43"

	dummyUserID := 123321

	reqChan := test.ReqChan()
	srv := test.MockHTTPServer(reqChan)
	defer srv.Close()

	srv.QueueResponses(
		resolveResponseWithoutPurchase,
		purchaseCreateResponse,
		resolveResponseWithPurchase,
	)

	err := paid.GeneratePrivateKey()
	require.NoError(t, err)

	request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": uri})
	resp, err := NewCaller(srv.URL, dummyUserID).Call(bgctx(), request)
	require.NoError(t, err)
	require.Nil(t, resp.Error)

	token, err := paid.CreateToken(claimName+"/"+claimID, txid, 585600621, paid.ExpTenSecPer100MB)
	require.NoError(t, err)

	receivedRequest := <-reqChan
	jsonRPCRequest := test.StrToReq(t, receivedRequest.Body)
	expectedParams := jsonRPCRequest.Params.(map[string]interface{})
	assert.EqualValues(t, sdkrouter.WalletID(dummyUserID), expectedParams["wallet_id"])
	assert.EqualValues(t, uri, expectedParams["urls"])
	assert.EqualValues(t, true, expectedParams["include_purchase_receipt"])
	assert.EqualValues(t, true, expectedParams["include_protobuf"])

	receivedRequest = <-reqChan
	expectedRequest := test.ReqToStr(t, &jsonrpc.RPCRequest{
		Method: MethodPurchaseCreate,
		Params: map[string]interface{}{
			"wallet_id": sdkrouter.WalletID(dummyUserID),
			"url":       uri,
			"blocking":  true,
		},
		JSONRPC: "2.0",
	})
	assert.EqualValues(t, expectedRequest, receivedRequest.Body)

	receivedRequest = <-reqChan
	jsonRPCRequest = test.StrToReq(t, receivedRequest.Body)
	expectedParams = jsonRPCRequest.Params.(map[string]interface{})
	assert.EqualValues(t, sdkrouter.WalletID(dummyUserID), expectedParams["wallet_id"])
	assert.EqualValues(t, uri, expectedParams["urls"])
	assert.EqualValues(t, true, expectedParams["include_purchase_receipt"])
	assert.EqualValues(t, true, expectedParams["include_protobuf"])

	getResponse := &ljsonrpc.GetResponse{}
	err = resp.GetObject(&getResponse)
	require.NoError(t, err)
	assert.Equal(t, "https://player.odycdn.com/api/v3/streams/paid/"+claimName+"/"+claimID+"/"+sdHash+"/"+token, getResponse.StreamingURL)
	assert.NotNil(t, getResponse.PurchaseReceipt)
	assert.EqualValues(t, "250.0", getResponse.PurchaseReceipt.(map[string]interface{})["amount"])
}

func TestCaller_GetPaidPurchasedMissingEverything(t *testing.T) {
	config.Override("PaidContentURL", "https://player.odycdn.com/api/v3/streams/paid/")
	defer config.RestoreOverridden()

	uri := "Body-Language---Robert-F.-Kennedy-Assassination---Hypnosis#d66f8ba85c85ca48daba9183bd349307fe30cb43"

	dummyUserID := 123321
	srv := test.MockHTTPServer(nil)
	defer srv.Close()

	srv.QueueResponses(
		resolveResponseWithoutPurchase,
		purchaseCreateExistingResponse,
		resolveResponseWithoutPurchase,
	)
	request := jsonrpc.NewRequest(MethodGet, map[string]interface{}{"uri": uri})
	_, err := NewCaller(srv.URL, dummyUserID).Call(bgctx(), request)
	require.EqualError(t, err, "couldn't find purchase receipt for paid stream")
}

func TestCaller_LogLevels(t *testing.T) {
	srv := test.MockHTTPServer(nil)
	defer srv.Close()
	srv.QueueResponses(
		resolveResponseFree,
		test.ResToStr(t, &jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Result:  `"99999.00"`,
		}),
		test.ResToStr(t, &jsonrpc.RPCResponse{
			JSONRPC: "2.0",
			Result:  true,
		}),
	)

	hook := logrusTest.NewLocal(logger.Entry.Logger)
	logger.Entry.Logger.SetLevel(logrus.DebugLevel)

	c := NewCaller(srv.URL, 123)

	_, err := c.Call(bgctx(), jsonrpc.NewRequest("resolve", map[string]interface{}{"urls": "what"}))
	require.NoError(t, err)
	e := hook.LastEntry()
	assert.Equal(t, "resolve", hook.LastEntry().Data["method"])
	assert.NotNil(t, hook.LastEntry().Data["params"])
	assert.Equal(t, logrus.InfoLevel, e.Level)

	_, err = c.Call(bgctx(), jsonrpc.NewRequest("wallet_balance"))
	require.NoError(t, err)
	e = hook.LastEntry()
	assert.Equal(t, "wallet_balance", hook.LastEntry().Data["method"])
	assert.NotNil(t, hook.LastEntry().Data["params"])
	assert.Equal(t, logrus.DebugLevel, e.Level)

	_, err = c.Call(bgctx(), jsonrpc.NewRequest("sync_apply"))
	require.NoError(t, err)
	e = hook.LastEntry()
	assert.Equal(t, "sync_apply", hook.LastEntry().Data["method"])
	assert.Nil(t, hook.LastEntry().Data["params"])
	assert.Equal(t, logrus.DebugLevel, e.Level)
}

func TestCaller_cutSublistsToSize(t *testing.T) {
	mockListBig := []interface{}{"1234", "1235", "1237", "9876", "0000", "1111", "9123"}

	mockParamsBig := map[string]interface{}{"channel_ids": mockListBig,
		"include_protobuf": true, "claim_type": []interface{}{"stream"}}

	mockListBigCpy := make([]interface{}, len(mockListBig))
	copy(mockListBigCpy, mockListBig)

	mockParamsBigCut := cutSublistsToSize(mockParamsBig, maxListSizeLogged)

	assert.NotEqual(t, mockParamsBigCut, mockParamsBig)
	assert.Equal(t, mockParamsBig["claim_type"], mockParamsBigCut["claim_type"])
	assert.Equal(t, mockListBig[0:5], mockParamsBigCut["channel_ids"].([]interface{})[0:5])
	assert.Equal(t, mockListBig, mockListBigCpy)
}

func TestCaller_JSONRPCNotCut(t *testing.T) {
	var err error
	srv := test.MockHTTPServer(nil)
	defer srv.Close()
	srv.NextResponse <- `
	{
		"jsonrpc": "2.0",
		"result": {
		  "blocked": {
			"channels": [],
			"total": 0
		  },
		  "items": [
			{
			  "address": "bHz3LpVcuadmbK8g6VVUszF9jjH4pxG2Ct",
			  "amount": "0.5",
			  "canonical_url": "lbry://@lbry#3f/youtube-is-over-lbry-odysee-are-here#4"
			}
		  ]
		},
		"id": 0
	}
	`

	c := NewCaller(srv.URL, 0)
	c.Cache, err = cache.New(cache.DefaultConfig())
	require.NoError(t, err)

	channelIds := []interface{}{"1234", "4321", "5678", "8765", "9999", "0000", "1111"}
	params := map[string]interface{}{"channel_ids": channelIds, "urls": "what", "number": 1}

	channelIdscpy := make([]interface{}, len(channelIds))
	copy(channelIdscpy, channelIds)

	req := jsonrpc.NewRequest("claim_search", params)

	_, err = c.Call(bgctx(), req)
	require.NoError(t, err)

	assert.Equal(t, channelIdscpy, req.Params.(map[string]interface{})["channel_ids"])
	assert.Equal(t, req.Params.(map[string]interface{})["urls"], "what")
}

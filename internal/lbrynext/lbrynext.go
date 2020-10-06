package lbrynext

import (
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/getsentry/sentry-go"
	"github.com/lbryio/lbrytv/app/query"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/test"

	"github.com/ybbus/jsonrpc"
)

var (
	resolveHookName     = "lbrynext_resolve"
	claimSearchHookName = "lbrynext_claim_search"
	logger              = monitor.NewModuleLogger("lbrynext")
	sentryURL           = "https://sentry.lbry.tech/organizations/lbry/projects/lbrytv/events/"
	fieldsToSkip        = []string{
		"activation_height",
		"expiration_height",
		"take_over_height",
		"trending_global",
		"trending_group",
		"trending_local",
		"trending_mixed",
		"reposted",
	}
)

func InstallHooks(c *query.Caller) {
	c.AddPostflightHook(query.MethodResolve, experimentNewSdkParam, resolveHookName)
}

func experimentNewSdkParam(c *query.Caller, hctx *query.HookContext) (*jsonrpc.RPCResponse, error) {
	q := hctx.Query
	hookName := resolveHookName
	if q.Method() == query.MethodClaimSearch {
		hookName = claimSearchHookName
	}
	if rand.Intn(100)+1 <= config.GetLbrynetXPercentage() {
		go func() {
			r := hctx.Response

			// This is done so the hook will not fire in a loop on repeated call
			cc := c.CloneWithoutHook(c.Endpoint(), q.Method(), hookName)

			params := q.ParamsAsMap()
			params[query.ParamNewSDKServer] = config.GetLbrynetXServer()
			q.Request.Params = params
			xr, err := cc.SendQuery(q)

			metrics.LbrynetXCallDurations.WithLabelValues(q.Method(), c.Endpoint(), metrics.GroupControl).Observe(c.Duration)
			metrics.LbrynetXCallDurations.WithLabelValues(q.Method(), cc.Endpoint(), metrics.GroupExperimental).Observe(cc.Duration)
			metrics.LbrynetXCallCounter.WithLabelValues(q.Method(), c.Endpoint(), metrics.GroupControl).Inc()
			metrics.LbrynetXCallCounter.WithLabelValues(q.Method(), cc.Endpoint(), metrics.GroupExperimental).Inc()

			log := logger.Log().WithField("method", query.MethodResolve)
			if err != nil {
				log.Error("experimental call errored:", err)
				return
			}
			rBody, xrBody, diffLog := compareResponses(r, xr)
			if diffLog != "" {
				metrics.LbrynetXCallFailedDurations.WithLabelValues(
					q.Method(), cc.Endpoint(), metrics.GroupExperimental, metrics.FailureKindLbrynetXMismatch,
				).Observe(cc.Duration)
				metrics.LbrynetXCallFailedCounter.WithLabelValues(
					q.Method(), cc.Endpoint(), metrics.GroupExperimental, metrics.FailureKindLbrynetXMismatch,
				).Inc()

				var requestStr string
				request, err := json.Marshal(q.Request)
				if err != nil {
					requestStr = fmt.Sprintf("error marshaling params: %v", err)
				} else {
					requestStr = string(request)
				}
				msg := fmt.Sprintf("experimental `%v` call result differs", q.Method())
				if config.IsProduction() {
					extra := map[string]string{
						"method":       query.MethodResolve,
						"request":      requestStr,
						"original":     rBody,
						"experimental": xrBody,
						"diff":         diffLog,
					}
					eventID := monitor.MessageToSentry(msg, sentry.LevelWarning, extra)
					log.Errorf("%v, see %v%v", msg, sentryURL, eventID)
				} else {
					log.Errorf("%v: %v", msg, diffLog)
				}
				return
			}
			log.Info("experimental call succeeded")
		}()
	}
	return nil, nil
}

func experimentParallel(c *query.Caller, hctx *query.HookContext) (*jsonrpc.RPCResponse, error) {
	q := hctx.Query
	if !q.IsAuthenticated() && rand.Intn(100)+1 <= config.GetLbrynetXPercentage() {
		r := hctx.Response
		cc := c.CloneWithoutHook(config.GetLbrynetXServer(), query.MethodResolve, resolveHookName)
		xr, err := cc.Call(q.Request)

		metrics.LbrynetXCallDurations.WithLabelValues(q.Method(), c.Endpoint(), metrics.GroupControl).Observe(c.Duration)
		metrics.LbrynetXCallDurations.WithLabelValues(q.Method(), cc.Endpoint(), metrics.GroupExperimental).Observe(cc.Duration)
		metrics.LbrynetXCallCounter.WithLabelValues(q.Method(), c.Endpoint(), metrics.GroupControl).Inc()
		metrics.LbrynetXCallCounter.WithLabelValues(q.Method(), cc.Endpoint(), metrics.GroupExperimental).Inc()

		log := logger.Log().WithField("method", query.MethodResolve)
		if err != nil {
			log.Error("experimental call errored:", err)
			return nil, nil
		}
		rBody, xrBody, diffLog := compareResponses(r, xr)
		if diffLog != "" {
			metrics.LbrynetXCallFailedDurations.WithLabelValues(
				q.Method(), cc.Endpoint(), metrics.GroupExperimental, metrics.FailureKindLbrynetXMismatch,
			).Observe(cc.Duration)
			metrics.LbrynetXCallFailedCounter.WithLabelValues(
				q.Method(), cc.Endpoint(), metrics.GroupExperimental, metrics.FailureKindLbrynetXMismatch,
			).Inc()

			msg := fmt.Sprintf("experimental `%v` call result differs", q.Method())
			if config.IsProduction() {
				extra := map[string]string{
					"method":       query.MethodResolve,
					"original":     rBody,
					"experimental": xrBody,
					"diff":         diffLog,
				}
				eventID := monitor.MessageToSentry(msg, sentry.LevelWarning, extra)
				log.Errorf("%v, see %v%v", msg, sentryURL, eventID)
			} else {
				log.Errorf("%v: %v", msg, diffLog)
			}
			return nil, nil
		}
		log.Info("experimental call succeeded")
	}
	return nil, nil
}

func skipField(f string) bool {
	for _, sf := range fieldsToSkip {
		if f == sf {
			return true
		}
	}
	return false
}

func stripFieldsFromMap(m map[string]interface{}) map[string]interface{} {
	for k, v := range m {
		if skipField(k) {
			delete(m, k)
			continue
		}
		if mm, ok := v.(map[string]interface{}); ok {
			m[k] = stripFieldsFromMap(mm)
		}
	}
	return m
}

func stripFieldsFromResponse(rsp *jsonrpc.RPCResponse) *jsonrpc.RPCResponse {
	rspMod := rsp
	if resultMap, ok := rsp.Result.(map[string]interface{}); ok {
		rspMod.Result = stripFieldsFromMap(resultMap)
	}
	return rspMod
}

func rspToByte(rsp *jsonrpc.RPCResponse) []byte {
	r, _ := json.Marshal(rsp)
	return r
}

func compareResponses(r, xr *jsonrpc.RPCResponse) (string, string, string) {
	rStripped := stripFieldsFromResponse(r)
	xrStripped := stripFieldsFromResponse(xr)
	_, diffLog := test.GetJSONDiffLog(rspToByte(rStripped), rspToByte(xrStripped))
	return string(rspToByte(r)), string(rspToByte(xr)), diffLog
}

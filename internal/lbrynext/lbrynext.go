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
)

func InstallHooks(c *query.Caller) {
	c.AddPostflightHook(query.MethodResolve, experimentNewSdkParam, resolveHookName)
	c.AddPostflightHook(query.MethodClaimSearch, experimentNewSdkParam, claimSearchHookName)
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

			log := logger.Log().WithField("method", query.MethodResolve)
			if err != nil {
				log.Error("experimental call errored:", err)
				return
			}
			_, _, diff := compareResponses(r, xr)
			if diff != "" {
				metrics.LbrynetXCallFailedDurations.WithLabelValues(
					q.Method(), cc.Endpoint(), metrics.GroupExperimental, metrics.FailureKindLbrynetXMismatch,
				).Observe(cc.Duration)

				msg := fmt.Sprintf("experimental `%v` call result differs", q.Method())
				if config.IsProduction() {
					extra := map[string]string{
						"method": query.MethodResolve,
						"diff":   diff,
					}
					eventID := monitor.MessageToSentry(msg, sentry.LevelWarning, extra)
					log.Errorf("%v, see %v%v", msg, sentryURL, eventID)
				} else {
					log.Errorf("%v: %v", msg, diff)
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

		log := logger.Log().WithField("method", query.MethodResolve)
		if err != nil {
			log.Error("experimental call errored:", err)
			return nil, nil
		}
		rBody, xrBody, diff := compareResponses(r, xr)
		if diff != "" {
			metrics.LbrynetXCallFailedDurations.WithLabelValues(
				q.Method(), cc.Endpoint(), metrics.GroupExperimental, metrics.FailureKindLbrynetXMismatch,
			).Observe(cc.Duration)

			msg := fmt.Sprintf("experimental `%v` call result differs", q.Method())
			if config.IsProduction() {
				extra := map[string]string{
					"method":       query.MethodResolve,
					"original":     rBody,
					"experimental": xrBody,
					"diff":         diff,
				}
				eventID := monitor.MessageToSentry(msg, sentry.LevelWarning, extra)
				log.Errorf("%v, see %v%v", msg, sentryURL, eventID)
			} else {
				log.Errorf("%v: %v", msg, diff)
			}
			return nil, nil
		}
		log.Info("experimental call succeeded")
	}
	return nil, nil
}

func resToByte(res *jsonrpc.RPCResponse) []byte {
	r, _ := json.Marshal(res)
	return r
}

func compareResponses(r, xr *jsonrpc.RPCResponse) (string, string, string) {
	br, bxr := resToByte(r), resToByte(xr)
	_, diffLog := test.GetJSONDiffLog(br, bxr)
	return string(br), string(bxr), diffLog
}

package lbrynext

import (
	"bytes"
	"fmt"
	"math/rand"

	"github.com/getsentry/sentry-go"
	"github.com/lbryio/lbrytv/app/query"
	"github.com/lbryio/lbrytv/apps/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/metrics"
	"github.com/lbryio/lbrytv/internal/monitor"
	"github.com/lbryio/lbrytv/internal/responses"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/ybbus/jsonrpc"
)

var (
	resolveHookName = "lbrynext_resolve"
	logger          = monitor.NewModuleLogger("lbrynext")
	sentryURL       = "https://sentry.lbry.tech/organizations/lbry/projects/lbrytv/events/"
)

func InstallHooks(c *query.Caller) {
	c.AddPostflightHook(query.MethodResolve, experimentNewSdkParam, resolveHookName)
}

func experimentNewSdkParam(c *query.Caller, hctx *query.HookContext) (*jsonrpc.RPCResponse, error) {
	q := hctx.Query
	if rand.Intn(100) <= config.GetLbrynetXPercentage() {
		go func() {
			r := hctx.Response

			// This is done so the hook will not fire in a loop on repeated call
			cc := c.CloneWithoutHook(c.Endpoint(), query.MethodResolve, resolveHookName)

			params := q.ParamsAsMap()
			params[query.ParamNewSDKServer] = config.GetLbrynetXServer()
			q.Request.Params = params
			xr, err := cc.SendQuery(q)

			metrics.LbrynextCallDurations.WithLabelValues(q.Method(), c.Endpoint(), metrics.GroupControl).Observe(c.Duration)
			metrics.LbrynextCallDurations.WithLabelValues(q.Method(), cc.Endpoint(), metrics.GroupExperimental).Observe(cc.Duration)

			log := logger.Log().WithField("method", query.MethodResolve)
			if err != nil {
				log.Error("experimental call errored:", err)
				return
			}
			rBody, xrBody, diff := compareResponses(r, xr)
			if diff != nil {
				if config.IsProduction() {
					msg := "experimental call result differs"
					extra := map[string]string{
						"method":       query.MethodResolve,
						"original":     rBody,
						"experimental": xrBody,
						"diff":         diffPlainText(diff),
					}
					eventID := monitor.MessageToSentry(msg, sentry.LevelWarning, extra)
					log.Errorf("%v, see %v%v", msg, sentryURL, eventID)
				} else {
					msg := "experimental call result differs"
					log.Errorf("%v: %v", msg, diffPlainText(diff))
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
	if !q.IsAuthenticated() && rand.Intn(100) <= config.GetLbrynetXPercentage() {
		r := hctx.Response
		cc := c.CloneWithoutHook(config.GetLbrynetXServer(), query.MethodResolve, resolveHookName)
		xr, err := cc.Call(q.Request)

		metrics.LbrynextCallDurations.WithLabelValues(q.Method(), c.Endpoint(), metrics.GroupControl).Observe(c.Duration)
		metrics.LbrynextCallDurations.WithLabelValues(q.Method(), cc.Endpoint(), metrics.GroupExperimental).Observe(cc.Duration)

		log := logger.Log().WithField("method", query.MethodResolve)
		if err != nil {
			log.Error("experimental call errored:", err)
			return nil, nil
		}
		rBody, xrBody, diff := compareResponses(r, xr)
		fmt.Println(diff)
		if diff != nil {
			if config.IsProduction() {
				msg := "experimental call result differs"
				extra := map[string]string{
					"method":       query.MethodResolve,
					"original":     rBody,
					"experimental": xrBody,
					"diff":         diffPlainText(diff),
				}
				eventID := monitor.MessageToSentry(msg, sentry.LevelWarning, extra)
				log.Errorf("%v, see %v%v", msg, sentryURL, eventID)
			} else {
				msg := "experimental call result differs"
				log.Errorf("%v: %v", msg, diffPlainText(diff))
			}
			return nil, nil
		}
		log.Info("experimental call succeeded")
	}
	return nil, nil
}

func compareResponses(r *jsonrpc.RPCResponse, xr *jsonrpc.RPCResponse) (string, string, []diffmatchpatch.Diff) {
	rBody, err := responses.JSONRPCSerialize(r)
	if err != nil {
		logger.Log().Error("original call response serialization error:", err)
		return "", "", nil
	}
	xrBody, err := responses.JSONRPCSerialize(xr)
	if err != nil {
		logger.Log().Error("experimental call response serialization error:", err)
		return "", "", nil
	}

	srBody := string(rBody)
	sxrBody := string(xrBody)

	if srBody == sxrBody {
		return srBody, sxrBody, nil
	}

	dmp := diffmatchpatch.New()
	diff := dmp.DiffMain(srBody, sxrBody, true)
	return srBody, sxrBody, diff
}

func diffPlainText(diffs []diffmatchpatch.Diff) string {
	var buff bytes.Buffer
	for _, diff := range diffs {
		text := diff.Text
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			buff.WriteString("+>>")
			buff.WriteString(text)
			buff.WriteString("<<+")
		case diffmatchpatch.DiffDelete:
			buff.WriteString("->>")
			buff.WriteString(text)
			buff.WriteString("<<-")
		case diffmatchpatch.DiffEqual:
			buff.WriteString(text)
		}
	}
	return buff.String()
}

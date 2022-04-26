package publish

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/lbryio/lbrytv/app/auth"
	"github.com/lbryio/lbrytv/app/wallet"
	"github.com/lbryio/lbrytv/internal/errors"
	"github.com/lbryio/lbrytv/internal/ip"
	"github.com/lbryio/lbrytv/models"
)

const (
	// default http retries config options
	defaultRetryWaitMin   = 5 * time.Second
	defaultRetryWaitMax   = 30 * time.Second
	defaultRequestTimeout = 600 * time.Second
	defaultRetryMax       = 3

	fetchTryLimit   = 5
	fetchTimeout    = 15 * time.Minute
	fetchRetryDelay = time.Second
)

var TusHeaders = []string{
	"Http-Method-Override",
	"Upload-Length",
	"Upload-Offset",
	"Tus-Resumable",
	"Upload-Metadata",
	"Upload-Defer-Length",
	"Upload-Concat",
}

// copied from hashicorp/go-retryablehttp/client.go
var (
	// A regular expression to match the error returned by net/http when the
	// configured number of redirects is exhausted. This error isn't typed
	// specifically so we resort to matching on the error string.
	redirectsErrorRe = regexp.MustCompile(`stopped after \d+ redirects\z`)

	// A regular expression to match the error returned by net/http when the
	// scheme specified in the URL is invalid. This error isn't typed
	// specifically so we resort to matching on the error string.
	schemeErrorRe = regexp.MustCompile(`unsupported protocol scheme`)

	// A regular expression to match the error returned by net/http when the
	// it failed to resolve the host address. The error will be wrapped with
	// the net/url error and another error, so for convenience we resort to
	// match the error string instead.
	lookupHostErrorRe = regexp.MustCompile(`dial tcp: lookup`)
)

// retryPolicy is the same as retryablehttp.DefaultRetryPolicy
// except we log the retryable error for more verbosity and only retry
// when connections error occured.
func retryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	// do not retry on context.Canceled or context.DeadlineExceeded
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	// check for http request error
	if err != nil {
		if v, ok := err.(*url.Error); ok {
			// Don't retry if the error was due to too many redirects.
			if redirectsErrorRe.MatchString(v.Error()) {
				return false, v
			}

			// Don't retry if the error was due to an invalid protocol scheme.
			if schemeErrorRe.MatchString(v.Error()) {
				return false, v
			}

			// Don't retry if the error was due to TLS cert verification failure.
			if _, ok := v.Err.(x509.UnknownAuthorityError); ok {
				return false, v
			}

			// Don't retry if the error was due to failure on host
			// resolution.
			if lookupHostErrorRe.MatchString(v.Error()) {
				return false, v
			}
		}

		// The error is likely recoverable so retry, but log the error
		// anyway to get more info if unexpected errors causing issues
		// in the future.
		logger.Log().WithError(err).Warn("retrying http request")

		return true, nil
	}

	// request is success, check the response code.
	// We retry on 500-range responses to allow the server time to recover,
	// as 500's are typically not permanent errors and may relate to outages
	//  on the server side.
	//
	// This will catch invalid response codes as well, like 0 and 999.
	if resp.StatusCode == 0 || (resp.StatusCode >= 500 && resp.StatusCode != 501) {
		err := fmt.Errorf("unexpected HTTP status code %d", resp.StatusCode)
		logger.Log().Error(err)
		return true, err
	}

	return false, nil
}

// userFromRequest attempts to validate the request and return authorized user
// if the request contains valid user access token.
func userFromRequest(provider auth.Provider, headers http.Header, remoteAddr string) (*models.User, error) {
	if token, ok := headers[wallet.LegacyTokenHeader]; ok {
		addr := ip.AddressForRequest(headers, remoteAddr)
		return provider(token[0], addr)
	}
	return nil, errors.Err(auth.ErrNoAuthInfo)
}

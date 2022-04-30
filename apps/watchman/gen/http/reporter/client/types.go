// Code generated by goa v3.5.2, DO NOT EDIT.
//
// reporter HTTP client types
//
// Command:
// $ goa gen github.com/lbryio/lbrytv/apps/watchman/design -o apps/watchman

package client

import (
	reporter "github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	goa "goa.design/goa/v3/pkg"
)

// AddRequestBody is the type of the "reporter" service "add" endpoint HTTP
// request body.
type AddRequestBody struct {
	// LBRY URL (lbry://... without the protocol part)
	URL string `form:"url" json:"url" xml:"url"`
	// Duration of time between event calls in ms (aiming for between 5s and 30s so
	// generally 5000–30000)
	Duration int32 `form:"duration" json:"duration" xml:"duration"`
	// Current playback report stream position, ms
	Position int32 `form:"position" json:"position" xml:"position"`
	// Relative stream position, pct, 0—100
	RelPosition int32 `form:"rel_position" json:"rel_position" xml:"rel_position"`
	// Rebuffering events count during the interval
	RebufCount int32 `form:"rebuf_count" json:"rebuf_count" xml:"rebuf_count"`
	// Sum of total rebuffering events duration in the interval, ms
	RebufDuration int32 `form:"rebuf_duration" json:"rebuf_duration" xml:"rebuf_duration"`
	// Standard binary stream (`stb`), HLS (`hls`) or live stream (`lvs`)
	Protocol string `form:"protocol" json:"protocol" xml:"protocol"`
	// Cache status of video
	Cache *string `form:"cache,omitempty" json:"cache,omitempty" xml:"cache,omitempty"`
	// Player server name
	Player string `form:"player" json:"player" xml:"player"`
	// User ID
	UserID string `form:"user_id" json:"user_id" xml:"user_id"`
	// Client bandwidth, bit/s
	Bandwidth *int32 `form:"bandwidth,omitempty" json:"bandwidth,omitempty" xml:"bandwidth,omitempty"`
	// Media bitrate, bit/s
	Bitrate *int32 `form:"bitrate,omitempty" json:"bitrate,omitempty" xml:"bitrate,omitempty"`
	// Client device
	Device string `form:"device" json:"device" xml:"device"`
}

// AddMultiFieldErrorResponseBody is the type of the "reporter" service "add"
// endpoint HTTP response body for the "multi_field_error" error.
type AddMultiFieldErrorResponseBody struct {
	Message *string `form:"message,omitempty" json:"message,omitempty" xml:"message,omitempty"`
}

// NewAddRequestBody builds the HTTP request body from the payload of the "add"
// endpoint of the "reporter" service.
func NewAddRequestBody(p *reporter.PlaybackReport) *AddRequestBody {
	body := &AddRequestBody{
		URL:           p.URL,
		Duration:      p.Duration,
		Position:      p.Position,
		RelPosition:   p.RelPosition,
		RebufCount:    p.RebufCount,
		RebufDuration: p.RebufDuration,
		Protocol:      p.Protocol,
		Cache:         p.Cache,
		Player:        p.Player,
		UserID:        p.UserID,
		Bandwidth:     p.Bandwidth,
		Bitrate:       p.Bitrate,
		Device:        p.Device,
	}
	return body
}

// NewAddMultiFieldError builds a reporter service add endpoint
// multi_field_error error.
func NewAddMultiFieldError(body *AddMultiFieldErrorResponseBody) *reporter.MultiFieldError {
	v := &reporter.MultiFieldError{
		Message: *body.Message,
	}

	return v
}

// ValidateAddMultiFieldErrorResponseBody runs the validations defined on
// add_multi_field_error_response_body
func ValidateAddMultiFieldErrorResponseBody(body *AddMultiFieldErrorResponseBody) (err error) {
	if body.Message == nil {
		err = goa.MergeErrors(err, goa.MissingFieldError("message", "body"))
	}
	return
}

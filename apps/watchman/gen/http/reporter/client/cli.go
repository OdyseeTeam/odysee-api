// Code generated by goa v3.5.2, DO NOT EDIT.
//
// reporter HTTP client CLI support package
//
// Command:
// $ goa gen github.com/lbryio/lbrytv/apps/watchman/design -o apps/watchman

package client

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	reporter "github.com/lbryio/lbrytv/apps/watchman/gen/reporter"
	goa "goa.design/goa/v3/pkg"
)

// BuildAddPayload builds the payload for the reporter add endpoint from CLI
// flags.
func BuildAddPayload(reporterAddBody string) (*reporter.PlaybackReport, error) {
	var err error
	var body AddRequestBody
	{
		err = json.Unmarshal([]byte(reporterAddBody), &body)
		if err != nil {
			return nil, fmt.Errorf("invalid JSON for body, \nerror: %s, \nexample of valid JSON:\n%s", err, "'{\n      \"bandwidth\": 64944106,\n      \"bitrate\": 13952061,\n      \"cache\": \"miss\",\n      \"device\": \"ios\",\n      \"duration\": 30000,\n      \"player\": \"sg-p2\",\n      \"position\": 1045058586,\n      \"protocol\": \"lvs\",\n      \"rebuf_count\": 2095695930,\n      \"rebuf_duration\": 38439,\n      \"rel_position\": 13,\n      \"url\": \"@veritasium#f/driverless-cars-are-already-here#1\",\n      \"user_id\": \"432521\"\n   }'")
		}
		if utf8.RuneCountInString(body.URL) > 512 {
			err = goa.MergeErrors(err, goa.InvalidLengthError("body.url", body.URL, utf8.RuneCountInString(body.URL), 512, false))
		}
		if body.Duration < 0 {
			err = goa.MergeErrors(err, goa.InvalidRangeError("body.duration", body.Duration, 0, true))
		}
		if body.Duration > 60000 {
			err = goa.MergeErrors(err, goa.InvalidRangeError("body.duration", body.Duration, 60000, false))
		}
		if body.Position < 0 {
			err = goa.MergeErrors(err, goa.InvalidRangeError("body.position", body.Position, 0, true))
		}
		if body.RelPosition < 0 {
			err = goa.MergeErrors(err, goa.InvalidRangeError("body.rel_position", body.RelPosition, 0, true))
		}
		if body.RelPosition > 100 {
			err = goa.MergeErrors(err, goa.InvalidRangeError("body.rel_position", body.RelPosition, 100, false))
		}
		if body.RebufCount < 0 {
			err = goa.MergeErrors(err, goa.InvalidRangeError("body.rebuf_count", body.RebufCount, 0, true))
		}
		if body.RebufDuration < 0 {
			err = goa.MergeErrors(err, goa.InvalidRangeError("body.rebuf_duration", body.RebufDuration, 0, true))
		}
		if body.RebufDuration > 60000 {
			err = goa.MergeErrors(err, goa.InvalidRangeError("body.rebuf_duration", body.RebufDuration, 60000, false))
		}
		if !(body.Protocol == "stb" || body.Protocol == "hls" || body.Protocol == "lvs") {
			err = goa.MergeErrors(err, goa.InvalidEnumValueError("body.protocol", body.Protocol, []interface{}{"stb", "hls", "lvs"}))
		}
		if body.Cache != nil {
			if !(*body.Cache == "local" || *body.Cache == "player" || *body.Cache == "miss") {
				err = goa.MergeErrors(err, goa.InvalidEnumValueError("body.cache", *body.Cache, []interface{}{"local", "player", "miss"}))
			}
		}
		if utf8.RuneCountInString(body.Player) > 64 {
			err = goa.MergeErrors(err, goa.InvalidLengthError("body.player", body.Player, utf8.RuneCountInString(body.Player), 64, false))
		}
		if utf8.RuneCountInString(body.UserID) < 1 {
			err = goa.MergeErrors(err, goa.InvalidLengthError("body.user_id", body.UserID, utf8.RuneCountInString(body.UserID), 1, true))
		}
		if utf8.RuneCountInString(body.UserID) > 45 {
			err = goa.MergeErrors(err, goa.InvalidLengthError("body.user_id", body.UserID, utf8.RuneCountInString(body.UserID), 45, false))
		}
		if !(body.Device == "ios" || body.Device == "adr" || body.Device == "web" || body.Device == "dsk" || body.Device == "stb") {
			err = goa.MergeErrors(err, goa.InvalidEnumValueError("body.device", body.Device, []interface{}{"ios", "adr", "web", "dsk", "stb"}))
		}
		if err != nil {
			return nil, err
		}
	}
	v := &reporter.PlaybackReport{
		URL:           body.URL,
		Duration:      body.Duration,
		Position:      body.Position,
		RelPosition:   body.RelPosition,
		RebufCount:    body.RebufCount,
		RebufDuration: body.RebufDuration,
		Protocol:      body.Protocol,
		Cache:         body.Cache,
		Player:        body.Player,
		UserID:        body.UserID,
		Bandwidth:     body.Bandwidth,
		Bitrate:       body.Bitrate,
		Device:        body.Device,
	}

	return v, nil
}

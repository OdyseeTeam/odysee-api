package design

import (
	"net/http"

	. "goa.design/goa/v3/dsl"
	cors "goa.design/plugins/v3/cors/dsl"
)

var _ = API("watchman", func() {
	Title("Watchman service")
	Description(`Watchman collects media playback reports.
		Playback time along with buffering count and duration is collected
		via playback reports, which should be sent from the client each n sec
		(with n being something reasonable between 5 and 30s)
	`)

	cors.Origin(`/(http:\/\/localhost:\d+)|(https:\/\/odysee.com)|(https:\/\/.+\.odysee.com)/`, func() {
		cors.Methods(http.MethodGet, http.MethodPost)
		cors.MaxAge(600)
	})

	Server("watchman", func() {
		Description("watchman hosts the Watchman service")

		Services("reporter")

		Host("production", func() {
			Description("Production host")
			URI("https://watchman.na-backend.odysee.com")
		})
		Host("dev", func() {
			Description("Development host")
			URI("https://watchman.na-backend.dev.odysee.com")
		})
	})
})

var _ = Service("reporter", func() {
	Description("Media playback reports")

	Method("add", func() {
		Payload(PlaybackReport)
		Result(Empty)
		HTTP(func() {
			POST("/reports/playback")
			Response(StatusCreated)
		})
	})
	Method("healthz", func() {
		Result(String)
		HTTP(func() {
			GET("/healthz")
			Response(StatusOK)
		})
	})
	Files("/openapi.json", "./gen/http/openapi.json")
})

var PlaybackReport = Type("PlaybackReport", func() {
	Attribute("url", String, "LBRY URL", func() {
		Example("what")
		MaxLength(512)
	})
	Attribute("duration", Int32, "Event duration, ms", func() {
		Minimum(0)
		Maximum(60000)
	})
	Attribute("position", Int32, "Current playback report stream position, ms", func() {
		Minimum(0)
	})
	Attribute("rel_position", Int32, "Relative stream position, pct, 0—100", func() {
		Minimum(0)
		Maximum(100)
	})

	Attribute("rebuf_count", Int32, "Rebuffering events count", func() {
		Minimum(0)
	})
	Attribute("rebuf_duration", Int32, "Rebuffering events duration, ms", func() {
		Minimum(0)
		Maximum(60000)
	})

	Attribute("protocol", String, "Video delivery protocol, stb (binary stream) or HLS", func() {
		Enum("stb", "hls")
	})

	Attribute("cache", String, "Cache status of video", func() {
		Enum("local", "player", "miss")
	})

	Attribute("player", String, "Player server name", func() {
		Example("sg-p2")
		MaxLength(64)
	})

	Attribute("user_id", Int32, "User ID")
	Attribute("bandwidth", Int32, "Client bandwidth, bit/s")
	Attribute("device", String, "Client device", func() {
		Enum("ios", "adr", "web")
	})

	Required(
		"url", "duration", "position", "rel_position", "rebuf_count", "rebuf_duration", "protocol",
		"player", "user_id", "device")
})

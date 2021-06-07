package design

import . "goa.design/goa/v3/dsl"

var _ = API("watchman", func() {
	Title("Watchman service")
	Description(`Watchman collects media playback reports.
		Playback time along with buffering count and duration is collected
		via playback reports, which should be sent from the client each n sec
		(with n being something reasonable between 5 and 30s)
	`)

	Server("watchman", func() {
		Description("watchman hosts the Watchman service")

		Services("reporter")

		Host("production", func() {
			Description("Production host")
			URI("https://watchman.na-backend.odysee.com")

			Variable("version", String, "API version", func() {
				Default("v1")
			})
		})
		Host("dev", func() {
			Description("Development host")
			URI("https://watchman.dev.na-backend.odysee.com")

			Variable("version", String, "API version", func() {
				Default("v1")
			})
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
	Attribute("position", Int32, "Current playback report stream position, ms", func() {
		Minimum(0)
	})
	Attribute("rel_position", Int32, "Relative stream position, pct, 0—100", func() {
		Minimum(0)
		Maximum(100)
	})

	Attribute("buf_count", Int32, "Buffering events count", func() {
		Minimum(0)
	})
	Attribute("buf_duration", Int32, "Buffering events total duration, ms")

	Attribute("format", String, "Video format", func() {
		Enum("std", "hls")
	})

	Attribute("player", String, "Player server name", func() {
		Example("sg-p2")
		MaxLength(64)
	})

	Attribute("client", String, "Unique client ID", func() {
		Example("b026324c6904b2a9cb4b88d6d61c81d1")
		MaxLength(64)
	})
	Attribute("client_rate", Int32, "Client download rate, bit/s")
	Attribute("device", String, "Client device", func() {
		Enum("ios", "adr", "web")
	})
	Attribute("t", String, "Timestamp", func() {
		Format(FormatRFC1123)
	})

	Required("url", "position", "rel_position", "buf_count", "buf_duration", "format", "player", "client", "device")
})

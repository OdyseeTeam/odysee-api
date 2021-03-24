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
			URI("https://watchman.api.lbry.tv/{version}")
			URI("https://watchman.api.odysee.tv/{version}")

			Variable("version", String, "API version", func() {
				Default("v1")
			})
		})
		Host("dev", func() {
			Description("Development host")
			URI("https://watchman-service.api.dev.lbry.tv")
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
	Files("/openapi.json", "./gen/http/openapi.json")
})

var PlaybackReport = Type("PlaybackReport", func() {
	Attribute("url", String, "LBRY URL", func() {
		Example("lbry://what")
		MaxLength(512)
	})
	Attribute("pos", Int32, "Current playback report stream position, ms", func() {
		Minimum(0)
	})
	Attribute("por", Int32, "Relative stream position, 0 — 10000 (0% — 100%)", func() {
		Minimum(0)
		Maximum(10000)
	})
	Attribute("dur", Int32, "Current playback report duration, ms", func() {
		Minimum(1000)
		Maximum(3_600_000)
	})

	Attribute("bfc", Int32, "Buffering events count", func() {
		Minimum(0)
	})
	Attribute("bfd", Int32, "Buffering events total duration, ms")

	Attribute("fmt", String, "Video format", func() {
		Enum("std", "hls")
	})

	Attribute("pid", String, "Player server name", func() {
		Example("player16")
		MaxLength(32)
	})

	Attribute("crt", Int32, "Client download rate, bits/s")
	Attribute("car", String, "Client area", func() {
		Example("Europe", "eu")
		MaxLength(3)
	})
	Attribute("cid", String, "Unique client ID", func() {
		Example("b026324c6904b2a9cb4b88d6d61c81d1")
		MaxLength(64)
	})
	Attribute("cdv", String, "Client device", func() {
		Enum("ios", "and", "web")
	})

	Required("url", "pos", "por", "dur", "bfc", "bfd", "fmt", "pid", "cid", "cdv")
})

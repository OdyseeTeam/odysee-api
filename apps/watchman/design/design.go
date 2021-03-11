package design

import . "goa.design/goa/v3/dsl"

var _ = API("watchman", func() {
	Title("Watchman service")
	Description(`Watchman collects playback metrics.
		Playback time along with buffering count and duration is collected
		via playback events, which should be sent from the client each n sec
		(with n being something reasonable between 5 and 30s)
	`)

	Server("watchman", func() {
		Description("watchman hosts the Watchman service")

		Services("playback")

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

var _ = Service("playback", func() {
	Description("Video playback events receptacle")
	Method("add", func() {
		Payload(Playback)
		Result(Empty)
		HTTP(func() {
			POST("/playback")
			Response(StatusCreated)
		})
	})
	Files("/openapi.json", "./gen/http/openapi.json")
})

var Playback = Type("Playback", func() {
	Attribute("url", String, "LBRY URL", func() {
		Example("lbry://what")
		MaxLength(512)
	})
	Attribute("pos", UInt64, "Playback event stream position, ms")
	Attribute("dur", UInt32, "Playback event duration, ms", func() {
		Minimum(1000)
		Maximum(3600_000)
	})

	Attribute("bfc", UInt32, "Buffering events count")
	Attribute("bfd", UInt64, "Buffering events total duration, ms")

	Attribute("fmt", String, "Video format", func() {
		Enum("def", "hls")
	})

	Attribute("pid", String, "Player server name", func() {
		Example("player16")
		MaxLength(32)
	})

	Attribute("crt", UInt64, "Client download rate, bits/s")
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

	Required("url", "pos", "dur", "bfc", "bfd", "fmt", "pid", "cid", "cdv")
})

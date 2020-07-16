package collector

type EventBuffering struct {
	URL            string `json:"url"`
	Position       int    `json:"position"`
	Duration       *int   `json:"duration,omitempty"`
	StreamDuration *int   `json:"stream_duration,omitempty"`
	StreamBitrate  *int   `json:"stream_bitrate,omitempty"`
}

type PlaybackStart struct {
	// media bitrate, bits/sec (if known)
	StreamBitrate *int `json:"stream_bitrate,omitempty"`
	// stream duration, seconds
	StreamDuration *int `json:"stream_duration,omitempty"`
	// time before playback started, milliseconds
	TimeToStart int `json:"time_to_start"`
	// LBRY content URL
	URL string `json:"url"`
}

package player

import (
	"errors"
)

var (
	errPaidStream         = errors.New("paid stream")
	errSeekingBeforeStart = errors.New("seeking before the beginning of file")
	errOutOfBounds        = errors.New("seeking out of bounds")
	errMissingBlob        = errors.New("blob missing")
	errStreamNotFound     = errors.New("could not resolve stream URI")
	errStreamSizeZero     = errors.New("stream size is zero")
)

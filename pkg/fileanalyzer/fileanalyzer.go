package fileanalyzer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"

	"github.com/h2non/filetype"
	"gopkg.in/vansante/go-ffprobe.v2"
)

type Analyzer struct {
	ffprobePath string
}

type StreamInfo struct {
	header       []byte
	RealFilePath string
	FileName     string
	MediaInfo    *MediaInfo
	MediaType    *MediaType
}

type MediaInfo struct {
	Duration int
	Width    int
	Height   int
}

type MediaType struct {
	MIME, Name, Extension string
}

func NewAnalyzer() (*Analyzer, error) {
	return &Analyzer{}, nil
}

func (a *Analyzer) Analyze(ctx context.Context, realFilePath, fileName string) (*StreamInfo, error) {
	if fileName == "" {
		fileName = path.Base(realFilePath)
	}
	s := &StreamInfo{
		FileName:     fileName,
		RealFilePath: realFilePath,
	}
	header := make([]byte, 261)
	file, err := os.Open(realFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = file.Read(header)
	if err != nil {
		return nil, err
	}
	s.header = header

	err = s.DetectMediaType()
	if err != nil {
		return nil, err
	}
	err = s.DetectMediaInfo(ctx)
	return s, err
}

// DetectMediaType attempts to detect the media type based on file header
// or file extension as a fallback.
func (si *StreamInfo) DetectMediaType() error {
	var fileExt, detExt string
	fileExt = path.Ext(si.FileName)

	kind, _ := filetype.Match(si.header)
	if kind == filetype.Unknown {
		detExt = fileExt
	} else {
		detExt = kind.Extension
	}

	var foundSyn bool
	if detExt != fileExt {
		for _, x := range synonyms[detExt] {
			if fileExt == x {
				foundSyn = true
			}
		}
		if !foundSyn {
			fileExt = detExt
		}
	}

	fileExt = "." + fileExt
	t, ok := extensions[fileExt]
	if !ok {
		si.MediaType = &defaultType
	} else {
		if t.Extension == "" {
			t.Extension = fileExt
		}
		si.MediaType = &t
	}
	return nil
}

// DetectMediaInfo attempts to read video stream metadata and saves a set of attributes
// for use in SDK stream_create calls.
func (si *StreamInfo) DetectMediaInfo(ctx context.Context) error {
	if si.MediaType == nil {
		return errors.New("DetectMediaType must be called first")
	}
	if si.MediaType.Name != "video" && si.MediaType.Name != "image" && si.MediaType.Name != "audio" {
		return fmt.Errorf("no media info for '%s' type", si.MediaType.Name)
	}
	data, err := ffprobe.ProbeURL(ctx, si.RealFilePath)
	if err != nil {
		return fmt.Errorf("error running ffprobe: %w", err)
	}
	if data.Format == nil || len(data.Streams) == 0 {
		return errors.New("format data is missing from ffprobe results")
	}

	info := &MediaInfo{}
	var (
		stream         *ffprobe.Stream
		needStreamType string
	)
	if si.MediaType.Name == "image" {
		needStreamType = "video"
	} else {
		needStreamType = si.MediaType.Name
	}
	for _, s := range data.Streams {
		if s.CodecType == needStreamType {
			stream = s
			break
		}
	}
	if stream == nil {
		return nil
	}

	info.Duration = int(data.Format.Duration().Seconds())
	info.Width = stream.Width
	info.Height = stream.Height
	si.MediaInfo = info

	return nil
}

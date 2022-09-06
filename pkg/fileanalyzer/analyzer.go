package fileanalyzer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/h2non/filetype"
	"gopkg.in/vansante/go-ffprobe.v2"
)

type Analyzer struct {
	ffprobePath string
}

type Analyzed struct {
	filePath  string
	header    []byte
	MediaInfo *MediaInfo
	MediaType *MediaType
}

type MediaInfo struct {
	Duration      int
	Width, Height int
}

func NewAnalyzer() (*Analyzer, error) {
	return &Analyzer{}, nil
}

func (a *Analyzer) Analyze(ctx context.Context, filePath string) (*Analyzed, error) {
	d := &Analyzed{filePath: filePath}
	header := make([]byte, 261)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = file.Read(header)
	if err != nil {
		return nil, err
	}
	d.header = header

	err = d.GetMediaType()
	if err != nil {
		return nil, err
	}
	err = d.GetMediaInfo(ctx)
	return d, err
}

// GetMediaInfo attempts to read video stream metadata and saves a set of attributes
// for use in SDK stream_create calls.
func (d *Analyzed) GetMediaInfo(ctx context.Context) error {
	if d.MediaType == nil {
		return errors.New("GetMediaType must be called first")
	}
	if d.MediaType.Name != "video" && d.MediaType.Name != "image" && d.MediaType.Name != "audio" {
		return fmt.Errorf("no media info for '%s' type", d.MediaType.Name)
	}
	f, err := os.Open(d.filePath)
	if err != nil {
		return err
	}
	data, err := ffprobe.ProbeReader(ctx, f)
	if err != nil {
		return fmt.Errorf("error running ffprobe: %w", err)
	}
	if data.Format == nil || len(data.Streams) == 0 {
		return errors.New("format data is missing from ffprobe results")
	}

	m := &MediaInfo{}
	s := data.Streams[0]
	if data.Format.DurationSeconds > 0 {
		m.Duration = int(data.Format.DurationSeconds)
	} else {
		d, _ := strconv.ParseFloat(s.Duration, 32)
		m.Duration = int(d)
	}
	m.Width = s.Width
	m.Height = s.Height
	d.MediaInfo = m

	return nil
}

func (d *Analyzed) GetMediaType() error {
	var fileExt, detExt string
	fileExt = path.Ext(d.filePath)

	kind, _ := filetype.Match(d.header)
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
		d.MediaType = &MediaType{
			MIME: fmt.Sprintf("application/x-ext-%s", strings.TrimPrefix(fileExt, ".")),
			Name: "binary",
		}
	} else {
		d.MediaType = &t
	}
	return nil
}

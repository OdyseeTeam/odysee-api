package fileanalyzer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

const testAssetsURL = "https://odytestassets.s3.us-east-2.amazonaws.com/"

type analyzerSuite struct {
	suite.Suite
}

func TestFileHandlingSuite(t *testing.T) {
	suite.Run(t, new(analyzerSuite))
}

func (s *analyzerSuite) SetupSuite() {
	os.Setenv("PATH", os.Getenv("PATH")+":/opt/homebrew/bin")
}

func (s *analyzerSuite) TestAnalyze() {
	cases := []struct {
		kind, file              string
		meta                    *MediaInfo
		getMediaInfoError       error
		mimeName, mimeType, ext string
	}{
		{
			kind: "MP3 audio",
			file: "96k",
			meta: &MediaInfo{
				Duration: 45,
				Width:    0,
				Height:   0,
			},
			mimeName: "audio",
			mimeType: "audio/mpeg",
			ext:      ".mp3",
		},
		{
			kind: "MOV video",
			file: "hdreel.mov",
			meta: &MediaInfo{
				Duration: 29,
				Width:    1920,
				Height:   1080,
			},
			mimeName: "video",
			mimeType: "video/quicktime",
			ext:      ".avi",
		},
		{
			kind: "AVI video",
			file: "sample_960x400_ocean_with_audio.avi",
			meta: &MediaInfo{
				Duration: 46,
				Width:    960,
				Height:   400,
			},
			mimeName: "video",
			mimeType: "video/x-msvideo",
			ext:      ".avi",
		},
		{
			kind: "JPEG",
			file: "Moeraki-Boulders-New-Zealand.jpg",
			meta: &MediaInfo{
				Width:  2048,
				Height: 1365,
			},
			mimeName: "image",
			mimeType: "image/jpeg",
			ext:      ".jpg",
		},
		{
			kind:              "DOC",
			file:              "sample2.doc",
			getMediaInfoError: errors.New("no media info for 'document' type"),
			mimeName:          "document",
			ext:               ".doc",
		},
		{
			kind:              "binary file",
			file:              "test1Mb.db",
			getMediaInfoError: errors.New("no media info for 'binary' type"),
			mimeName:          "binary",
			mimeType:          "application/octet-stream",
			ext:               ".bin",
		},
	}

	for _, c := range cases {
		s.Run(c.kind, func() {
			filePath := s.getTestAsset(c.file)
			f, err := os.Open(filePath)
			s.Require().NoError(err)
			defer f.Close()

			a, err := NewAnalyzer()
			s.Require().NoError(err)

			d, err := a.Analyze(context.Background(), filePath, "")
			s.Require().Equal(c.getMediaInfoError, err)
			if c.mimeType != "" {
				s.Equal(c.mimeType, d.MediaType.MIME)
			}
			s.Equal(c.mimeName, d.MediaType.Name)
			if c.meta != nil {
				s.Equal(c.meta, d.MediaInfo)
			}
		})
	}
}

func (s *analyzerSuite) getTestAsset(file string) string {
	r, err := http.Get(testAssetsURL + file)
	s.Require().NoError(err)
	defer r.Body.Close()
	s.Require().Equal(http.StatusOK, r.StatusCode)
	f, err := os.CreateTemp(s.T().TempDir(), "")
	s.Require().NoError(err)
	defer f.Close()
	_, err = io.Copy(f, r.Body)
	s.Require().NoError(err)
	return f.Name()
}

package fileanalyzer

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

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
		kind, url               string
		videoMeta               *MediaInfo
		getMediaInfoError       error
		mimeName, mimeType, ext string
	}{
		{
			kind:     "audio",
			url:      "https://getsamplefiles.com/download/mp3/96k",
			mimeName: "audio",
			mimeType: "audio/mpeg",
			ext:      ".mp3",
		},
		{
			kind: "video",
			url:  "https://filesamples.com/samples/video/avi/sample_960x400_ocean_with_audio.avi",
			videoMeta: &MediaInfo{
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
			url:  "https://filesamples.com/samples/image/jpeg/sample_1280×853.jpeg",
			videoMeta: &MediaInfo{
				Width:  1280,
				Height: 853,
			},
			mimeName: "image",
			mimeType: "image/jpeg",
			ext:      ".jpg",
		},
		{
			kind:              "doc",
			url:               "https://filesamples.com/samples/document/doc/sample2.doc",
			getMediaInfoError: errors.New("no media info for 'document' type"),
			mimeName:          "document",
			ext:               ".doc",
		},
	}

	for _, c := range cases {
		s.Run(c.kind, func() {
			filePath := s.getTestAsset(c.url)
			f, err := os.Open(filePath)
			s.Require().NoError(err)
			defer f.Close()

			a, err := NewAnalyzer()
			s.Require().NoError(err)

			d, err := a.Analyze(context.Background(), filePath)
			s.Require().Equal(c.getMediaInfoError, err)
			if c.mimeType != "" {
				s.Equal(c.mimeType, d.MediaType.MIME)
			}
			s.Equal(c.mimeName, d.MediaType.Name)
			s.Equal(c.videoMeta, d.MediaInfo)
		})
	}
}

func (s *analyzerSuite) getTestAsset(url string) string {
	r, err := http.Get(url)
	s.Require().NoError(err)
	defer r.Body.Close()
	s.Require().Equal(http.StatusOK, r.StatusCode)
	f, err := ioutil.TempFile(s.T().TempDir(), "")
	s.Require().NoError(err)
	defer f.Close()
	_, err = io.Copy(f, r.Body)
	s.Require().NoError(err)
	return f.Name()
}

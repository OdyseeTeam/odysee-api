package forklift

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/e2etest"
	"github.com/OdyseeTeam/odysee-api/internal/test"

	"github.com/stretchr/testify/suite"
	"github.com/ybbus/jsonrpc"
)

type carriageSuite struct {
	suite.Suite
	userHelper     *e2etest.UserTestHelper
	forkliftHelper *ForkliftTestHelper
}

func TestCarriageSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}
	suite.Run(t, new(carriageSuite))
}

func (s *carriageSuite) TestProcessReel() {
	c, err := NewCarriage(s.T().TempDir(), nil, config.GetReflectorUpstream(), nil)
	s.Require().NoError(err)

	ts := time.Now().Format("2006-01-02-15-04-05-000")
	claimName := fmt.Sprintf("publishv3testreel-%s", ts)

	for i := 0; i <= 1; i++ {
		p := UploadProcessPayload{
			UploadID: "",
			UserID:   s.userHelper.UserID(),
			Path:     test.StaticAsset(s.T(), "hdreel.mov"),
			Request: jsonrpc.NewRequest(query.MethodStreamCreate, map[string]interface{}{
				"name":                 claimName,
				"title":                fmt.Sprintf("Publish v3 Test: Reel %s", ts),
				"description":          "",
				"locations":            []string{},
				"bid":                  "0.001",
				"languages":            []string{"en"},
				"tags":                 []string{"c:disable-comments"},
				"thumbnail_url":        "https://thumbs.odycdn.com/92399dc6df41af6f7c61def97335dfa5.webp",
				"release_time":         time.Now().Unix(),
				"blocking":             true,
				"preview":              false,
				"license":              "None",
				"channel_id":           "febc557fcfbe5c1813eb621f7d38a80bc4355085",
				"file_path":            "__POST_FILE__",
				"allow_duplicate_name": true,
			}),
		}

		res, err := c.Process(p)
		s.Require().NoError(err)

		pp := p.Request.Params.(map[string]interface{})
		s.EqualValues(17809516, pp["file_size"])
		s.EqualValues(1920, pp["width"])
		s.EqualValues(1080, pp["height"])
		s.EqualValues(29, pp["duration"])

		scr := StreamCreateResponse{}
		rr, err := json.Marshal(res.Response.Result)
		s.Require().NoError(err)
		err = json.Unmarshal(rr, &scr)
		s.Require().NoError(err, fmt.Sprintf("RESPONSE: %+v", scr))

		s.Equal("video/quicktime", scr.Outputs[0].Value.Source.MediaType)
		s.Equal("hdreel.mov", scr.Outputs[0].Value.Source.Name)
		s.Equal(claimName, scr.Outputs[0].Name)
		s.EqualValues(strconv.Itoa(17809516), scr.Outputs[0].Value.Source.Size)
		s.Equal(res.SDHash, scr.Outputs[0].Value.Source.SdHash)
		fmt.Printf("%+v\n", scr.Outputs[0])

		s.NoFileExists(c.blobsPath, res.SDHash)
	}
}

func (s *carriageSuite) TestProcessReelWithNotTheBestName() {
	c, err := NewCarriage(s.T().TempDir(), nil, config.GetReflectorUpstream(), nil)
	s.Require().NoError(err)

	ts := time.Now().Format("2006-01-02-15-04-05-000")
	claimName := fmt.Sprintf("publishv3testreel-%s", ts)
	ap := test.StaticAsset(s.T(), "hdreel.mov")
	np := path.Join(path.Dir(ap), "forkingname")
	s.Require().NoError(os.Rename(ap, np))

	for i := 0; i <= 1; i++ {
		p := UploadProcessPayload{
			UploadID: "",
			UserID:   s.userHelper.UserID(),
			Path:     np,
			Request: jsonrpc.NewRequest(query.MethodStreamCreate, map[string]interface{}{
				"name":                 claimName,
				"title":                fmt.Sprintf("Publish v3 Test: Reel %s", ts),
				"description":          "",
				"locations":            []string{},
				"bid":                  "0.001",
				"languages":            []string{"en"},
				"tags":                 []string{"c:disable-comments"},
				"thumbnail_url":        "https://thumbs.odycdn.com/92399dc6df41af6f7c61def97335dfa5.webp",
				"release_time":         time.Now().Unix(),
				"blocking":             true,
				"preview":              false,
				"license":              "None",
				"channel_id":           "febc557fcfbe5c1813eb621f7d38a80bc4355085",
				"file_path":            "__POST_FILE__",
				"allow_duplicate_name": true,
			}),
		}

		res, err := c.Process(p)
		s.Require().NoError(err)

		pp := p.Request.Params.(map[string]interface{})
		s.EqualValues(17809516, pp["file_size"])
		s.EqualValues(1920, pp["width"])
		s.EqualValues(1080, pp["height"])
		s.EqualValues(29, pp["duration"])

		scr := StreamCreateResponse{}
		rr, err := json.Marshal(res.Response.Result)
		s.Require().NoError(err)
		err = json.Unmarshal(rr, &scr)
		s.Require().NoError(err, fmt.Sprintf("RESPONSE: %+v", scr))

		s.Equal("video/quicktime", scr.Outputs[0].Value.Source.MediaType)
		s.Equal("forkingname.mov", scr.Outputs[0].Value.Source.Name)
		s.Equal(claimName, scr.Outputs[0].Name)
		s.EqualValues(strconv.Itoa(17809516), scr.Outputs[0].Value.Source.Size)
		s.Equal(res.SDHash, scr.Outputs[0].Value.Source.SdHash)
		fmt.Printf("%+v\n", scr.Outputs[0])

		s.NoFileExists(c.blobsPath, res.SDHash)
	}
}
func (s *carriageSuite) TestProcessImage() {
	c, err := NewCarriage(s.T().TempDir(), nil, config.GetReflectorUpstream(), nil)
	s.Require().NoError(err)

	ts := time.Now().Format("2006-01-02-15-04-05-000")
	claimName := fmt.Sprintf("publishv3testimage-%s", ts)
	for i := 0; i <= 1; i++ {
		p := UploadProcessPayload{
			UploadID: "",
			UserID:   s.userHelper.UserID(),
			Path:     test.StaticAsset(s.T(), "image2.jpg"),
			Request: jsonrpc.NewRequest(query.MethodStreamCreate, map[string]interface{}{
				"name":                 claimName,
				"title":                fmt.Sprintf("Publish v3 Image: Reel %s", ts),
				"description":          "",
				"locations":            []string{},
				"bid":                  "0.001",
				"languages":            []string{"en"},
				"tags":                 []string{"c:disable-comments"},
				"thumbnail_url":        "https://thumbs.odycdn.com/92399dc6df41af6f7c61def97335dfa5.webp",
				"release_time":         time.Now().Unix(),
				"blocking":             true,
				"preview":              false,
				"license":              "None",
				"channel_id":           "febc557fcfbe5c1813eb621f7d38a80bc4355085",
				"file_path":            "__POST_FILE__",
				"allow_duplicate_name": true,
			}),
		}

		res, err := c.Process(p)
		s.Require().NoError(err)

		pp := p.Request.Params.(map[string]interface{})
		s.EqualValues(375172, pp["file_size"])
		s.EqualValues(2048, pp["width"])
		s.EqualValues(1365, pp["height"])

		scr := StreamCreateResponse{}
		rr, err := json.Marshal(res.Response.Result)
		s.Require().NoError(err)
		err = json.Unmarshal(rr, &scr)
		s.Require().NoError(err, fmt.Sprintf("RESPONSE: %+v", scr))

		s.Equal("image/jpeg", scr.Outputs[0].Value.Source.MediaType)
		s.Equal("image2.jpg", scr.Outputs[0].Value.Source.Name)
		s.Equal(claimName, scr.Outputs[0].Name)
		s.EqualValues(strconv.Itoa(375172), scr.Outputs[0].Value.Source.Size)
		s.Equal(res.SDHash, scr.Outputs[0].Value.Source.SdHash)

		s.NoFileExists(c.blobsPath, res.SDHash)
	}
}

func (s *carriageSuite) TestProcessDoc() {
	c, err := NewCarriage(s.T().TempDir(), nil, config.GetReflectorUpstream(), nil)
	s.Require().NoError(err)
	ts := time.Now().Format("2006-01-02-15-04-05-000")
	claimName := fmt.Sprintf("publishv3testdoc-%s", ts)
	for i := 0; i <= 1; i++ {
		p := UploadProcessPayload{
			UploadID: "",
			UserID:   s.userHelper.UserID(),
			Path:     test.StaticAsset(s.T(), "doc.pdf"),
			Request: jsonrpc.NewRequest(query.MethodStreamCreate, map[string]interface{}{
				"name":                 claimName,
				"title":                fmt.Sprintf("Publish v3 Image: Doc %s", ts),
				"description":          "",
				"locations":            []string{},
				"bid":                  "0.001",
				"languages":            []string{"en"},
				"tags":                 []string{"c:disable-comments"},
				"thumbnail_url":        "https://thumbs.odycdn.com/92399dc6df41af6f7c61def97335dfa5.webp",
				"release_time":         time.Now().Unix(),
				"blocking":             true,
				"preview":              false,
				"license":              "None",
				"channel_id":           "febc557fcfbe5c1813eb621f7d38a80bc4355085",
				"file_path":            "__POST_FILE__",
				"allow_duplicate_name": true,
			}),
		}

		res, err := c.Process(p)
		s.Require().NoError(err)

		scr := StreamCreateResponse{}
		rr, err := json.Marshal(res.Response.Result)
		s.Require().NoError(err)
		err = json.Unmarshal(rr, &scr)
		s.Require().NoError(err, fmt.Sprintf("RESPONSE: %+v", scr))

		s.Equal("application/pdf", scr.Outputs[0].Value.Source.MediaType)
		s.Equal("doc.pdf", scr.Outputs[0].Value.Source.Name)
		s.Equal(claimName, scr.Outputs[0].Name)
		s.EqualValues(strconv.Itoa(474475), scr.Outputs[0].Value.Source.Size)
		s.Equal(res.SDHash, scr.Outputs[0].Value.Source.SdHash)

		s.NoFileExists(c.blobsPath, res.SDHash)
	}
}

func (s *carriageSuite) SetupSuite() {
	s.userHelper = &e2etest.UserTestHelper{}
	s.forkliftHelper = &ForkliftTestHelper{}
	s.Require().NoError(s.userHelper.Setup(s.T()))
	err := s.forkliftHelper.Setup()
	if errors.Is(err, ErrMissingEnv) {
		s.T().Skipf(err.Error())
	}
	s.T().Cleanup(config.RestoreOverridden)
}

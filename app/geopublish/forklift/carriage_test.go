package forklift

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/e2etest"

	"github.com/stretchr/testify/suite"
	"github.com/ybbus/jsonrpc"
)

type carriageSuite struct {
	suite.Suite
	userHelper     *e2etest.UserTestHelper
	forkliftHelper *ForkliftTestHelper
}

func TestCarriageSuite(t *testing.T) {
	suite.Run(t, new(carriageSuite))
}

func (s *carriageSuite) SetupSuite() {
	s.userHelper = &e2etest.UserTestHelper{}
	s.forkliftHelper = &ForkliftTestHelper{}
	s.Require().NoError(s.userHelper.Setup())
	s.Require().NoError(s.userHelper.InjectTestingWallet())
	err := s.forkliftHelper.Setup()
	if errors.Is(err, ErrMissingEnv) {
		s.T().Skipf(err.Error())
	}
}

func (s *carriageSuite) TearDownSuite() {
	config.RestoreOverridden()
	s.userHelper.Cleanup()
}

func (s *carriageSuite) TestProcess() {
	c, err := NewCarriage(s.T().TempDir(), nil, config.GetReflectorUpstream(), nil)
	s.Require().NoError(err)

	for i := 0; i <= 1; i++ {
		p := UploadProcessPayload{
			UploadID: "",
			UserID:   s.userHelper.UserID(),
			Path:     filepath.Join("testdata", "od_blues.mp4"),
			Request: jsonrpc.NewRequest(query.MethodStreamCreate, map[string]interface{}{
				"name":          "publish2test",
				"title":         "Publish v2 test",
				"description":   "",
				"locations":     []string{},
				"bid":           "0.01000000",
				"languages":     []string{"en"},
				"tags":          []string{"c:disable-comments"},
				"thumbnail_url": "https://thumbs.odycdn.com/92399dc6df41af6f7c61def97335dfa5.webp",
				"release_time":  1661882701,
				"blocking":      true,
				"preview":       false,
				"license":       "None",
				"channel_id":    "febc557fcfbe5c1813eb621f7d38a80bc4355085",
				"file_path":     "__POST_FILE__",
			}),
		}

		res, err := c.Process(p)
		s.Require().NoError(err)

		scr := StreamCreateResponse{}
		rr, err := json.Marshal(res.Response.Result)
		s.Require().NoError(err)
		err = json.Unmarshal(rr, &scr)
		s.Require().NoError(err)
		fmt.Printf("RESPONSE: %+v", scr)

		s.Equal("video/mp4", scr.Outputs[0].Value.Source.MediaType)
		s.Equal("od_blues.mp4", scr.Outputs[0].Value.Source.Name)
		s.Equal("publish2test", scr.Outputs[0].Name)
		s.EqualValues(strconv.Itoa(56109368), scr.Outputs[0].Value.Source.Size)
		s.Equal(res.SDHash, scr.Outputs[0].Value.Source.SdHash)

		s.NoFileExists(c.blobsPath, res.SDHash)
	}
}

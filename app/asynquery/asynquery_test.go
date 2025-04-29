package asynquery

import (
	"reflect"
	"testing"

	"github.com/OdyseeTeam/odysee-api/app/query"
	"github.com/OdyseeTeam/odysee-api/apps/lbrytv/config"
	"github.com/OdyseeTeam/odysee-api/internal/e2etest"
	"github.com/OdyseeTeam/odysee-api/models"
	"github.com/OdyseeTeam/odysee-api/pkg/logging/zapadapter"

	"github.com/Pallinder/go-randomdata"
	"github.com/stretchr/testify/suite"
	"github.com/ybbus/jsonrpc/v2"
)

type asynquerySuite struct {
	suite.Suite

	manager    *CallManager
	userHelper *e2etest.UserTestHelper
}

func TestParseFilePath(t *testing.T) {
	shortID := randomdata.Alphanumeric(32)
	longID := randomdata.Alphanumeric(64)
	tests := []struct {
		filePath string
		want     *FileLocation
		wantErr  bool
	}{
		{
			filePath: "https://uploads.odysee.com/v1/uploads/" + shortID,
			want:     &FileLocation{Server: "uploads.odysee.com", UploadID: shortID},
			wantErr:  false,
		},
		{
			filePath: "https://uploads.odysee.com/v1/uploads/" + longID,
			want:     &FileLocation{Server: "uploads.odysee.com", UploadID: longID},
			wantErr:  false,
		},
		{
			filePath: "invalidpath",
			want:     nil,
			wantErr:  true,
		},
		{
			filePath: "https://uploads.odysee.com/v1/uploads/123",
			want:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			got, err := parseFilePath(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFilePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseFilePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAsynquerySuite(t *testing.T) {
	suite.Run(t, new(asynquerySuite))
}

func (s *asynquerySuite) TestCall() {
	uploadID := randomdata.Alphanumeric(64)
	req := jsonrpc.NewRequest(query.MethodStreamCreate, map[string]any{
		"name":          "publish2test-dummymd",
		"title":         "Publish v2 test for dummy.md",
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
		FilePathParam:   "https://uploads-v4.api.na-backend.odysee.com/v1/uploads/" + uploadID,
	})
	req.ID = randomdata.Number(1, 999999999)

	// Making sure there are no duplicate entries afterwards
	for i := 0; i < 10; i++ {
		_, err := s.manager.Call(s.userHelper.UserID(), req)
		s.Require().NoError(err)
	}
	aqs, err := models.Asynqueries(
		models.AsynqueryWhere.UploadID.EQ(uploadID),
		models.AsynqueryWhere.UserID.EQ(s.userHelper.UserID()),
	).All(s.userHelper.DB)
	s.Require().NoError(err)
	s.EqualValues(1, len(aqs))

	aq := aqs[0]
	s.EqualValues(uploadID, aq.UploadID)
	s.EqualValues(s.userHelper.UserID(), aq.UserID)
	s.EqualValues(models.AsynqueryStatusReceived, aq.Status)

	dreq := &jsonrpc.RPCRequest{}
	s.Require().NoError(aq.Body.Unmarshal(dreq))
	dparams := dreq.Params.(map[string]any)
	params := req.Params.(map[string]any)
	s.Equal(params["name"], dparams["name"])
	s.Equal(params["title"], dparams["title"])
	s.Equal(params[FilePathParam], dparams[FilePathParam])
}

func (s *asynquerySuite) SetupSuite() {
	s.userHelper = &e2etest.UserTestHelper{}
	s.Require().NoError(s.userHelper.Setup(s.T()))

	ro, err := config.GetRedisBusOpts()
	s.Require().NoError(err)
	m, err := NewCallManager(ro, s.userHelper.DB, zapadapter.NewKV(nil))
	s.Require().NoError(err)
	s.manager = m
	// This should be called per-test, if needed
	// go m.Start()
}

func (s *asynquerySuite) TearDownSuite() {
	config.RestoreOverridden()
	s.manager.Shutdown()
}

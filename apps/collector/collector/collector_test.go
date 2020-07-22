package collector

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/lbryio/lbrytv/apps/collector/models"
	"github.com/lbryio/lbrytv/config"
	"github.com/lbryio/lbrytv/internal/storage"
	"github.com/lbryio/lbrytv/pkg/app"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/volatiletech/null"
)

func TestMain(m *testing.M) {
	appConfig := config.ReadConfig("collector")
	dbConfig := appConfig.GetDatabase()
	params := storage.ConnParams{
		Connection:     dbConfig.Connection,
		DBName:         dbConfig.DBName,
		Options:        dbConfig.Options,
		MigrationsPath: "../migrations",
	}
	dbConn, connCleanup := storage.CreateTestConn(params)
	dbConn.SetDefaultConnection()

	code := m.Run()

	connCleanup()
	os.Exit(code)
}

func TestHealthz(t *testing.T) {
	app := app.New("127.0.0.1:11111")
	app.InstallRoutes(RouteInstaller)
	app.Start()
	defer app.Shutdown()
	time.Sleep(200 * time.Millisecond)

	r, err := http.Get("http://127.0.0.1:11111/healthz")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, r.StatusCode)
}

func TestEventHandler(t *testing.T) {
	type testData struct {
		name           string
		input          []byte
		expectedStatus int
		expectedBody   []byte
	}
	tests := []testData{}

	data := EventBuffering{
		URL:      "lbry://one",
		Position: 11654,
	}
	event := models.Event{
		Client: "aaa",
		Type:   "buffering",
		Data:   null.JSON{},
	}
	err := event.Data.Marshal(data)
	require.NoError(t, err)
	serialized, err := json.Marshal(event)
	require.NoError(t, err)
	tests = append(tests, testData{"buffering event", serialized, http.StatusOK, []byte(``)})

	data = EventBuffering{
		URL:      "lbry://one",
		Position: 11654,
	}
	event = models.Event{
		Client: "aaa",
		Type:   "buffering",
		Device: null.StringFrom("android"),
		Data:   null.JSON{},
	}
	err = event.Data.Marshal(data)
	require.NoError(t, err)
	serialized, err = json.Marshal(event)
	require.NoError(t, err)
	tests = append(tests, testData{"buffering event with device", serialized, http.StatusOK, []byte(``)})

	data = EventBuffering{
		Position: 11654,
	}
	event = models.Event{
		Client: "aaa",
		Type:   "buffering",
		Device: null.StringFrom("android"),
		Data:   null.JSON{},
	}
	err = event.Data.Marshal(data)
	require.NoError(t, err)
	serialized, err = json.Marshal(event)
	require.NoError(t, err)
	tests = append(tests, testData{"buffering event with missing fields", serialized, http.StatusBadRequest, []byte(``)})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/events/video", bytes.NewReader(test.input))
			req.Header.Add("content-type", "application/json; charset=utf-8")
			req.Header.Add("host", "collector-service.dev.lbry.tv")
			rr := httptest.NewRecorder()
			EventHandler(rr, req)
			response := rr.Result()
			respBody, err := ioutil.ReadAll(response.Body)
			require.NoError(t, err)
			assert.Equal(t, test.expectedStatus, response.StatusCode)
			if test.expectedStatus != 200 {
				assert.Equal(t, test.expectedBody, respBody, "unexpected response: %s", respBody)
			}
		})
	}

	count, err := models.Events().CountG()
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
	e, err := models.Events(models.EventWhere.ID.EQ(1)).OneG()
	require.NoError(t, err)
	assert.Equal(t, "aaa", e.Client)
	eData := EventBuffering{}
	err = e.Data.Unmarshal(&eData)
	require.NoError(t, err)
	assert.Equal(t, 11654, eData.Position)
	assert.Equal(t, "lbry://one", eData.URL)
}

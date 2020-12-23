package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/lbryio/lbrytv/apps/collector/models"
	env "github.com/lbryio/lbrytv/apps/environment"
	"github.com/lbryio/lbrytv/internal/responses"
	"github.com/lbryio/lbrytv/internal/storage"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/gorilla/mux"
	"github.com/markbates/pkger"
	"github.com/volatiletech/sqlboiler/boil"
)

// This is needed for pkger to pack migration files
var Migrator = storage.NewMigrator("/apps/collector/migrations")

func RouteInstaller(r *mux.Router, _ *env.Environment) {
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {})
	v1Router := r.PathPrefix("/api/v1").Subrouter()

	v1Router.HandleFunc("/events/video", EventHandler).Methods(http.MethodPost)
	v1Router.HandleFunc("/events/video", func(w http.ResponseWriter, r *http.Request) {
		// hs := w.Header()
		// hs.Set("Access-Control-Max-Age", "7200")
		// hs.Set("Access-Control-Allow-Origin", "*")
		// hs.Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept")
	}).Methods(http.MethodOptions)
}

func EventHandler(w http.ResponseWriter, r *http.Request) {
	responses.AddJSONContentType(w)

	f, err := pkger.Open("/apps/collector/openapi.yml")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error opening file for pkger: %v", err)
		return
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	swagger, _ := openapi3.NewSwaggerLoader().LoadSwaggerFromData(data)
	router := openapi3filter.NewRouter().WithSwagger(swagger)

	url := r.URL
	url.Host = r.Host
	url.Scheme = r.URL.Scheme

	route, pathParams, err := router.FindRoute(r.Method, url)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "route error: %v", err)
		return
	}

	validator := &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: pathParams,
		Route:      route,
	}
	if err := openapi3filter.ValidateRequest(context.Background(), validator); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "validation error: %v", err)
		return
	}

	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "error reading request: %v", err)
		return
	}

	eventData := BufferingPost{}
	err = json.Unmarshal(reqBody, &eventData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "error marshaling event: %v", err)
		return
	}

	if eventData.Type != "buffering" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "unsupported event type: %s", eventData.Type)
		return
	}

	device := eventData.Device
	if device != models.BufferEventDeviceAndroid && device != models.BufferEventDeviceWeb {
		device = models.BufferEventDeviceUnknown
	}

	event := models.BufferEvent{
		URL:        eventData.Data.URL,
		Client:     eventData.Client,
		Device:     device,
		Position:   eventData.Data.Position,
		ReadyState: int16(eventData.Data.ReadyState),
	}

	if eventData.Data.Duration != nil {
		event.Duration.SetValid(*eventData.Data.Duration)
	}

	if eventData.Data.StreamDuration != nil {
		event.StreamDuration.SetValid(*eventData.Data.StreamDuration)
	}

	if eventData.Data.StreamBitrate != nil {
		event.StreamBitrate.SetValid(*eventData.Data.StreamBitrate)
	}

	if eventData.Data.Player != "" {
		event.Player.SetValid(eventData.Data.Player)
	}

	err = event.InsertG(boil.Infer())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error saving event: %v", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

type BufferingPost struct {
	Type   string            `json:"type"`
	Client string            `json:"client" `
	Device string            `json:"device,omitempty"`
	Data   BufferingPostData `json:"data"`
}

type BufferingPostData struct {
	URL            string `json:"url"`
	Position       int    `json:"position"`
	Duration       *int   `json:"duration,omitempty"`
	StreamDuration *int   `json:"stream_duration,omitempty"`
	StreamBitrate  *int   `json:"stream_bitrate,omitempty"`
	ReadyState     int    `json:"readyState"`
	Player         string `json:"player"`
}

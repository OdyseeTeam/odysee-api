package uploads

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/go-chi/render"
	"github.com/mitchellh/mapstructure"
)

const MaxURLLength = 2083
const (
	StatusInputError         = "input_error"
	StatusInternalError      = "internal_error"
	StatusSerializationError = "serialization_error"
	StatusURLCreated         = "url_created"
)

var ErrNotFound = &Response{HTTPStatusCode: http.StatusNotFound, Error: "resource not found"}

type Response struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

type URLPayload struct {
	URL      string `json:"url"`
	Filename string `json:"-"`
	UploadID string `json:"-"`
}

type URLCreatedPayload struct {
	UploadID string `json:"upload_id"`
}

// Bind on URLPayload will run after the unmarshalling is complete, its
// a good time to focus some post-processing after a decoding.
func (u *URLPayload) Bind(r *http.Request) error {
	if len(u.URL) > MaxURLLength {
		return errors.New("URL too long")
	}
	pu, err := url.ParseRequestURI(u.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	fn := path.Base(pu.Path)
	if fn == "/" || fn == "." || fn == "" {
		return errors.New("couldn't determine remote file name")
	}
	uuid, err := generateURLUID()
	if err != nil {
		return fmt.Errorf("couldn't generate upload ID: %w", err)
	}
	u.Filename = fn
	u.UploadID = uuid
	return nil
}

func ErrInvalidRequest(err error) render.Renderer {
	return &Response{
		Err:            err,
		HTTPStatusCode: http.StatusBadRequest,
		Status:         StatusInputError,
		Error:          fmt.Sprintf("invalid request: %s", err.Error()),
	}
}

func ErrInternalError(err error) render.Renderer {
	return &Response{
		Err:            err,
		HTTPStatusCode: http.StatusInternalServerError,
		Status:         StatusInternalError,
		Error:          fmt.Sprintf("internal error: %s", err.Error()),
	}
}

func ErrRender(err error) render.Renderer {
	return &Response{
		Err:            err,
		HTTPStatusCode: http.StatusUnprocessableEntity,
		Status:         StatusSerializationError,
		Error:          fmt.Sprintf("error rendering response: %s", err.Error()),
	}
}

func ResponseURLCreated(uploadID string) render.Renderer {
	return &Response{
		HTTPStatusCode: http.StatusCreated,
		Status:         StatusURLCreated,
		Payload:        &URLCreatedPayload{uploadID},
	}
}

func (e *Response) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.HTTPStatusCode)
	return nil
}

func (e *Response) UnmarshalJSON(data []byte) error {
	type responseAlias Response // Alias to avoid recursion
	aux := &responseAlias{
		Payload: json.RawMessage{},
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*e = Response(*aux)
	var payload any
	switch e.Status {
	case StatusURLCreated:
		payload = URLCreatedPayload{}
	default:
		return errors.New("unknown status")
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           &payload,
		TagName:          "json",
		WeaklyTypedInput: true,
	})
	if err != nil {
		return fmt.Errorf("error configuring payload decoder: %w", err)
	}

	if err := decoder.Decode(e.Payload); err != nil {
		return fmt.Errorf("error decoding payload: %w", err)
	}
	e.Payload = payload

	return nil
}

func generateURLUID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

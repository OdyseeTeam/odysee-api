package publish

import "fmt"

// FetchError reports an error and the remote URL that caused it.
type FetchError struct {
	URL string
	Err error
}

func (e *FetchError) Unwrap() error { return e.Err }
func (e *FetchError) Error() string { return fmt.Sprintf("fetch error on %q: %s", e.URL, e.Err) }

// RequestError reports an error that was due invalid request from client.
type RequestError struct {
	Err error
	Msg string
}

func (e *RequestError) Unwrap() error { return e.Err }
func (e *RequestError) Error() string { return fmt.Sprintf("request error: %s", e.Err) }

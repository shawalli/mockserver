package httpmock

import (
	"errors"
	"net/http"
)

var ErrWriteReturnBody = errors.New("error writing return body")

// ResponseWriter writes a HTTP response and returns the number of bytes written
// and whether or not the operation encountered an error.
//
// [*http.Request] is provided as an argument so that the response writer can
// use information from the request when crafting the response. If the
// [ResponseWriter] is static, the [*http.Request] may be safely ignored.
type ResponseWriter func(w http.ResponseWriter, r *http.Request) (int, error)

// Response hold the parts of the response that should be returned.
type Response struct {
	parent *Request

	// The HTTP status code that should be used in a response.
	statusCode int

	// Headers that should be used in a response.
	header http.Header

	// Body that should be used in a response.
	body []byte

	// Custom response writer that overrides statusCode, header, and body
	// configurations.
	writer ResponseWriter
}

func newResponse(parent *Request, statusCode int, body []byte) *Response {
	return &Response{
		parent:     parent,
		statusCode: statusCode,
		header:     http.Header{},
		body:       body,
	}
}

// lock is a convenience method to lock the grandparent mock's mutex.
func (r *Response) lock() {
	r.parent.parent.mutex.Lock()
}

// unlock is a convenience method to unlock the grandparent mock's mutex.
func (r *Response) unlock() {
	r.parent.parent.mutex.Unlock()
}

// Header sets the value or values for a response header. Any prior values that
// have already been set for a header with the same key will be overridden.
func (r *Response) Header(key string, value string, values ...string) *Response {
	r.lock()
	defer r.unlock()

	v := append(r.header[key], value)
	r.header[key] = append(v, values...)
	return r
}

// Once is a convenience method which indicates that the grandparent mock
// should only expect the parent request once.
//
//	Mock.On(http.MethodDelete, "/some/path/1234").RespondNoContent().Once()
func (r *Response) Once() *Request {
	return r.parent.Once()
}

// Twice is a convenience method which indicates that the grandparent mock
// should only expect the parent request twice.
//
//	Mock.On(http.MethodDelete, "/some/path/1234").RespondNoContent().Twice()
func (r *Response) Twice() *Request {
	return r.parent.Twice()
}

// Times is a convenience method which indicates that the grandparent mock
// should only expect the parent request the indicated number of times.
//
//	Mock.On(http.MethodDelete, "/some/path/1234").RespondNoContent().Times(5)
func (r *Response) Times(i int) *Request {
	return r.parent.Times(i)
}

// On chains a new expectation description onto the grandparent mock. This
// allows syntax like:
//
//	Mock.
//		On(http.MethodPost, "/some/path").RespondOk([]byte(`{"id": "1234"}`)).
//		On(http.MethodDelete, "/some/path/1234").RespondNoContent().
//		On(http.MethodDelete, "/some/path/1234").Respond(http.StatusNotFound, nil)
func (r *Response) On(method string, path string, body []byte) *Request {
	return r.parent.parent.On(method, path, body)
}

// Write the response to the provided [http.ResponseWriter]. The number of bytes
// successfully written to the [http.ResponseWriter] are returned, as well as
// any errors.
//
// Note: If [Request.RespondUsing] was previously called, all response
// configurations are ignored except for the provided custom [ResponseWriter].
func (r *Response) Write(w http.ResponseWriter, req *http.Request) (int, error) {
	r.lock()
	defer r.unlock()

	if r.writer != nil {
		return r.writer(w, req)
	}

	h := w.Header()
	for key, values := range r.header {
		h[key] = values
	}

	w.WriteHeader(r.statusCode)

	if r.body != nil {
		n, err := w.Write(r.body)
		if err != nil {
			return n, ErrWriteReturnBody
		}
		return n, nil
	}

	return 0, nil
}

package httpmock

import (
	"errors"
	"net/http"
)

var ErrWriteReturnBody = errors.New("error writing return body")

type Response struct {
	parent *Request

	statusCode int
	header     http.Header
	body       []byte
}

func newResponse(parent *Request, statusCode int, body []byte) *Response {
	return &Response{
		parent:     parent,
		statusCode: statusCode,
		header:     http.Header{},
		body:       body,
	}
}

func (r *Response) lock() {
	r.parent.parent.mutex.Lock()
}

func (r *Response) unlock() {
	r.parent.parent.mutex.Unlock()
}

func (r *Response) Header(key string, value string, values ...string) *Response {
	r.lock()
	defer r.unlock()

	v := append(r.header[key], value)
	r.header[key] = append(v, values...)
	return r
}

func (r *Response) Times(i int) *Request {
	return r.parent.Times(i)
}

func (r *Response) Once() *Request {
	return r.parent.Once()
}

func (r *Response) Twice() *Request {
	return r.parent.Twice()
}

func (r *Response) Write(w http.ResponseWriter) (int, error) {
	r.lock()
	defer r.unlock()

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

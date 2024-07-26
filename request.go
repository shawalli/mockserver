package httpmock

import (
	"errors"
	"net/http"
	"net/url"
)

var (
	ErrReturnStatusCode = errors.New("invalid return status code")
	ErrWriteReturnBody  = errors.New("error writing return body")

	AnyMethod = "mock.AnyMethod"
)

type Request struct {
	method string
	url    *url.URL
	body   []byte

	returnStatusCode int
	returnHeaders    *http.Header
	returnBody       []byte

	repeatability int
}

func newRequest(method string, URL *url.URL) *Request {
	return &Request{
		method: method,
		url:    URL,
	}
}

func (r *Request) Body(body []byte) *Request {
	r.body = body

	return r
}

func (r *Request) QueryParam(param string, value string) *Request {
	values := r.url.Query()

	values.Set(param, value)
	r.url.RawQuery = values.Encode()

	return r
}

func (r *Request) ReturnStatusCode(statusCode int) *Request {
	r.returnStatusCode = statusCode

	return r
}

func (r *Request) ReturnStatusOK() *Request {
	r.returnStatusCode = http.StatusOK

	return r
}

func (r *Request) ReturnStatusNoContent() *Request {
	r.returnStatusCode = http.StatusNoContent

	return r
}

func (r *Request) ReturnHeaders(header string, value string) *Request {
	if r.returnHeaders == nil {
		r.returnHeaders = &http.Header{}
	}

	r.returnHeaders.Set(header, value)

	return r
}

func (r *Request) ReturnBody(body []byte) *Request {
	r.returnBody = body

	return r
}

func (r *Request) WriteResponse(w http.ResponseWriter) error {
	if r.returnStatusCode == 0 {
		return ErrReturnStatusCode
	}

	if r.returnHeaders != nil {
		for header, values := range *r.returnHeaders {
			for _, value := range values {
				w.Header().Set(header, value)
			}
		}
	}

	w.WriteHeader(r.returnStatusCode)

	if r.returnBody != nil {
		_, err := w.Write(r.returnBody)
		if err != nil {
			return ErrWriteReturnBody
		}
	}

	return nil
}

func (r *Request) Once() *Request {
	return r.Times(1)
}

func (r *Request) Twice() *Request {
	return r.Times(2)
}

func (r *Request) Times(i int) *Request {
	r.repeatability = i
	return r
}

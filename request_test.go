package httpmock

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type constraintFunc func(r *Request) *Request

type badReader struct{}

func (br *badReader) Read(_ []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func TestRequest_Modifiers(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		url         string
		body        []byte
		constraints []constraintFunc
		want        *Request
	}{
		{
			name:   "no-constraints",
			method: http.MethodGet,
			url:    "https://test.com/foo",
			want: &Request{
				method: "GET",
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
			},
		},
		{
			name:   "method",
			method: http.MethodPut,
			url:    "https://test.com/foo",
			want: &Request{
				method: "PUT",
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
			},
		},
		{
			name:   "any-method",
			method: AnyMethod,
			url:    "https://test.com/foo",
			want: &Request{
				method: AnyMethod,
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
			},
		},
		{
			name:   "body",
			method: http.MethodGet,
			url:    "https://test.com/foo",
			body:   []byte(`Hello World!`),
			want: &Request{
				method: "GET",
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				body: []byte(`Hello World!`),
			},
		},
		{
			name:   "query-param-first",
			method: http.MethodGet,
			url:    "https://test.com/foo",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.QueryParam("limit", "1234") },
			},
			want: &Request{
				method: "GET",
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "limit=1234",
				},
			},
		},
		{
			name:   "query-param-repeat-call",
			method: http.MethodGet,
			url:    "https://test.com/foo",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.QueryParam("limit", "1234") },
				func(r *Request) *Request { return r.QueryParam("limit", "5678") },
			},
			want: &Request{
				method: "GET",
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "limit=5678",
				},
			},
		},
		{
			name:   "query-param",
			method: http.MethodGet,
			url:    "https://test.com/foo?next=aaa21242&count=2",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.QueryParam("limit", "1234") },
				func(r *Request) *Request { return r.QueryParam("next", "aaa21242") },
			},
			want: &Request{
				method: "GET",
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "next=aaa21242&count=2&limit=1234",
				},
			},
		},
		{
			name:   "return-status-code",
			method: AnyMethod,
			url:    "https://test.com/foo",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.ReturnStatusCode(http.StatusForbidden) },
			},
			want: &Request{
				method: AnyMethod,
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				returnStatusCode: 403,
			},
		},
		{
			name:   "return-status-ok",
			method: AnyMethod,
			url:    "https://test.com/foo",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.ReturnStatusOK() },
			},
			want: &Request{
				method: AnyMethod,
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				returnStatusCode: 200,
			},
		},
		{
			name:   "return-status-no-content",
			method: AnyMethod,
			url:    "https://test.com/foo",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.ReturnStatusNoContent() },
			},
			want: &Request{
				method: AnyMethod,
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				returnStatusCode: 204,
			},
		},
		{
			name:   "return-headers-nil",
			method: AnyMethod,
			url:    "https://test.com/foo",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.ReturnHeaders("Content-Length", "5") },
			},
			want: &Request{
				method: AnyMethod,
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				returnHeaders: &http.Header{
					"Content-Length": []string{"5"},
				},
			},
		},
		{
			name:   "return-headers",
			method: AnyMethod,
			url:    "https://test.com/foo",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.ReturnHeaders("Content-Length", "5") },
				func(r *Request) *Request { return r.ReturnHeaders("Content-Type", "application/json") },
			},
			want: &Request{
				method: AnyMethod,
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				returnHeaders: &http.Header{
					"Content-Length": []string{"5"},
					"Content-Type":   []string{"application/json"},
				},
			},
		},
		{
			name:   "return-headers-overwrite",
			method: AnyMethod,
			url:    "https://test.com/foo",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.ReturnHeaders("Content-Length", "5") },
				func(r *Request) *Request { return r.ReturnHeaders("Content-Length", "7") },
			},
			want: &Request{
				method: AnyMethod,
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				returnHeaders: &http.Header{
					"Content-Length": []string{"7"},
				},
			},
		},
		{
			name:   "return-body",
			method: AnyMethod,
			url:    "https://test.com/foo",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.ReturnBody([]byte(`Hello World!`)) },
			},
			want: &Request{
				method: AnyMethod,
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				returnBody: []byte(`Hello World!`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			m := new(Mock)

			url, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("unexpected failure to parse test url: %v", err)
			}

			got := newRequest(m, tt.method, url, tt.body)

			// Test
			for _, constraint := range tt.constraints {
				got = constraint(got)
			}

			// Assertions
			gotQuery := got.url.Query()
			got.url.RawQuery = ""
			wantQuery := tt.want.url.Query()
			tt.want.parent = m
			tt.want.url.RawQuery = ""
			assert.Equal(t, wantQuery, gotQuery)
			assert.Equal(t, tt.want, got)

		})
	}
}

func TestRequest_WriteResponse_MissingStatusCode(t *testing.T) {
	// Setup
	r := &Request{
		method: AnyMethod,
		url: &url.URL{
			Scheme: "https",
			Host:   "test.com",
		},
		parent: new(Mock),
	}

	mockResponseWriter := httptest.NewRecorder()

	// Test
	gotErr := r.WriteResponse(mockResponseWriter)

	// Assertions
	assert.ErrorIs(t, gotErr, ErrReturnStatusCode)
}

func TestRequest_WriteResponse(t *testing.T) {
	tests := []struct {
		name        string
		constraints []constraintFunc
		wantHeaders http.Header
		wantBody    []byte
	}{
		{
			name:        "no-headers-no-body",
			wantHeaders: http.Header{},
			wantBody:    []byte(``),
		},
		{
			name: "headers-no-body",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.ReturnHeaders("Content-Length", "5") },
				func(r *Request) *Request { return r.ReturnHeaders("Content-Type", "text/plain; charset=utf-8") },
			},
			wantHeaders: http.Header{
				"Content-Length": []string{"5"},
				"Content-Type":   []string{"text/plain; charset=utf-8"},
			},
			wantBody: []byte(``),
		},
		{
			name: "no-headers-body",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.ReturnBody([]byte(`Hello World!`)) },
			},
			wantHeaders: http.Header{},
			wantBody:    []byte(`Hello World!`),
		},
		{
			name: "headers-and-body",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.ReturnHeaders("Content-Length", "5") },
				func(r *Request) *Request { return r.ReturnHeaders("Content-Type", "text/plain; charset=utf-8") },
				func(r *Request) *Request { return r.ReturnBody([]byte(`Hello World!`)) },
			},
			wantHeaders: http.Header{
				"Content-Length": []string{"5"},
				"Content-Type":   []string{"text/plain; charset=utf-8"},
			},
			wantBody: []byte(`Hello World!`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			r := &Request{
				method: AnyMethod,
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
				},
				parent:           new(Mock),
				returnStatusCode: http.StatusOK,
			}
			for _, constraint := range tt.constraints {
				r = constraint(r)
			}

			mockResponseWriter := httptest.NewRecorder()

			// Test
			r.WriteResponse(mockResponseWriter)

			got := mockResponseWriter.Result()

			gotBody, err := io.ReadAll(got.Body)
			if err != nil {
				t.Fatalf("unexpected error reading response body: %v", err)
			}
			defer got.Body.Close()

			// Assertions
			assert.Equal(t, http.StatusOK, got.StatusCode)
			assert.Equal(t, tt.wantHeaders, got.Header)
			assert.Equal(t, tt.wantBody, gotBody)
		})
	}
}

func TestRequest_Times(t *testing.T) {
	// Setup
	r := Request{
		parent: new(Mock),
	}

	// Test
	r.Times(4)

	// Assertions
	assert.Equal(t, 4, r.repeatability)
}

func TestRequest_Once(t *testing.T) {
	// Setup
	r := Request{
		parent: new(Mock),
	}

	// Test
	r.Once()

	// Assertions
	assert.Equal(t, 1, r.repeatability)
}

func TestRequest_Twice(t *testing.T) {
	// Setup
	r := Request{
		parent: new(Mock),
	}

	// Test
	r.Twice()

	// Assertions
	assert.Equal(t, 2, r.repeatability)
}

func TestRequest_diffMethod(t *testing.T) {
	tests := []struct {
		name            string
		request         *Request
		other           *http.Request
		wantDifferences bool
	}{
		{
			name:            "missing-request-method",
			request:         &Request{},
			other:           &http.Request{Method: http.MethodGet},
			wantDifferences: true,
		},
		{
			name: "missing-other-method",
			request: &Request{
				method: http.MethodGet,
			},
			other:           &http.Request{},
			wantDifferences: true,
		},
		{
			name:            "different-methods",
			request:         &Request{method: http.MethodGet},
			other:           &http.Request{Method: http.MethodPost},
			wantDifferences: true,
		},
		{
			name:            "anymethod-connect",
			request:         &Request{method: AnyMethod},
			other:           &http.Request{Method: http.MethodConnect},
			wantDifferences: false,
		},
		{
			name:            "anymethod-delete",
			request:         &Request{method: AnyMethod},
			other:           &http.Request{Method: http.MethodDelete},
			wantDifferences: false,
		},
		{
			name:            "anymethod-get",
			request:         &Request{method: AnyMethod},
			other:           &http.Request{Method: http.MethodGet},
			wantDifferences: false,
		},
		{
			name:            "anymethod-head",
			request:         &Request{method: AnyMethod},
			other:           &http.Request{Method: http.MethodHead},
			wantDifferences: false,
		},
		{
			name:            "anymethod-options",
			request:         &Request{method: AnyMethod},
			other:           &http.Request{Method: http.MethodOptions},
			wantDifferences: false,
		},
		{
			name:            "anymethod-patch",
			request:         &Request{method: AnyMethod},
			other:           &http.Request{Method: http.MethodPatch},
			wantDifferences: false,
		},
		{
			name:            "anymethod-post",
			request:         &Request{method: AnyMethod},
			other:           &http.Request{Method: http.MethodPost},
			wantDifferences: false,
		},
		{
			name:            "anymethod-put",
			request:         &Request{method: AnyMethod},
			other:           &http.Request{Method: http.MethodPut},
			wantDifferences: false,
		},
		{
			name:            "anymethod-trace",
			request:         &Request{method: AnyMethod},
			other:           &http.Request{Method: http.MethodTrace},
			wantDifferences: false,
		},
		{
			name:            "equal",
			request:         &Request{method: http.MethodGet},
			other:           &http.Request{Method: http.MethodGet},
			wantDifferences: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test
			got, gotDifferences := tt.request.diffMethod(tt.other)

			// Assertions
			assert.NotEmpty(t, got)
			assert.Equal(t, tt.wantDifferences, gotDifferences != 0)
		})
	}
}

func TestRequest_diffURL(t *testing.T) {
	tests := []struct {
		name            string
		request         *Request
		other           *http.Request
		wantDifferences bool
	}{
		{
			name:            "missing-request-url",
			request:         &Request{url: &url.URL{}},
			other:           &http.Request{URL: &url.URL{Path: "test.com"}},
			wantDifferences: true,
		},
		{
			name:            "missing-other-url",
			request:         &Request{url: &url.URL{Path: "test.com"}},
			other:           &http.Request{URL: &url.URL{}},
			wantDifferences: true,
		},
		{
			name:            "missing-both-url",
			request:         &Request{url: &url.URL{}},
			other:           &http.Request{URL: &url.URL{}},
			wantDifferences: true,
		},
		{
			name:            "missing-request-scheme",
			request:         &Request{url: &url.URL{}},
			other:           &http.Request{URL: &url.URL{Scheme: "http"}},
			wantDifferences: true,
		},
		{
			name:            "missing-other-scheme",
			request:         &Request{url: &url.URL{Scheme: "http"}},
			other:           &http.Request{URL: &url.URL{}},
			wantDifferences: true,
		},
		{
			name:            "different-schemes",
			request:         &Request{url: &url.URL{Scheme: "http"}},
			other:           &http.Request{URL: &url.URL{Scheme: "https"}},
			wantDifferences: true,
		},
		{
			name:            "missing-request-host",
			request:         &Request{url: &url.URL{}},
			other:           &http.Request{URL: &url.URL{Host: "test.com"}},
			wantDifferences: true,
		},
		{
			name:            "missing-other-host",
			request:         &Request{url: &url.URL{Host: "test.com"}},
			other:           &http.Request{URL: &url.URL{}},
			wantDifferences: true,
		},
		{
			name:            "different-hosts",
			request:         &Request{url: &url.URL{Host: "test.com"}},
			other:           &http.Request{URL: &url.URL{Host: "notest.com"}},
			wantDifferences: true,
		},
		{
			name:            "missing-request-path",
			request:         &Request{url: &url.URL{}},
			other:           &http.Request{URL: &url.URL{Path: "/foo"}},
			wantDifferences: true,
		},
		{
			name:            "missing-other-path",
			request:         &Request{url: &url.URL{Path: "/foo"}},
			other:           &http.Request{URL: &url.URL{}},
			wantDifferences: true,
		},
		{
			name:            "different-path",
			request:         &Request{url: &url.URL{Path: "/foo"}},
			other:           &http.Request{URL: &url.URL{Path: "/bar"}},
			wantDifferences: true,
		},
		{
			name:            "missing-other-query",
			request:         &Request{url: &url.URL{RawQuery: "limit=5"}},
			other:           &http.Request{URL: &url.URL{}},
			wantDifferences: true,
		},
		{
			name:            "different-queries",
			request:         &Request{url: &url.URL{RawQuery: "limit=5"}},
			other:           &http.Request{URL: &url.URL{RawQuery: "offset=10"}},
			wantDifferences: true,
		},
		{
			name:            "different-query-values",
			request:         &Request{url: &url.URL{RawQuery: "limit=5"}},
			other:           &http.Request{URL: &url.URL{RawQuery: "limit=10"}},
			wantDifferences: true,
		},
		{
			name:            "different-query-valuesets",
			request:         &Request{url: &url.URL{RawQuery: "limit=5"}},
			other:           &http.Request{URL: &url.URL{RawQuery: "limit=10&limit=5"}},
			wantDifferences: true,
		},
		{
			name: "equal",
			request: &Request{url: &url.URL{
				Scheme:   "https",
				Host:     "test.com",
				Path:     "/foo",
				Fragment: "top",
			}},
			other: &http.Request{URL: &url.URL{
				Scheme:   "https",
				Host:     "test.com",
				Path:     "/foo",
				Fragment: "top",
			}},
			wantDifferences: false,
		},
		{
			name: "equal-query",
			request: &Request{url: &url.URL{
				Scheme:   "https",
				Host:     "test.com",
				Path:     "/foo",
				RawQuery: "limit=5&offset=10&next=abcd",
			}},
			other: &http.Request{URL: &url.URL{
				Scheme:   "https",
				Host:     "test.com",
				Path:     "/foo",
				RawQuery: "limit=5&offset=10&next=abcd",
			}},
			wantDifferences: false,
		},
		{
			name: "equal-query-subset",
			request: &Request{url: &url.URL{
				Scheme:   "https",
				Host:     "test.com",
				Path:     "/foo",
				RawQuery: "limit=5",
			}},
			other: &http.Request{URL: &url.URL{
				Scheme:   "https",
				Host:     "test.com",
				Path:     "/foo",
				RawQuery: "limit=5&offset=10&next=abcd",
			}},
			wantDifferences: false,
		},
		{
			name: "equal-query-unordered",
			request: &Request{url: &url.URL{
				Scheme:   "https",
				Host:     "test.com",
				Path:     "/foo",
				RawQuery: "limit=5&next=abcd&offset=10",
			}},
			other: &http.Request{URL: &url.URL{
				Scheme:   "https",
				Host:     "test.com",
				Path:     "/foo",
				RawQuery: "next=abcd&offset=10&limit=5",
			}},
			wantDifferences: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test
			got, gotDifferences := tt.request.diffURL(tt.other)

			// Assertions
			assert.NotEmpty(t, got)
			assert.Equal(t, tt.wantDifferences, gotDifferences != 0)
		})
	}
}

func TestRequest_diffBody(t *testing.T) {
	tests := []struct {
		name            string
		request         *Request
		other           *http.Request
		wantDifferences bool
	}{
		{
			name:            "missing-request-body",
			request:         &Request{},
			other:           &http.Request{Body: io.NopCloser(strings.NewReader("Hi"))},
			wantDifferences: true,
		},
		{
			name:            "missing-other-body",
			request:         &Request{body: []byte(`Hi`)},
			other:           &http.Request{Body: http.NoBody},
			wantDifferences: true,
		},
		{
			name:            "different-bodies",
			request:         &Request{body: []byte(`Hi`)},
			other:           &http.Request{Body: io.NopCloser(strings.NewReader("HI"))},
			wantDifferences: true,
		},
		{
			name:            "missing-both-bodies",
			request:         &Request{},
			other:           &http.Request{Body: http.NoBody},
			wantDifferences: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test
			got, gotDifferences := tt.request.diffBody(tt.other)

			// Assertions
			assert.NotEmpty(t, got)
			assert.Equal(t, tt.wantDifferences, gotDifferences != 0)
		})
	}
}

func TestRequest_diff(t *testing.T) {
	tests := []struct {
		name            string
		request         *Request
		other           *http.Request
		wantDifferences int
	}{
		{
			name: "method",
			request: &Request{
				method: http.MethodGet,
				url:    &url.URL{Path: "test.com"},
			},
			other: &http.Request{
				Method: http.MethodPut,
				URL:    &url.URL{Path: "test.com"},
				Body:   http.NoBody,
			},
			wantDifferences: 1,
		},
		{
			name: "url",
			request: &Request{
				method: http.MethodGet,
				url: &url.URL{
					Scheme: "http",
					Host:   "test.com",
				},
			},
			other: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme: "https",
					Host:   "test.com",
				},
				Body: http.NoBody,
			},
			wantDifferences: 1,
		},
		{
			name: "query",
			request: &Request{
				method: http.MethodGet,
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "page=2",
				},
			},
			other: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "page=3&limit=10",
				},
				Body: http.NoBody,
			},
			wantDifferences: 1,
		},
		{
			name: "body",
			request: &Request{
				method: http.MethodPost,
				url:    &url.URL{Path: "test.com/foo"},
				body:   []byte(`Hello World!`),
			},
			other: &http.Request{
				Method: http.MethodPost,
				URL:    &url.URL{Path: "test.com/foo"},
				Body:   io.NopCloser(strings.NewReader(`Hi World.`)),
			},
			wantDifferences: 1,
		},
		{
			name: "method-query",
			request: &Request{
				method: http.MethodPost,
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "page=2",
				},
			},
			other: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "page=3&limit=10",
				},
				Body: http.NoBody,
			},
			wantDifferences: 2,
		},
		{
			name: "method-url-query-body",
			request: &Request{
				method: http.MethodGet,
				url: &url.URL{
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "page=2",
				},
			},
			other: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/bar",
				},
				Body: io.NopCloser(strings.NewReader(`{"id": 5, "foo": "bar"}`)),
			},
			wantDifferences: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test
			got, gotDifferennces := tt.request.diff(tt.other)

			// Assertions
			assert.Equal(t, tt.wantDifferences, gotDifferennces)
			assert.NotEmpty(t, got)
		})
	}
}

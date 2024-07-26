package httpmock

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

type constraintFunc func(r *Request) *Request

func TestRequest_Modifiers(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		url         string
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
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.Body([]byte("Hello World!")) },
			},
			want: &Request{
				method: "GET",
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				body: []byte("Hello World!"),
			},
		},
		{
			name:   "body-repeat-call",
			method: http.MethodGet,
			url:    "https://test.com/foo",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.Body([]byte("Hello World!")) },
				func(r *Request) *Request { return r.Body([]byte("Hi there!")) },
			},
			want: &Request{
				method: "GET",
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				body: []byte("Hi there!"),
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
				func(r *Request) *Request { return r.ReturnBody([]byte("Hello World!")) },
			},
			want: &Request{
				method: AnyMethod,
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				returnBody: []byte("Hello World!"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			url, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("unexpected failure to parse test url: %v", err)
			}

			got := newRequest(tt.method, url)

			// Test
			for _, constraint := range tt.constraints {
				got = constraint(got)
			}

			// Assertions
			gotQuery := got.url.Query()
			got.url.RawQuery = ""
			wantQuery := tt.want.url.Query()
			tt.want.url.RawQuery = ""
			assert.Equal(t, wantQuery, gotQuery)
			assert.Equal(t, tt.want, got)

		})
	}
}

func TestRequest_WriteResponse_MissingStatusCode(t *testing.T) {
	// Setup
	url, err := url.Parse("https://test.com/foo")
	if err != nil {
		t.Fatalf("unexpected failure to parse test url: %v", err)
	}

	r := newRequest(AnyMethod, url)

	mockResponseWriter := httptest.NewRecorder()

	// Test
	gotErr := r.WriteResponse(mockResponseWriter)

	// Assertions
	assert.ErrorIs(t, gotErr, ErrReturnStatusCode)
}

func TestRequest_WriteResponse(t *testing.T) {
	url, err := url.Parse("https://test.com/foo")
	if err != nil {
		t.Fatalf("unexpected failure to parse test url: %v", err)
	}

	tests := []struct {
		name        string
		constraints []constraintFunc
		wantHeaders http.Header
		wantBody    []byte
	}{
		{
			name:        "no-headers-no-body",
			wantHeaders: http.Header{},
			wantBody:    []byte(""),
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
			wantBody: []byte(""),
		},
		{
			name: "no-headers-body",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.ReturnBody([]byte("Hello World!")) },
			},
			wantHeaders: http.Header{},
			wantBody:    []byte("Hello World!"),
		},
		{
			name: "headers-and-body",
			constraints: []constraintFunc{
				func(r *Request) *Request { return r.ReturnHeaders("Content-Length", "5") },
				func(r *Request) *Request { return r.ReturnHeaders("Content-Type", "text/plain; charset=utf-8") },
				func(r *Request) *Request { return r.ReturnBody([]byte("Hello World!")) },
			},
			wantHeaders: http.Header{
				"Content-Length": []string{"5"},
				"Content-Type":   []string{"text/plain; charset=utf-8"},
			},
			wantBody: []byte("Hello World!"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			r := newRequest(AnyMethod, url).ReturnStatusOK()
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

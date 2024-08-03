package httpmock

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type badResponseWriter struct{}

func (brw *badResponseWriter) Header() http.Header               { return http.Header{} }
func (brw *badResponseWriter) Write(_ []byte) (n int, err error) { return -1, io.ErrClosedPipe }
func (brw *badResponseWriter) WriteHeader(statusCode int)        {}

func TestResponse_Header(t *testing.T) {
	tests := []struct {
		name       string
		headerFunc func(r *Response) *Response
		want       *Response
	}{
		{
			name:       "no-header",
			headerFunc: func(r *Response) *Response { return r },
			want: &Response{
				statusCode: http.StatusTemporaryRedirect,
				header:     http.Header{},
			},
		},
		{
			name: "one-header-value",
			headerFunc: func(r *Response) *Response {
				return r.Header("foo", "bar")
			},
			want: &Response{
				statusCode: http.StatusTemporaryRedirect,
				header: http.Header{
					"foo": []string{"bar"},
				},
			},
		},
		{
			name: "multiple-header-values",
			headerFunc: func(r *Response) *Response {
				return r.Header("foo", "bar", "baz")
			},
			want: &Response{
				statusCode: http.StatusTemporaryRedirect,
				header: http.Header{
					"foo": []string{"bar", "baz"},
				},
			},
		},
		{
			name: "multiple-header-values-iter",
			headerFunc: func(r *Response) *Response {
				return r.Header("foo", "bar").Header("foo", "baz")
			},
			want: &Response{
				statusCode: http.StatusTemporaryRedirect,
				header: http.Header{
					"foo": []string{"bar", "baz"},
				},
			},
		},
		{
			name: "multiple-headers",
			headerFunc: func(r *Response) *Response {
				return r.Header("foo", "bar").Header("baz", "quz", "2")
			},
			want: &Response{
				statusCode: http.StatusTemporaryRedirect,
				header: http.Header{
					"foo": []string{"bar"},
					"baz": []string{"quz", "2"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			req := &Request{parent: new(Mock).Test(t)}
			resp := newResponse(
				req,
				http.StatusTemporaryRedirect,
				nil,
			)

			// Test
			resp = tt.headerFunc(resp)

			// Assertions
			tt.want.parent = req
			assert.Equal(t, tt.want, resp)
		})
	}
}

func TestResponse_Once(t *testing.T) {
	// Setup
	req := &Request{parent: new(Mock).Test(t)}
	resp := Response{parent: req}

	// Test
	resp.Once()

	// Assertions
	assert.Equal(t, 1, req.repeatability)
}

func TestResponse_Twice(t *testing.T) {
	// Setup
	req := &Request{parent: new(Mock).Test(t)}
	resp := Response{parent: req}

	// Test
	resp.Twice()

	// Assertions
	assert.Equal(t, 2, req.repeatability)
}

func TestResponse_Times(t *testing.T) {
	// Setup
	req := &Request{parent: new(Mock).Test(t)}
	resp := Response{parent: req}

	// Test
	resp.Times(4)

	// Assertions
	assert.Equal(t, 4, req.repeatability)
}

func TestResponse_On(t *testing.T) {
	// Setup
	r := &Response{
		parent:     &Request{parent: new(Mock).Test(t)},
		statusCode: http.StatusOK,
		body:       []byte(`Hello World!`),
	}

	// Test
	got := r.On(http.MethodPut, "test.com/foo", []byte(`{"foo": "bar"}`))

	// Assertions
	assert.NotNil(t, got)
	wantExpectedRequests := []*Request{got}
	assert.Equal(t, wantExpectedRequests, r.parent.parent.ExpectedRequests)
}

func TestResponse_Write_FailWriteBody(t *testing.T) {
	// Setup
	r := &Response{
		parent:     &Request{parent: new(Mock).Test(t)},
		statusCode: http.StatusOK,
		body:       []byte(`Hello World!`),
	}

	// Test
	gotN, gotErr := r.Write(&badResponseWriter{})

	// Assertions
	assert.Equal(t, -1, gotN)
	assert.ErrorIs(t, gotErr, ErrWriteReturnBody)
}

func TestResponse_Write(t *testing.T) {
	tests := []struct {
		name           string
		response       *Response
		wantStatusCode int
		wantHeaders    http.Header
		wantBody       []byte
	}{
		{
			name: "ok",
			response: &Response{
				statusCode: http.StatusOK,
			},
			wantStatusCode: http.StatusOK,
			wantHeaders:    http.Header{},
			wantBody:       []byte(``),
		},
		{
			name: "ok-headers",
			response: &Response{
				statusCode: http.StatusOK,
				body:       []byte(`Hello World!`),
				header:     http.Header{"next": []string{"aaa21242"}},
			},
			wantStatusCode: http.StatusOK,
			wantHeaders:    http.Header{"next": []string{"aaa21242"}},
			wantBody:       []byte(`Hello World!`),
		},
		{
			name: "ok-body",
			response: &Response{
				statusCode: http.StatusOK,
				body:       []byte(`Hello World!`),
			},
			wantStatusCode: http.StatusOK,
			wantHeaders:    http.Header{},
			wantBody:       []byte(`Hello World!`),
		},
		{
			name: "ok-headers-body",
			response: &Response{
				statusCode: http.StatusOK,
				header: http.Header{
					"X-Session-Id": []string{"1234"},
					"X-Request-Id": []string{"5678"},
				},
				body: []byte(`Hello World!`),
			},
			wantStatusCode: http.StatusOK,
			wantHeaders: http.Header{
				"X-Session-Id": []string{"1234"},
				"X-Request-Id": []string{"5678"},
			},
			wantBody: []byte(`Hello World!`),
		},
		{
			name: "bad-request",
			response: &Response{
				statusCode: http.StatusBadRequest,
				body:       []byte(`{"error": "invalid foo"}`),
			},
			wantStatusCode: http.StatusBadRequest,
			wantHeaders:    http.Header{},
			wantBody:       []byte(`{"error": "invalid foo"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tt.response.parent = &Request{parent: new(Mock).Test(t)}

			recorder := httptest.NewRecorder()

			// Test
			gotN, gotErr := tt.response.Write(recorder)

			resp := recorder.Result()
			gotBody, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("unexpected error reading test response body: %v", err)
			}

			// Assertions
			assert.NoError(t, gotErr)
			assert.Equal(t, tt.wantStatusCode, resp.StatusCode)
			assert.Equal(t, tt.wantHeaders, resp.Header)
			assert.Equal(t, len(tt.wantBody), gotN)
			assert.Equal(t, tt.wantBody, gotBody)
		})
	}
}

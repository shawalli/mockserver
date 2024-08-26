package httpmock

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// badResponseWriter implements the [http.ResponseWriter] interface, but always
// fails to write.
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
			expected := &Request{parent: new(Mock).Test(t)}
			response := newResponse(
				expected,
				http.StatusTemporaryRedirect,
				nil,
			)

			// Test
			response = tt.headerFunc(response)

			// Assertions
			tt.want.parent = expected
			assert.Equal(t, tt.want, response)
		})
	}
}

func TestResponse_Once(t *testing.T) {
	// Setup
	expected := &Request{parent: new(Mock).Test(t)}
	response := Response{parent: expected}

	// Test
	response.Once()

	// Assertions
	assert.Equal(t, 1, expected.repeatability)
}

func TestResponse_Twice(t *testing.T) {
	// Setup
	expected := &Request{parent: new(Mock).Test(t)}
	response := Response{parent: expected}

	// Test
	response.Twice()

	// Assertions
	assert.Equal(t, 2, expected.repeatability)
}

func TestResponse_Times(t *testing.T) {
	// Setup
	expected := &Request{parent: new(Mock).Test(t)}
	response := Response{parent: expected}

	// Test
	response.Times(4)

	// Assertions
	assert.Equal(t, 4, expected.repeatability)
}

func TestResponse_On(t *testing.T) {
	// Setup
	response := &Response{
		parent:     &Request{parent: new(Mock).Test(t)},
		statusCode: http.StatusOK,
		body:       []byte(testBody),
	}

	// Test
	got := response.On(http.MethodPut, "test.com/foo", []byte(`{"foo": "bar"}`))

	// Assertions
	assert.NotNil(t, got)
	wantExpectedRequests := []*Request{got}
	assert.Equal(t, wantExpectedRequests, response.parent.parent.ExpectedRequests)
}

func TestResponse_Write_FailWriteBody(t *testing.T) {
	// Setup
	response := &Response{
		parent:     &Request{parent: new(Mock).Test(t)},
		statusCode: http.StatusOK,
		body:       []byte(testBody),
	}

	// Test
	gotN, gotErr := response.Write(&badResponseWriter{}, nil)

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
				body:       []byte(testBody),
				header:     http.Header{"next": []string{"aaa21242"}},
			},
			wantStatusCode: http.StatusOK,
			wantHeaders:    http.Header{"next": []string{"aaa21242"}},
			wantBody:       []byte(testBody),
		},
		{
			name: "ok-body",
			response: &Response{
				statusCode: http.StatusOK,
				body:       []byte(testBody),
			},
			wantStatusCode: http.StatusOK,
			wantHeaders:    http.Header{},
			wantBody:       []byte(testBody),
		},
		{
			name: "ok-headers-body",
			response: &Response{
				statusCode: http.StatusOK,
				header: http.Header{
					"X-Session-Id": []string{"1234"},
					"X-Request-Id": []string{"5678"},
				},
				body: []byte(testBody),
			},
			wantStatusCode: http.StatusOK,
			wantHeaders: http.Header{
				"X-Session-Id": []string{"1234"},
				"X-Request-Id": []string{"5678"},
			},
			wantBody: []byte(testBody),
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
		{
			name: "response-writer",
			response: &Response{
				// Set statusCode, header, and body to verify they are not used
				// during response writing.
				statusCode: http.StatusInternalServerError,
				header:     http.Header{"X-Request-Id": []string{"5678"}},
				body:       []byte("HELP"),
				writer: func(w http.ResponseWriter, r *http.Request) (int, error) {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte(`{"error": "invalid foo"}`))
					return 24, nil
				},
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
			gotN, gotErr := tt.response.Write(recorder, nil)

			response := recorder.Result()
			gotBody, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("unexpected error reading test response body: %v", err)
			}

			// Assertions
			assert.NoError(t, gotErr)
			assert.Equal(t, tt.wantStatusCode, response.StatusCode)
			assert.Equal(t, tt.wantHeaders, response.Header)
			assert.Equal(t, len(tt.wantBody), gotN)
			assert.Equal(t, tt.wantBody, gotBody)
		})
	}
}

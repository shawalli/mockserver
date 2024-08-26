package httpmock

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const testBody = `Hello World!`

var testLongBody = []byte(`
0000000000000000000000000000000000000000000000000000000000000000
1111111111111111111111111111111111111111111111111111111111111111
2222222222222222222222222222222222222222222222222222222222222222
3333333333333333333333333333333333333333333333333333333333333333
4444444444444444444444444444444444444444444444444444444444444444
5555555555555555555555555555555555555555555555555555555555555555
6666666666666666666666666666666666666666666666666666666666666666
7777777777777777777777777777777777777777777777777777777777777777
8888888888888888888888888888888888888888888888888888888888888888
9999999999999999999999999999999999999999999999999999999999999999
aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee
ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
Now I am too long
`)

// badReader implements the io.Reader interface, but always fails to read.
type badReader struct{}

func (br *badReader) Read(_ []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func Test_newRequest(t *testing.T) {
	tests := []struct {
		name   string
		method string
		url    string
		body   []byte
		want   *Request
	}{
		{
			name:   "basic",
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
			name:   "any-method",
			method: AnyMethod,
			url:    "https://test.com/foo",
			want: &Request{
				method: "httpmock.AnyMethod",
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
			body:   []byte(testBody),
			want: &Request{
				method: "GET",
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				body: []byte(testBody),
			},
		},
		{
			name:   "any-body",
			method: http.MethodGet,
			url:    "https://test.com/foo",
			body:   AnyBody,
			want: &Request{
				method: "GET",
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				body: []byte("httpmock.AnyBody"),
			},
		},
		{
			name:   "query-param",
			method: http.MethodGet,
			url:    "https://test.com/foo?limit=1234",
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
			name:   "query-param-multiple-values",
			method: http.MethodGet,
			url:    "https://test.com/foo?limit=1234&limit=5678",
			want: &Request{
				method: "GET",
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "limit=1234&limit=5678",
				},
			},
		},
		{
			name:   "query-param-multiples-keys",
			method: http.MethodGet,
			url:    "https://test.com/foo?next=aaa21242&count=2&limit=1234",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			m := new(Mock)

			url, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("unexpected failure to parse test url: %v", err)
			}

			// Test
			got := newRequest(m, tt.method, url, tt.body)

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

func TestRequest_Respond(t *testing.T) {
	// Setup
	r := &Request{parent: new(Mock)}

	// Test
	got := r.Respond(http.StatusForbidden, []byte(`And stay out!`))

	// Assertions
	want := &Response{
		parent:     r,
		statusCode: http.StatusForbidden,
		header:     http.Header{},
		body:       []byte(`And stay out!`),
	}
	assert.Equal(t, want, got)
	assert.Equal(t, got, r.response)
}

func TestRequest_RespondOK(t *testing.T) {
	// Setup
	r := &Request{parent: new(Mock)}

	// Test
	got := r.RespondOK([]byte(testBody))

	// Assertions
	want := &Response{
		parent:     r,
		statusCode: http.StatusOK,
		header:     http.Header{},
		body:       []byte(testBody),
	}
	assert.Equal(t, want, got)
	assert.Equal(t, got, r.response)
}

func TestRequest_RespondNoContent(t *testing.T) {
	// Setup
	r := &Request{parent: new(Mock)}

	// Test
	got := r.RespondNoContent()

	// Assertions
	want := &Response{
		parent:     r,
		header:     http.Header{},
		statusCode: http.StatusNoContent,
	}
	assert.Equal(t, want, got)
	assert.Equal(t, got, r.response)
}

func TestRequest_RespondUsing(t *testing.T) {
	// Setup
	r := &Request{parent: new(Mock)}

	testWriter := func(w http.ResponseWriter, r *http.Request) (int, error) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`And stay out!`))
		return 13, nil
	}

	// Test
	got := r.RespondUsing(testWriter)

	// Assertions
	assert.Equal(t, got, r.response)
	assert.Zero(t, got.statusCode)
	assert.Empty(t, got.header)
	assert.Empty(t, got.body)
	assert.NotNil(t, got.writer)

	// Instead of comparing function pointers, run the function and assert its
	// output.
	recorder := httptest.NewRecorder()
	gotN, gotErr := got.writer(recorder, &http.Request{})
	assert.Equal(t, 13, gotN)
	assert.Nil(t, gotErr)
	gotResult := recorder.Result()
	gotResultBody, err := io.ReadAll(gotResult.Body)
	if err != nil {
		t.Fatalf("unexpected error reading response: %v", err)
	}
	defer gotResult.Body.Close()
	assert.Equal(t, "And stay out!", string(gotResultBody))
}

func TestRequest_Once(t *testing.T) {
	// Setup
	r := Request{parent: new(Mock)}

	// Test
	r.Once()

	// Assertions
	assert.Equal(t, 1, r.repeatability)
}

func TestRequest_Twice(t *testing.T) {
	// Setup
	r := Request{parent: new(Mock)}

	// Test
	r.Twice()

	// Assertions
	assert.Equal(t, 2, r.repeatability)
}

func TestRequest_Times(t *testing.T) {
	// Setup
	r := Request{parent: new(Mock)}

	// Test
	r.Times(4)

	// Assertions
	assert.Equal(t, 4, r.repeatability)
}

func TestRequest_Matches(t *testing.T) {
	// Setup
	r := Request{parent: new(Mock)}

	match1 := func(received *http.Request) (output string, differences int) {
		return "match1", 0
	}
	match2 := func(received *http.Request) (output string, differences int) {
		return "match2", 1
	}
	match3 := func(received *http.Request) (output string, differences int) {
		return "match3", 2
	}
	match4 := func(received *http.Request) (output string, differences int) {
		return "match4", 3
	}

	// Test
	r.Matches(match1).Matches(match4, match3).Matches(match2)

	// Assertions
	assert.Len(t, r.matchers, 4)
	want := [][]any{
		{"match1", 0},
		{"match4", 3},
		{"match3", 2},
		{"match2", 1},
	}
	for i, m := range r.matchers {
		w := want[i]
		wantOutput := w[0]
		wantDifferences := w[1]

		gotOutput, gotDifferences := m(&http.Request{})

		assert.Equal(t, wantOutput, gotOutput)
		assert.Equal(t, wantDifferences, gotDifferences)
	}
}

func TestRequest_diffMethod(t *testing.T) {
	tests := []struct {
		name            string
		request         *Request
		received        *http.Request
		wantDifferences bool
	}{
		{
			name:            "missing-request-method",
			request:         &Request{},
			received:        &http.Request{Method: http.MethodGet},
			wantDifferences: true,
		},
		{
			name: "missing-received-method",
			request: &Request{
				method: http.MethodGet,
			},
			received:        &http.Request{},
			wantDifferences: true,
		},
		{
			name:            "different-methods",
			request:         &Request{method: http.MethodGet},
			received:        &http.Request{Method: http.MethodPost},
			wantDifferences: true,
		},
		{
			name:            "any-method-connect",
			request:         &Request{method: AnyMethod},
			received:        &http.Request{Method: http.MethodConnect},
			wantDifferences: false,
		},
		{
			name:            "any-method-delete",
			request:         &Request{method: AnyMethod},
			received:        &http.Request{Method: http.MethodDelete},
			wantDifferences: false,
		},
		{
			name:            "any-method-get",
			request:         &Request{method: AnyMethod},
			received:        &http.Request{Method: http.MethodGet},
			wantDifferences: false,
		},
		{
			name:            "any-method-head",
			request:         &Request{method: AnyMethod},
			received:        &http.Request{Method: http.MethodHead},
			wantDifferences: false,
		},
		{
			name:            "any-method-options",
			request:         &Request{method: AnyMethod},
			received:        &http.Request{Method: http.MethodOptions},
			wantDifferences: false,
		},
		{
			name:            "any-method-patch",
			request:         &Request{method: AnyMethod},
			received:        &http.Request{Method: http.MethodPatch},
			wantDifferences: false,
		},
		{
			name:            "any-method-post",
			request:         &Request{method: AnyMethod},
			received:        &http.Request{Method: http.MethodPost},
			wantDifferences: false,
		},
		{
			name:            "any-method-put",
			request:         &Request{method: AnyMethod},
			received:        &http.Request{Method: http.MethodPut},
			wantDifferences: false,
		},
		{
			name:            "any-method-trace",
			request:         &Request{method: AnyMethod},
			received:        &http.Request{Method: http.MethodTrace},
			wantDifferences: false,
		},
		{
			name:            "equal",
			request:         &Request{method: http.MethodGet},
			received:        &http.Request{Method: http.MethodGet},
			wantDifferences: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test
			got, gotDifferences := tt.request.diffMethod(tt.received)

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
		received        *http.Request
		wantDifferences bool
	}{
		{
			name:            "missing-request-url",
			request:         &Request{url: &url.URL{}},
			received:        &http.Request{URL: &url.URL{Path: "test.com"}},
			wantDifferences: true,
		},
		{
			name:            "missing-received-url",
			request:         &Request{url: &url.URL{Path: "test.com"}},
			received:        &http.Request{URL: &url.URL{}},
			wantDifferences: true,
		},
		{
			name:            "missing-both-url",
			request:         &Request{url: &url.URL{}},
			received:        &http.Request{URL: &url.URL{}},
			wantDifferences: true,
		},
		{
			name:            "missing-request-scheme",
			request:         &Request{url: &url.URL{}},
			received:        &http.Request{URL: &url.URL{Scheme: "http"}},
			wantDifferences: true,
		},
		{
			name:            "missing-received-scheme",
			request:         &Request{url: &url.URL{Scheme: "http"}},
			received:        &http.Request{URL: &url.URL{}},
			wantDifferences: true,
		},
		{
			name:            "different-schemes",
			request:         &Request{url: &url.URL{Scheme: "http"}},
			received:        &http.Request{URL: &url.URL{Scheme: "https"}},
			wantDifferences: true,
		},
		{
			name:            "missing-request-host",
			request:         &Request{url: &url.URL{}},
			received:        &http.Request{URL: &url.URL{Host: "test.com"}},
			wantDifferences: true,
		},
		{
			name:            "missing-received-host",
			request:         &Request{url: &url.URL{Host: "test.com"}},
			received:        &http.Request{URL: &url.URL{}},
			wantDifferences: true,
		},
		{
			name:            "different-hosts",
			request:         &Request{url: &url.URL{Host: "test.com"}},
			received:        &http.Request{URL: &url.URL{Host: "notest.com"}},
			wantDifferences: true,
		},
		{
			name:            "missing-request-path",
			request:         &Request{url: &url.URL{}},
			received:        &http.Request{URL: &url.URL{Path: "/foo"}},
			wantDifferences: true,
		},
		{
			name:            "missing-received-path",
			request:         &Request{url: &url.URL{Path: "/foo"}},
			received:        &http.Request{URL: &url.URL{}},
			wantDifferences: true,
		},
		{
			name:            "different-path",
			request:         &Request{url: &url.URL{Path: "/foo"}},
			received:        &http.Request{URL: &url.URL{Path: "/bar"}},
			wantDifferences: true,
		},
		{
			name:            "missing-received-query",
			request:         &Request{url: &url.URL{RawQuery: "limit=5"}},
			received:        &http.Request{URL: &url.URL{}},
			wantDifferences: true,
		},
		{
			name:            "different-queries",
			request:         &Request{url: &url.URL{RawQuery: "limit=5"}},
			received:        &http.Request{URL: &url.URL{RawQuery: "offset=10"}},
			wantDifferences: true,
		},
		{
			name:            "different-query-values",
			request:         &Request{url: &url.URL{RawQuery: "limit=5"}},
			received:        &http.Request{URL: &url.URL{RawQuery: "limit=10"}},
			wantDifferences: true,
		},
		{
			name:            "different-query-valuesets",
			request:         &Request{url: &url.URL{RawQuery: "limit=5"}},
			received:        &http.Request{URL: &url.URL{RawQuery: "limit=10&limit=5"}},
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
			received: &http.Request{URL: &url.URL{
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
			received: &http.Request{URL: &url.URL{
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
			received: &http.Request{URL: &url.URL{
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
			received: &http.Request{URL: &url.URL{
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
			got, gotDifferences := tt.request.diffURL(tt.received)

			// Assertions
			assert.NotEmpty(t, got)
			assert.Equal(t, tt.wantDifferences, gotDifferences != 0)
		})
	}
}

func TestRequest_diffBody_FailToReadBody(t *testing.T) {
	// Setup
	r := &Request{}

	received := mustNewRequest(http.NewRequest(http.MethodPut, "https://test.com/foo", io.NopCloser(&badReader{})))

	// Test
	gotOutput, gotDifferences := r.diffBody(received)

	// Assertions
	assert.Contains(t, gotOutput, ErrReadBody.Error())
	assert.Equal(t, 1, gotDifferences)
}

func TestRequest_diffBody(t *testing.T) {
	tests := []struct {
		name            string
		request         *Request
		received        *http.Request
		wantDifferences bool
	}{
		{
			name:            "missing-request-body",
			request:         &Request{},
			received:        &http.Request{Body: io.NopCloser(strings.NewReader("Hello World!"))},
			wantDifferences: true,
		},
		{
			name:            "missing-received-body",
			request:         &Request{body: []byte(testBody)},
			received:        &http.Request{Body: http.NoBody},
			wantDifferences: true,
		},
		{
			name:            "different-bodies",
			request:         &Request{body: []byte(testBody)},
			received:        &http.Request{Body: io.NopCloser(strings.NewReader("Hello World."))},
			wantDifferences: true,
		},
		{
			name:            "missing-both-bodies",
			request:         &Request{},
			received:        &http.Request{Body: http.NoBody},
			wantDifferences: false,
		},
		{
			name:            "same-bodies",
			request:         &Request{body: []byte("Hello World!")},
			received:        &http.Request{Body: io.NopCloser(strings.NewReader("Hello World!"))},
			wantDifferences: false,
		},
		{
			name:            "long-request-body",
			request:         &Request{body: testLongBody},
			received:        &http.Request{Body: io.NopCloser(strings.NewReader("Hello World!"))},
			wantDifferences: true,
		},
		{
			name:            "long-received-body",
			request:         &Request{},
			received:        &http.Request{Body: io.NopCloser(bytes.NewBuffer(testLongBody))},
			wantDifferences: true,
		},
		{
			name:            "long-both-bodies",
			request:         &Request{body: testLongBody},
			received:        &http.Request{Body: io.NopCloser(bytes.NewBuffer(testLongBody))},
			wantDifferences: false,
		},
		{
			name:            "any-body-no-received-body",
			request:         &Request{body: AnyBody},
			received:        &http.Request{Body: http.NoBody},
			wantDifferences: false,
		},
		{
			name:            "any-body-received-body",
			request:         &Request{body: AnyBody},
			received:        &http.Request{Body: io.NopCloser(strings.NewReader("Hello World!"))},
			wantDifferences: false,
		},
		{
			name:            "any-body-received-long-body",
			request:         &Request{body: AnyBody},
			received:        &http.Request{Body: io.NopCloser(bytes.NewBuffer(testLongBody))},
			wantDifferences: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test
			got, gotDifferences := tt.request.diffBody(tt.received)

			// Assertions
			assert.NotEmpty(t, got)
			assert.Equal(t, tt.wantDifferences, gotDifferences != 0)
		})
	}
}

func testRequestMatcherAlwaysPass(received *http.Request) (output string, differences int) {
	return "PASS:  GOOD == GOOD", 0
}

func testRequestMatcherAlwaysFail(received *http.Request) (output string, differences int) {
	return "PASS:  BAD != GOOD", 1
}

func testRequestMatcherSometimesPass(received *http.Request) (output string, differences int) {
	if received.Method == http.MethodGet {
		return "PASS:  GOOD == GOOD", 0
	}
	return "FAIL:  BAD != GOOD", 1
}

func TestRequest_diff(t *testing.T) {
	tests := []struct {
		name            string
		request         *Request
		received        *http.Request
		wantDifferences int
	}{
		{
			name: "method",
			request: &Request{
				method: http.MethodGet,
				url:    &url.URL{Path: "test.com"},
			},
			received: &http.Request{
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
			received: &http.Request{
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
			received: &http.Request{
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
				body:   []byte(testBody),
			},
			received: &http.Request{
				Method: http.MethodPost,
				URL:    &url.URL{Path: "test.com/foo"},
				Body:   io.NopCloser(strings.NewReader(`Hi World.`)),
			},
			wantDifferences: 1,
		},
		{
			name: "matcher",
			request: &Request{
				method:   http.MethodPost,
				url:      &url.URL{Path: "test.com/foo"},
				matchers: []RequestMatcher{testRequestMatcherAlwaysFail},
			},
			received: &http.Request{
				Method: http.MethodPost,
				URL:    &url.URL{Path: "test.com/foo"},
				Body:   http.NoBody,
			},
			wantDifferences: 1,
		},
		{
			name: "matchers",
			request: &Request{
				method:   http.MethodPost,
				url:      &url.URL{Path: "test.com/foo"},
				matchers: []RequestMatcher{testRequestMatcherAlwaysFail, testRequestMatcherSometimesPass},
			},
			received: &http.Request{
				Method: http.MethodPost,
				URL:    &url.URL{Path: "test.com/foo"},
				Body:   http.NoBody,
			},
			wantDifferences: 2,
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
			received: &http.Request{
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
			name: "method-matcher",
			request: &Request{
				method: http.MethodPost,
				url: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
				},
				matchers: []RequestMatcher{testRequestMatcherAlwaysPass, testRequestMatcherSometimesPass},
			},
			received: &http.Request{
				Method: http.MethodPut,
				URL: &url.URL{
					Scheme: "https",
					Host:   "test.com",
					Path:   "/foo",
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
			received: &http.Request{
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
			got, gotDifferennces := tt.request.diff(tt.received)

			// Assertions
			assert.Equal(t, tt.wantDifferences, gotDifferennces)
			assert.NotEmpty(t, got)
		})
	}
}

func TestRequest_String(t *testing.T) {
	tests := []struct {
		name    string
		request *Request
		want    string
	}{
		{
			name:    "missing-everything",
			request: &Request{url: &url.URL{}},
			want: `
Method: (Missing)
URL: (Missing)
Body: (0) (Missing)`,
		},
		{
			name: "missing-method",
			request: &Request{
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "limit=1",
					Fragment: "back",
				},
				body: []byte(testBody),
			},
			want: `
Method: (Missing)
URL: https://test.com/foo?limit=1#back
	Scheme: https
	Host: test.com
	Path: /foo
	Query: limit=1
	Fragment: back
Body: (12) Hello World!`,
		},
		{
			name: "any-method",
			request: &Request{
				method: AnyMethod,
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "limit=1",
					Fragment: "back",
				},
				body: []byte(testBody),
			},
			want: `
Method: (AnyMethod)
URL: https://test.com/foo?limit=1#back
	Scheme: https
	Host: test.com
	Path: /foo
	Query: limit=1
	Fragment: back
Body: (12) Hello World!`,
		},
		{
			name: "missing-url",
			request: &Request{
				method: http.MethodGet,
				url:    &url.URL{},
				body:   []byte(testBody),
			},
			want: `
Method: GET
URL: (Missing)
Body: (12) Hello World!`,
		},
		{
			name: "missing-url-scheme",
			request: &Request{
				method: http.MethodGet,
				url: &url.URL{
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "limit=1",
					Fragment: "back",
				},
				body: []byte(testBody),
			},
			want: `
Method: GET
URL: //test.com/foo?limit=1#back
	Scheme: (Missing)
	Host: test.com
	Path: /foo
	Query: limit=1
	Fragment: back
Body: (12) Hello World!`,
		},
		{
			name: "missing-url-host",
			request: &Request{
				method: http.MethodGet,
				url: &url.URL{
					Scheme:   "https",
					Path:     "/foo",
					RawQuery: "limit=1",
					Fragment: "back",
				},
				body: []byte(testBody),
			},
			want: `
Method: GET
URL: https:///foo?limit=1#back
	Scheme: https
	Host: (Missing)
	Path: /foo
	Query: limit=1
	Fragment: back
Body: (12) Hello World!`,
		},
		{
			name: "missing-url-path",
			request: &Request{
				method: http.MethodGet,
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					RawQuery: "limit=1",
					Fragment: "back",
				},
				body: []byte(testBody),
			},
			want: `
Method: GET
URL: https://test.com?limit=1#back
	Scheme: https
	Host: test.com
	Path: (Missing)
	Query: limit=1
	Fragment: back
Body: (12) Hello World!`,
		},
		{
			name: "missing-url-query",
			request: &Request{
				method: http.MethodGet,
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					Fragment: "back",
				},
				body: []byte(testBody),
			},
			want: `
Method: GET
URL: https://test.com/foo#back
	Scheme: https
	Host: test.com
	Path: /foo
	Query: (Missing)
	Fragment: back
Body: (12) Hello World!`,
		},
		{
			name: "missing-url-fragment",
			request: &Request{
				method: http.MethodGet,
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "limit=1",
				},
				body: []byte(testBody),
			},
			want: `
Method: GET
URL: https://test.com/foo?limit=1
	Scheme: https
	Host: test.com
	Path: /foo
	Query: limit=1
	Fragment: (Missing)
Body: (12) Hello World!`,
		},
		{
			name: "missing-body",
			request: &Request{
				method: http.MethodGet,
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "limit=1",
					Fragment: "back",
				},
			},
			want: `
Method: GET
URL: https://test.com/foo?limit=1#back
	Scheme: https
	Host: test.com
	Path: /foo
	Query: limit=1
	Fragment: back
Body: (0) (Missing)`,
		},
		{
			name: "long-body",
			request: &Request{
				method: http.MethodGet,
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "limit=1",
					Fragment: "back",
				},
				body: testLongBody[1:],
			},
			want: `
Method: GET
URL: https://test.com/foo?limit=1#back
	Scheme: https
	Host: test.com
	Path: /foo
	Query: limit=1
	Fragment: back
Body: (1058) 0000000000000000000000000000000000000000000000000000000000000000
1111111111111111111111111111111111111111111111111111111111111111
2222222222222222222222222222222222222222222222222222222222222222
3333333333333333333333333333333333333333333333333333333333333333
4444444444444444444444444444444444444444444444444444444444444444
5555555555555555555555555555555555555555555555555555555555555555
6666666666666666666666666666666666666666666666666666666666666666
7777777777777777777777777777777777777777777777777777777777777777
8888888888888888888888888888888888888888888888888888888888888888
9999999999999999999999999999999999999999999999999999999999999999
aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee
fffffffffffffffffffffffffffffffffffffffffffffffff...`,
		},
		{
			name: "any-body",
			request: &Request{
				method: AnyMethod,
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "limit=1",
					Fragment: "back",
				},
				body: AnyBody,
			},
			want: `
Method: (AnyMethod)
URL: https://test.com/foo?limit=1#back
	Scheme: https
	Host: test.com
	Path: /foo
	Query: limit=1
	Fragment: back
Body: (X) (AnyBody)`,
		},
		{
			name: "matcher",
			request: &Request{
				method: http.MethodGet,
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "limit=1",
					Fragment: "back",
				},
				body:     []byte(testBody),
				matchers: []RequestMatcher{testRequestMatcherAlwaysPass},
			},
			want: `
Method: GET
URL: https://test.com/foo?limit=1#back
	Scheme: https
	Host: test.com
	Path: /foo
	Query: limit=1
	Fragment: back
Body: (12) Hello World!
Matcher[0]: github.com/shawalli/httpmock.testRequestMatcherAlwaysPass`,
		},
		{
			name: "matchers",
			request: &Request{
				method: http.MethodGet,
				url: &url.URL{
					Scheme:   "https",
					Host:     "test.com",
					Path:     "/foo",
					RawQuery: "limit=1",
					Fragment: "back",
				},
				body:     []byte(testBody),
				matchers: []RequestMatcher{testRequestMatcherAlwaysPass, testRequestMatcherAlwaysFail},
			},
			want: `
Method: GET
URL: https://test.com/foo?limit=1#back
	Scheme: https
	Host: test.com
	Path: /foo
	Query: limit=1
	Fragment: back
Body: (12) Hello World!
Matcher[0]: github.com/shawalli/httpmock.testRequestMatcherAlwaysPass
Matcher[1]: github.com/shawalli/httpmock.testRequestMatcherAlwaysFail`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test
			got := tt.request.String()
			got = "\n" + got

			// Assertions
			assert.Equal(t, tt.want, got)
		})
	}
}

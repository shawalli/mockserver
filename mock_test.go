package httpmock

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockTestingT mocks a test struct
// Borrowed from testify/mock tests
type MockTestingT struct {
	logfCount, errorfCount, failNowCount int
}

func (m *MockTestingT) Logf(string, ...interface{}) {
	m.logfCount++
}

func (m *MockTestingT) Errorf(string, ...interface{}) {
	m.errorfCount++
}

// FailNow mocks the FailNow call.
// It panics in order to mimic the FailNow behavior in the sense that
// the execution stops.
func (m *MockTestingT) FailNow() {
	m.failNowCount++

	// this function should panic now to stop the execution as expected
	panic("FailNow was called")
}

func (m *MockTestingT) Helper() {}

// mustNewRequest is a convenience test helper that wraps a call to
// http.NewRequest() and panics if an error is returned. It is only
// intended to be used during test setup.
//
//	mustNewRequest(http.NewRequest(http.GetMethpd, "https://test.com/foo", nil))
func mustNewRequest(request *http.Request, err error) *http.Request {
	if err != nil {
		panic(fmt.Sprintf("unexpected error making request: %v", err))
	}
	return request
}

func TestMock_fail_NoTestingT(t *testing.T) {
	// Setup
	var successfulCall int
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Did not expect to get here")
		}
		// Assertions
		assert.Equal(t, "I failed...badly! some error", r.(string))
		assert.Zero(t, successfulCall)
	}()

	m := new(Mock)

	// Test
	m.fail("I failed...%s %v", "badly!", errors.New("some error"))
}

func TestMock_On_BadURL(t *testing.T) {
	// Setup
	var successfulRequestedCall int

	mockT := new(MockTestingT)
	m := new(Mock).Test(mockT)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Did not expect to get here")
		}
		// Assertions
		assert.Equal(t, "FailNow was called", r.(string))
		assert.Equal(t, 1, mockT.failNowCount)
		assert.Zero(t, successfulRequestedCall)
	}()

	// Test
	m.On(http.MethodGet, "\r", nil)
	successfulRequestedCall++
}

func TestMock_On(t *testing.T) {
	// Setup
	m := new(Mock)

	// Test
	got := m.On(http.MethodGet, "https://test.com/foo", nil)

	// Assertions
	assert.Len(t, m.ExpectedRequests, 1)
	want := &Request{
		method: http.MethodGet,
		url: &url.URL{
			Scheme: "https",
			Host:   "test.com",
			Path:   "/foo",
		},
		parent: m,
	}
	assert.Equal(t, want, got)
	assert.Equal(t, want, m.ExpectedRequests[0])
}

func TestMock_findExpectedRequest_Fail(t *testing.T) {
	requestMatcherRequireNextToken := func(received *http.Request) (output string, differences int) {
		if ok := received.URL.Query().Has("next"); !ok {
			output = fmt.Sprintf("FAIL:  missing query-parameter next ((%s))", received.URL.Query().Encode())
			differences = 1
			return
		}
		output = fmt.Sprintf("PASS:  found query-parameter next ((%s))", received.URL.Query().Encode())
		return
	}

	requestMatcherRequirePageToken := func(received *http.Request) (output string, differences int) {
		if ok := received.URL.Query().Has("page"); !ok {
			output = fmt.Sprintf("FAIL:  missing query-parameter page ((%s))", received.URL.Query().Encode())
			differences = 1
			return
		}
		output = fmt.Sprintf("PASS:  found query-parameter page ((%s))", received.URL.Query().Encode())
		return
	}

	tests := []struct {
		name    string
		request *http.Request
	}{
		{
			name:    "wrong-method",
			request: mustNewRequest(http.NewRequest(http.MethodDelete, "https://test.com/bars/1234", http.NoBody)),
		},
		{
			name:    "wrong-path",
			request: mustNewRequest(http.NewRequest(http.MethodGet, "https://test.com/bar", http.NoBody)),
		},
		{
			name:    "wrong-query-param-value",
			request: mustNewRequest(http.NewRequest(http.MethodGet, "https://test.com/bars/1234?limit=2", http.NoBody)),
		},
		{
			name:    "missing-query-param-constraint",
			request: mustNewRequest(http.NewRequest(http.MethodGet, "https://test.com/bars/1234?limit=100", http.NoBody)),
		},
		{
			name:    "missing-body",
			request: mustNewRequest(http.NewRequest(http.MethodPatch, "https://test.com/bars/1234", http.NoBody)),
		},
		{
			name:    "wrong-body",
			request: mustNewRequest(http.NewRequest(http.MethodPatch, "https://test.com/bars/1234", io.NopCloser(strings.NewReader(`{"quz": "west"}`)))),
		},
		{
			name:    "extra-body",
			request: mustNewRequest(http.NewRequest(http.MethodPut, "https://test.com/bars/1234", io.NopCloser(strings.NewReader(`{"quz": "west"}`)))),
		},
		{
			name:    "fail-request-matcher",
			request: mustNewRequest(http.NewRequest(http.MethodPut, "https://test.com/bars/5678", http.NoBody)),
		},
		{
			name:    "fail-one-request-matcher",
			request: mustNewRequest(http.NewRequest(http.MethodPut, "https://test.com/bars/5678", http.NoBody)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			m := new(Mock)
			m.On(http.MethodPatch, "https://test.com/bars/1234", []byte(`{"quz": "east"}`))
			m.On(http.MethodGet, "https://test.com/bars/1234?limit=1", nil)
			m.On(http.MethodGet, "https://test.com/bars/1234?limit=100&page=2", nil)
			m.On(http.MethodPut, "https://test.com/bars/1234", nil)
			m.On(http.MethodPut, "https://test.com/bars/5678", nil).Matches(requestMatcherRequireNextToken)
			m.On(http.MethodPut, "https://test.com/bars/5678?next=1234", nil).Matches(requestMatcherRequireNextToken, requestMatcherRequirePageToken)

			// Test
			gotIndex, gotExpectedRequest := m.findExpectedRequest(tt.request)

			// Assertions
			assert.Nil(t, gotExpectedRequest)
			assert.Equal(t, -1, gotIndex)
		})
	}
}

func TestMock_findExpectedRequest_TooManyRepeats(t *testing.T) {
	// Setup
	m := new(Mock)
	m.On(http.MethodDelete, "https://test.com/bars/1234", nil).Times(-1)

	test := mustNewRequest(http.NewRequest(http.MethodDelete, "https://test.com/bars/1234", http.NoBody))

	// Test
	gotIndex, gotExpectedResult := m.findExpectedRequest(test)

	// Assertions
	assert.Equal(t, -1, gotIndex)
	assert.NotNil(t, gotExpectedResult)
}

func TestMock_findExpectedRequest(t *testing.T) {
	requestMatcherLimitAtLeastTwo := func(received *http.Request) (output string, differences int) {
		if ok := received.URL.Query().Has("limit"); !ok {
			output = fmt.Sprintf("FAIL:  missing query-parameter limit ((%s))", received.URL.Query().Encode())
			differences = 1
			return
		}

		v := received.URL.Query().Get("limit")
		val, err := strconv.Atoi(v)
		if err != nil {
			output = fmt.Sprintf("FAIL:  query-parameter limit=%q (%T) cannot be coerced into int", v, v)
			differences = 1
			return
		}

		if val < 2 {
			output = fmt.Sprintf("FAIL:  query-parameter limit=%v < 2", val)
			differences = 1
			return
		}
		output = fmt.Sprintf("PASS:  query-parameter limit=%v >= 2", val)
		return
	}

	tests := []struct {
		name      string
		request   *http.Request
		wantIndex int
	}{
		{
			name:      "any-method",
			request:   mustNewRequest(http.NewRequest(http.MethodGet, "https://test.com/foo", http.NoBody)),
			wantIndex: 1,
		},
		{
			name:      "no-body",
			request:   mustNewRequest(http.NewRequest(http.MethodPut, "https://test.com/bars/1234", http.NoBody)),
			wantIndex: 3,
		}, {
			name:      "query-param",
			request:   mustNewRequest(http.NewRequest(http.MethodGet, "https://test.com/bars/1234?limit=1", http.NoBody)),
			wantIndex: 2,
		},
		{
			name:      "body",
			request:   mustNewRequest(http.NewRequest(http.MethodPatch, "https://test.com/bars/1234", io.NopCloser(strings.NewReader(`{"quz": "east"}`)))),
			wantIndex: 0,
		},
		{
			name:      "request-matcher",
			request:   mustNewRequest(http.NewRequest(http.MethodGet, "https://test.com/bars/1234?limit=4", http.NoBody)),
			wantIndex: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			m := new(Mock)
			m.On(http.MethodPatch, "https://test.com/bars/1234", []byte(`{"quz": "east"}`))
			m.On(AnyMethod, "https://test.com/foo", nil)
			m.On(http.MethodGet, "https://test.com/bars/1234?limit=1", AnyBody)
			m.On(http.MethodPut, "https://test.com/bars/1234", nil)
			m.On(http.MethodGet, "https://test.com/bars/1234", nil).Matches(requestMatcherLimitAtLeastTwo)

			// Test
			gotIndex, gotExpectedRequest := m.findExpectedRequest(tt.request)

			// Assertions
			assert.NotNil(t, gotExpectedRequest)
			assert.Equal(t, tt.wantIndex, gotIndex)
		})
	}
}

func TestMock_findClosestRequest(t *testing.T) {
	tests := []struct {
		name         string
		mock         func() *Mock
		test         *http.Request
		wantRequest  *Request
		wantMismatch bool
	}{
		{
			name: "no-match",
			mock: func() *Mock {
				return new(Mock)
			},
			test:         mustNewRequest(http.NewRequest(http.MethodGet, "/foo/bar?limit=3", http.NoBody)),
			wantRequest:  nil,
			wantMismatch: false,
		},
		{
			name: "default-match",
			mock: func() *Mock {
				m := new(Mock)
				m.On(http.MethodPut, "/foo", nil)
				return m
			},
			test: mustNewRequest(http.NewRequest(http.MethodGet, "/foo/bar?limit=3", http.NoBody)),
			wantRequest: &Request{
				method: http.MethodPut,
				url:    &url.URL{Path: "/foo"},
			},
			wantMismatch: true,
		},
		{
			name: "favor-first-match",
			mock: func() *Mock {
				m := new(Mock)
				m.On(http.MethodPut, "/foo", nil)
				m.On(http.MethodGet, "/bar", nil)
				return m
			},
			test: mustNewRequest(http.NewRequest(http.MethodGet, "/foo?limit=3", http.NoBody)),
			wantRequest: &Request{
				method: http.MethodPut,
				url:    &url.URL{Path: "/foo"},
			},
			wantMismatch: true,
		},
		{
			name: "favor-repeatability",
			mock: func() *Mock {
				m := new(Mock)
				// mark this endpoint as already matched
				m.On(http.MethodPut, "/foo", nil).Times(-1)
				m.On(http.MethodGet, "/bar", nil).Once()
				return m
			},
			test: mustNewRequest(http.NewRequest(http.MethodPut, "/bar", http.NoBody)),
			wantRequest: &Request{
				method:        http.MethodGet,
				url:           &url.URL{Path: "/bar"},
				repeatability: 1,
			},
			wantMismatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			m := tt.mock()

			// Test
			gotRequest, gotMismatch := m.findClosestRequest(tt.test)

			// Assertions
			if tt.wantRequest != nil {
				tt.wantRequest.parent = m
			}
			assert.Equal(t, tt.wantRequest, gotRequest)
			assert.Equal(t, tt.wantMismatch, gotMismatch != "")
		})
	}
}

func TestMock_Requested_FailToReadRequestBody(t *testing.T) {
	// Setup
	var successfulRequestedCall int

	mockT := &MockTestingT{}
	m := new(Mock).Test(mockT)
	m.On(http.MethodGet, "https://test.com/foo", nil).RespondOK(nil)

	received := mustNewRequest(http.NewRequest(http.MethodPut, "https://test.com/foo", io.NopCloser(&badReader{})))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Did not expect to get here")
		}
		// Assertions
		assert.Equal(t, "FailNow was called", r.(string))
		assert.Equal(t, 1, mockT.failNowCount)
		assert.Zero(t, successfulRequestedCall)
	}()

	// Test
	m.Requested(received)
	successfulRequestedCall++
}

func TestMock_Requested_FailToFindAnyMatch(t *testing.T) {
	// Setup
	var successfulRequestedCall int

	mockT := &MockTestingT{}
	m := new(Mock).Test(mockT)
	m.On(http.MethodGet, "https://test.com/foo", nil).RespondOK(nil)

	received := mustNewRequest(http.NewRequest(http.MethodPut, "https://test.com/foo", http.NoBody))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Did not expect to get here")
		}
		// Assertions
		assert.Equal(t, "FailNow was called", r.(string))
		assert.Equal(t, 1, mockT.failNowCount)
		assert.Zero(t, successfulRequestedCall)
	}()

	// Test
	m.Requested(received)
	successfulRequestedCall++
}

func TestMock_Requested_FailToFindRepeatableMatch(t *testing.T) {
	// Setup
	var successfulRequestedCall int

	mockT := &MockTestingT{}
	m := new(Mock).Test(mockT)
	m.On(http.MethodPut, "https://test.com/foo", nil).RespondOK(nil).Once()

	received := mustNewRequest(http.NewRequest(http.MethodPut, "https://test.com/foo", http.NoBody))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Did not expect to get here")
		}
		// Assertions
		assert.Equal(t, "FailNow was called", r.(string))
		assert.Equal(t, 1, mockT.failNowCount)
		assert.Equal(t, 1, successfulRequestedCall)
	}()

	// Test
	m.Requested(received)
	successfulRequestedCall++
	m.Requested(received)
	successfulRequestedCall++
}

func TestMock_Requested_FailToFindClosestRequest(t *testing.T) {
	// Setup
	var successfulRequestedCall int

	mockT := &MockTestingT{}
	m := new(Mock).Test(mockT)

	received := mustNewRequest(http.NewRequest(http.MethodPut, "https://test.com/foo", http.NoBody))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Did not expect to get here")
		}
		// Assertions
		assert.Equal(t, "FailNow was called", r.(string))
		assert.Equal(t, 1, mockT.failNowCount)
		assert.Zero(t, successfulRequestedCall)
	}()

	// Test
	m.Requested(received)
	successfulRequestedCall++
}

func TestMock_Requested(t *testing.T) {
	// Setup
	m := new(Mock).Test(t)
	wantReq := m.On(http.MethodGet, "https://test.com/foo", nil)
	wantResp := wantReq.RespondOK(nil)

	received := mustNewRequest(http.NewRequest(http.MethodGet, "https://test.com/foo", http.NoBody))

	// Test
	got := m.Requested(received)

	// Assertions
	assert.Equal(t, wantResp, got)
	assert.Equal(t, got.parent, wantReq)
	assert.Equal(t, 1, got.parent.totalRequests)
}

func TestMock_RequestedOnce(t *testing.T) {
	// Setup
	m := new(Mock).Test(t)
	wantReq := m.On(http.MethodGet, "https://test.com/foo", nil).Once()
	wantResp := wantReq.RespondOK(nil)

	received := mustNewRequest(http.NewRequest(http.MethodGet, "https://test.com/foo", http.NoBody))

	// Test
	got := m.Requested(received)

	// Assertions
	assert.Equal(t, wantResp, got)
	assert.Equal(t, got.parent, wantReq)
	assert.Equal(t, -1, got.parent.repeatability)
	assert.Equal(t, 1, got.parent.totalRequests)
}

func TestMock_RequestedTimes(t *testing.T) {
	// Setup
	m := new(Mock).Test(t)
	expected := m.On(http.MethodGet, "https://test.com/foo", nil).Times(4)
	wantResponse := expected.RespondOK(nil)

	received := mustNewRequest(http.NewRequest(http.MethodGet, "https://test.com/foo", http.NoBody))

	// Test
	got := m.Requested(received)

	// Assertions
	assert.Equal(t, wantResponse, got)
	assert.Equal(t, got.parent, expected)
	assert.Equal(t, 3, got.parent.repeatability)
	assert.Equal(t, 1, got.parent.totalRequests)
}

func TestMock_AssertExpectations_NoMatch(t *testing.T) {
	// Setup
	var successfulRequestedCall int

	mockT := new(MockTestingT)
	m := new(Mock).Test(mockT)
	m.On(http.MethodDelete, "test.com/foo/1234", nil).RespondOK([]byte(`{"foo": "bar"}`))

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Did not expect to get here")
		}
		// Assertions
		assert.Equal(t, "FailNow was called", r.(string))
		assert.Equal(t, 1, mockT.failNowCount)
		assert.False(t, m.AssertExpectations(mockT))
		assert.Zero(t, successfulRequestedCall)
	}()

	assert.False(t, m.AssertExpectations(mockT))

	received := mustNewRequest(http.NewRequest(http.MethodGet, "test.com/foo/1234", http.NoBody))

	// Test
	m.Requested(received)
	successfulRequestedCall++
}

func makeEqualToRequestMatcher(key string, value string) RequestMatcher {
	fn := func(received *http.Request) (output string, differences int) {
		v := received.URL.Query().Get(key)

		if !strings.EqualFold(value, v) {
			output = fmt.Sprintf("FAIL:  query-parameter %s=%v != %v", key, v, value)
			differences = 1
			return
		}

		output = fmt.Sprintf("PASS:  query-parameter %s=%v == %v", key, v, value)
		return
	}

	return fn
}

func TestMock_AssertExpectations(t *testing.T) {

	tests := []struct {
		name            string
		method          string
		path            string
		body            []byte
		requestMatchers []RequestMatcher
		received        *http.Request
	}{
		{
			name:   "basic",
			method: http.MethodGet,
			path:   "test.com/foo",
			body:   nil,
			received: mustNewRequest(
				http.NewRequest(
					http.MethodGet,
					"test.com/foo",
					http.NoBody,
				),
			),
		},
		{
			name:   "basic-anymethod",
			method: AnyMethod,
			path:   "test.com/foo",
			body:   nil,
			received: mustNewRequest(
				http.NewRequest(
					http.MethodPost,
					"test.com/foo",
					http.NoBody,
				),
			),
		},
		{
			name:   "query",
			method: http.MethodGet,
			path:   "test.com/foo?page=2",
			body:   nil,
			received: mustNewRequest(
				http.NewRequest(
					http.MethodGet,
					"test.com/foo?page=2",
					http.NoBody,
				),
			),
		},
		{
			name:   "body",
			method: http.MethodPost,
			path:   "test.com/foo",
			body:   []byte(`{"baz": "quz"}`),
			received: mustNewRequest(
				http.NewRequest(
					http.MethodPost,
					"test.com/foo",
					strings.NewReader(`{"baz": "quz"}`),
				),
			),
		},
		{
			name:   "request-matchers",
			method: http.MethodGet,
			path:   "test.com/foo",
			body:   nil,
			requestMatchers: []RequestMatcher{
				makeEqualToRequestMatcher("page", "2"),
				makeEqualToRequestMatcher("limit", "10"),
			},
			received: mustNewRequest(
				http.NewRequest(
					http.MethodGet,
					"test.com/foo?limit=10&page=2",
					http.NoBody,
				),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			m := new(Mock)
			m.On(tt.method, tt.path, tt.body).Matches(tt.requestMatchers...).RespondOK([]byte(`{"foo": "bar"}`))

			mockT := new(testing.T)
			assert.False(t, m.AssertExpectations(mockT))

			// Test
			m.Requested(tt.received)

			// Assertions
			assert.True(t, m.AssertExpectations(mockT))
		})
	}
}

func TestMock_AssertExpectations_Multiple(t *testing.T) {
	// Setup
	m := new(Mock)
	m.On(http.MethodGet, "test.com/foo/1234", nil).RespondOK([]byte(`{"foo": "bar"}`))
	m.On(http.MethodDelete, "test.com/foo/1234", nil).RespondNoContent()

	mockT := new(MockTestingT)
	assert.False(t, m.AssertExpectations(mockT))

	received := mustNewRequest(http.NewRequest(http.MethodGet, "test.com/foo/1234", http.NoBody))

	// Test and Assertions
	m.Requested(received)
	assert.False(t, m.AssertExpectations(mockT))

	received = mustNewRequest(http.NewRequest(http.MethodDelete, "test.com/foo/1234", http.NoBody))
	m.Requested(received)

	assert.True(t, m.AssertExpectations(mockT))
}

func TestMock_AssertExpectations_Once(t *testing.T) {
	// Setup
	m := new(Mock)
	m.On(http.MethodGet, "test.com/foo/1234", nil).RespondOK([]byte(`{"foo": "bar"}`)).Once()

	mockT := new(MockTestingT)
	assert.False(t, m.AssertExpectations(mockT))

	received := mustNewRequest(http.NewRequest(http.MethodGet, "test.com/foo/1234", http.NoBody))

	// Test
	m.Requested(received)

	// Assertions
	assert.True(t, m.AssertExpectations(mockT))
}

func TestMock_AssertExpectations_Twice(t *testing.T) {
	// Setup
	m := new(Mock)
	m.On(http.MethodGet, "test.com/foo/1234", nil).RespondOK([]byte(`{"foo": "bar"}`)).Twice()

	mockT := new(MockTestingT)
	assert.False(t, m.AssertExpectations(mockT))

	received := mustNewRequest(http.NewRequest(http.MethodGet, "test.com/foo/1234", http.NoBody))

	// Test and Assertions
	m.Requested(received)
	assert.False(t, m.AssertExpectations(mockT))

	m.Requested(received)

	assert.True(t, m.AssertExpectations(mockT))
}

func TestMock_AssertExpectations_Repeatability(t *testing.T) {
	// Setup
	m := new(Mock)
	m.On(http.MethodGet, "test.com/foo/1234", nil).RespondOK([]byte(`{"foo": "bar"}`))

	mockT := new(MockTestingT)
	assert.False(t, m.AssertExpectations(mockT))

	received := mustNewRequest(http.NewRequest(http.MethodGet, "test.com/foo/1234", http.NoBody))

	// Test and Assertions
	m.Requested(received)
	assert.True(t, m.AssertExpectations(mockT))

	m.Requested(received)

	assert.True(t, m.AssertExpectations(mockT))
}

func TestMock_AssertNumberOfRequests_FailToParsePath(t *testing.T) {
	// Setup
	mockT := new(MockTestingT)
	m := new(Mock).Test(mockT)

	var successfulAssertion int
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Did not expect to get here")
		}
		// Assertions
		assert.Equal(t, "FailNow was called", r.(string))
		assert.Equal(t, 1, mockT.errorfCount)
		assert.Equal(t, 1, mockT.failNowCount)
		assert.Zero(t, successfulAssertion)
	}()

	// Test
	m.AssertNumberOfRequests(mockT, http.MethodGet, "https://^.com", 1)
	successfulAssertion++
}

func TestMock_AssertNumberOfRequests_Mismatch(t *testing.T) {
	tests := []struct {
		name           string
		requestMethods []string
		requestPath    string
		wantMethod     string
	}{
		{
			name:           "too-few-calls",
			requestMethods: []string{http.MethodGet},
			requestPath:    "https://test.com",
			wantMethod:     http.MethodGet,
		},
		{
			name:           "too-many-calls",
			requestMethods: []string{http.MethodGet, http.MethodGet, http.MethodGet},
			requestPath:    "https://test.com",
			wantMethod:     http.MethodGet,
		},
		{
			name:           "wrong-method",
			requestMethods: []string{http.MethodGet, http.MethodGet},
			requestPath:    "https://test.com",
			wantMethod:     http.MethodPut,
		},
		{
			name:           "wrong-path",
			requestMethods: []string{http.MethodGet, http.MethodGet},
			requestPath:    "https://test.com/foo",
			wantMethod:     http.MethodGet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockT := new(MockTestingT)
			m := new(Mock).Test(mockT)

			for _, method := range tt.requestMethods {
				u, err := url.Parse(tt.requestPath)
				if err != nil {
					t.Fatalf("unexpected error parsing request path: %v", err)
				}

				expected := newRequest(m, method, u, nil)

				m.Requests = append(m.Requests, *expected)
			}

			// Test
			got := m.AssertNumberOfRequests(mockT, tt.wantMethod, "test.com", 2)

			// Assertions
			assert.False(t, got)
		})
	}
}

func TestMock_AssertNumberOfRequests(t *testing.T) {
	tests := []struct {
		name           string
		requestMethods []string
		requestPath    string
		wantMethod     string
		wantPath       string
	}{
		{
			name:           "single",
			requestMethods: []string{http.MethodDelete, http.MethodDelete},
			requestPath:    "https://test.com/foo/1234",
			wantMethod:     http.MethodDelete,
			wantPath:       "https://test.com/foo/1234",
		},
		{
			name:           "multiple",
			requestMethods: []string{http.MethodDelete, http.MethodPut, http.MethodPatch, http.MethodDelete},
			requestPath:    "https://test.com/foo/1234",
			wantMethod:     http.MethodDelete,
			wantPath:       "https://test.com/foo/1234",
		},
		{
			name:           "ignore-arg-path-user",
			requestMethods: []string{http.MethodDelete, http.MethodDelete},
			requestPath:    "https://test.com/foo/1234",
			wantMethod:     http.MethodDelete,
			wantPath:       "https://username:password@test.com/foo/1234",
		},
		{
			name:           "ignore-arg-path-query",
			requestMethods: []string{http.MethodDelete, http.MethodDelete},
			requestPath:    "https://test.com/foo/1234",
			wantMethod:     http.MethodDelete,
			wantPath:       "https://test.com/foo/1234?page=4",
		},
		{
			name:           "ignore-arg-path-fragment",
			requestMethods: []string{http.MethodDelete, http.MethodDelete},
			requestPath:    "https://test.com/foo/1234",
			wantMethod:     http.MethodDelete,
			wantPath:       "https://test.com/foo/1234#back",
		},
		{
			name:           "ignore-request-path-user",
			requestMethods: []string{http.MethodDelete, http.MethodDelete},
			requestPath:    "https://username:password@test.com/foo/1234",
			wantMethod:     http.MethodDelete,
			wantPath:       "https://test.com/foo/1234",
		},
		{
			name:           "ignore-request-path-query",
			requestMethods: []string{http.MethodDelete, http.MethodDelete},
			requestPath:    "https://test.com/foo/1234?page=2",
			wantMethod:     http.MethodDelete,
			wantPath:       "https://test.com/foo/1234",
		},
		{
			name:           "ignore-request-path-fragment",
			requestMethods: []string{http.MethodDelete, http.MethodDelete},
			requestPath:    "https://test.com/foo/1234#bottom",
			wantMethod:     http.MethodDelete,
			wantPath:       "https://test.com/foo/1234",
		},
		{
			name:           "ignore-different-users",
			requestMethods: []string{http.MethodDelete, http.MethodDelete},
			requestPath:    "https://username:PASSWORD@test.com/foo/1234",
			wantMethod:     http.MethodDelete,
			wantPath:       "https://username:password@test.com/foo/1234",
		},
		{
			name:           "ignore-different-queries",
			requestMethods: []string{http.MethodDelete, http.MethodDelete},
			requestPath:    "https://test.com/foo/1234?count=10&page=2&page=3",
			wantMethod:     http.MethodDelete,
			wantPath:       "https://test.com/foo/1234?page=4",
		},
		{
			name:           "ignore-different-fragments",
			requestMethods: []string{http.MethodDelete, http.MethodDelete},
			requestPath:    "https://test.com/foo/1234#bottom",
			wantMethod:     http.MethodDelete,
			wantPath:       "https://test.com/foo/1234#top",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockT := new(MockTestingT)
			m := new(Mock).Test(mockT)

			for _, method := range tt.requestMethods {
				u, err := url.Parse(tt.requestPath)
				if err != nil {
					t.Fatalf("unexpected error parsing request path: %v", err)
				}

				expected := newRequest(m, method, u, nil)

				m.Requests = append(m.Requests, *expected)
			}

			// Test
			got := m.AssertNumberOfRequests(mockT, tt.wantMethod, tt.wantPath, 2)

			// Assertions
			assert.True(t, got)
		})
	}
}

func TestMock_AssertRequested_FailToParsePath(t *testing.T) {
	// Setup
	mockT := new(MockTestingT)
	m := new(Mock).Test(mockT)

	var successfulAssertion int
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Did not expect to get here")
		}
		// Assertions
		assert.Equal(t, "FailNow was called", r.(string))
		assert.Equal(t, 1, mockT.errorfCount)
		assert.Equal(t, 1, mockT.failNowCount)
		assert.Zero(t, successfulAssertion)
	}()

	// Test
	m.AssertRequested(mockT, http.MethodGet, "https://^.com", nil)
	successfulAssertion++
}

func TestMock_AssertRequested_NoMatch(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{
			name:   "wrong-method",
			method: http.MethodDelete,
			path:   "https://test.com/foo/1234",
			body:   nil,
		},
		{
			name:   "wrong-path",
			method: http.MethodGet,
			path:   "https://test.com/foo/1234?limit=2",
			body:   nil,
		},
		{
			name:   "wrong-body",
			method: http.MethodGet,
			path:   "https://test.com/foo/1234?limit=2",
			body:   []byte(testBody),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockT := new(MockTestingT)
			m := new(Mock).Test(mockT)

			u, err := url.Parse(tt.path)
			if err != nil {
				t.Fatalf("unexpected error parsing request path: %v", err)
			}

			actual := newRequest(m, tt.method, u, tt.body)
			m.Requests = append(m.Requests, *actual)

			// Test
			got := m.AssertRequested(mockT, http.MethodGet, "https://test.com/foo/1234", nil)

			// Assertions
			assert.False(t, got)
		})
	}
}

func TestMock_AssertRequested(t *testing.T) {
	// Setup
	mockT := new(MockTestingT)
	m := new(Mock).Test(mockT)

	u, err := url.Parse("https://test.com/foo/1234")
	if err != nil {
		t.Fatalf("unexpected error parsing request path: %v", err)
	}

	actual := newRequest(m, http.MethodPut, u, []byte(testBody))
	m.Requests = append(m.Requests, *actual)

	// Test
	got := m.AssertRequested(mockT, http.MethodPut, "https://test.com/foo/1234", []byte(testBody))

	// Assertions
	assert.True(t, got)
}

func TestMock_AssertNotRequested_FailToParsePath(t *testing.T) {
	// Setup
	mockT := new(MockTestingT)
	m := new(Mock).Test(mockT)

	var successfulAssertion int
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Did not expect to get here")
		}
		// Assertions
		assert.Equal(t, "FailNow was called", r.(string))
		assert.Equal(t, 1, mockT.errorfCount)
		assert.Equal(t, 1, mockT.failNowCount)
		assert.Zero(t, successfulAssertion)
	}()

	// Test
	m.AssertNotRequested(mockT, http.MethodGet, "https://^.com", nil)
	successfulAssertion++
}

func TestMock_AssertNotRequested_NoMatch(t *testing.T) {
	// Setup
	mockT := new(MockTestingT)
	m := new(Mock).Test(mockT)

	u, err := url.Parse("https://test.com/foo/1234")
	if err != nil {
		t.Fatalf("unexpected error parsing request path: %v", err)
	}

	actual := newRequest(m, http.MethodPut, u, []byte(testBody))
	m.Requests = append(m.Requests, *actual)

	// Test
	got := m.AssertNotRequested(mockT, http.MethodPut, "https://test.com/foo/1234", []byte(testBody))

	// Assertions
	assert.False(t, got)
}

func TestMock_AssertNotRequested(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{
			name:   "wrong-method",
			method: http.MethodDelete,
			path:   "https://test.com/foo/1234",
			body:   nil,
		},
		{
			name:   "wrong-path",
			method: http.MethodGet,
			path:   "https://test.com/foo/1234?limit=2",
			body:   nil,
		},
		{
			name:   "wrong-body",
			method: http.MethodGet,
			path:   "https://test.com/foo/1234?limit=2",
			body:   []byte(testBody),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockT := new(MockTestingT)
			m := new(Mock).Test(mockT)

			u, err := url.Parse(tt.path)
			if err != nil {
				t.Fatalf("unexpected error parsing request path: %v", err)
			}

			actual := newRequest(m, tt.method, u, tt.body)
			m.Requests = append(m.Requests, *actual)

			// Test
			got := m.AssertNotRequested(mockT, http.MethodGet, "https://test.com/foo/1234", nil)

			// Assertions
			assert.True(t, got)
		})
	}
}

func TestMatchCandidate_isBetterMatchThan(t *testing.T) {
	tests := []struct {
		name  string
		test  matchCandidate
		other matchCandidate
		want  bool
	}{
		{
			name:  "nil-request",
			test:  matchCandidate{},
			other: matchCandidate{},
			want:  false,
		},
		{
			name:  "nil-other-request",
			test:  matchCandidate{request: &Request{}},
			other: matchCandidate{},
			want:  true,
		},
		{
			name:  "higher-diffcount-than-other",
			test:  matchCandidate{request: &Request{}, diffCount: 3},
			other: matchCandidate{request: &Request{}, diffCount: 2},
			want:  false,
		},
		{
			name:  "lower-diffcount-than-other",
			test:  matchCandidate{request: &Request{}, diffCount: 2},
			other: matchCandidate{request: &Request{}, diffCount: 3},
			want:  true,
		},
		{
			name:  "higher-repeatability-than-other",
			test:  matchCandidate{request: &Request{repeatability: 1}, diffCount: 2},
			other: matchCandidate{request: &Request{repeatability: -1}, diffCount: 2},
			want:  true,
		},
		{
			name:  "higher-repeatability-than-other",
			test:  matchCandidate{request: &Request{repeatability: -1}, diffCount: 2},
			other: matchCandidate{request: &Request{repeatability: 1}, diffCount: 2},
			want:  false,
		},
		{
			name:  "equal-repeatability",
			test:  matchCandidate{request: &Request{repeatability: 1}, diffCount: 2},
			other: matchCandidate{request: &Request{repeatability: 1}, diffCount: 2},
			want:  false,
		},
		{
			name:  "negative-other-repeatability",
			test:  matchCandidate{request: &Request{repeatability: 0}, diffCount: 2},
			other: matchCandidate{request: &Request{repeatability: -1}, diffCount: 2},
			want:  false,
		},
		{
			name:  "equal-negative-repeatability",
			test:  matchCandidate{request: &Request{repeatability: -1}, diffCount: 2},
			other: matchCandidate{request: &Request{repeatability: -1}, diffCount: 2},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test
			got := tt.test.isBetterMatchThan(tt.other)

			// Assertions
			assert.Equal(t, tt.want, got)
		})
	}
}

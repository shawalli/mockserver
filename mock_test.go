package httpmock

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type badReader struct{}

func (br *badReader) Read(_ []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func mustNewRequest(r *http.Request, err error) *http.Request {
	if err != nil {
		panic(fmt.Sprintf("unexpected error making request: %v", err))
	}
	return r
}

func TestMock_DoBadURL(t *testing.T) {
	// Setup
	var gotPanic string
	defer func() {
		if r := recover(); r != nil {
			gotPanic = r.(string)
		}
	}()

	m := new(Mock)

	// Test
	m.On(http.MethodGet, "\r", nil)

	// Assertions
	wantPanic := "failed to parse url"
	assert.Truef(t, strings.HasPrefix(gotPanic, wantPanic), "panic message %q does not contain %q", gotPanic, wantPanic)
}

func TestMock_Do(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			m := new(Mock)
			m.On(http.MethodPatch, "https://test.com/bars/1234", []byte(`{"quz": "east"}`))
			m.On(http.MethodGet, "https://test.com/bars/1234?limit=1", nil)
			m.On(http.MethodGet, "https://test.com/bars/1234?limit=100&page=2", nil)
			m.On(http.MethodPut, "https://test.com/bars/1234", nil)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			m := new(Mock)
			m.On(http.MethodPatch, "https://test.com/bars/1234", []byte(`{"quz": "east"}`))
			m.On(AnyMethod, "https://test.com/foo", nil)
			m.On(http.MethodGet, "https://test.com/bars/1234?limit=1", nil)
			m.On(http.MethodPut, "https://test.com/bars/1234", nil)

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

			// Assert
			if tt.wantRequest != nil {
				tt.wantRequest.parent = m
			}
			assert.Equal(t, tt.wantRequest, gotRequest)
			assert.Equal(t, tt.wantMismatch, gotMismatch != "")
		})
	}
}

// MockTestingT mocks a test struct
// Borrowed from testify/mock tests
type MockTestingT struct {
	logfCount, errorfCount, failNowCount int
}

const mockTestingTFailNowCalled = "FailNow was called"

func (m *MockTestingT) Logf(string, ...interface{}) {
	m.logfCount++
}

func (m *MockTestingT) Errorf(string, ...interface{}) {
	m.errorfCount++
}

func (m *MockTestingT) FailNow() {
	m.failNowCount++

	// this function should panic now to stop the execution as expected
	panic(mockTestingTFailNowCalled)
}

func TestMock_Requested_FailToReadRequestBody(t *testing.T) {
	// Setup
	var gotPanic bool
	defer func() {
		if r := recover(); r != nil {
			gotPanic = true
		}
	}()

	fakeT := &MockTestingT{}
	m := new(Mock).Test(fakeT)
	m.On(http.MethodGet, "https://test.com/foo", nil).RespondOK(nil)

	test := mustNewRequest(http.NewRequest(http.MethodPut, "https://test.com/foo", io.NopCloser(&badReader{})))

	// Test
	got := m.Requested(test)

	// Assertions
	assert.True(t, gotPanic)
	assert.Equal(t, 1, fakeT.failNowCount)
	assert.Nil(t, got)
}

func TestMock_Requested_FailToFindAnyMatch(t *testing.T) {
	// Setup
	var gotPanic bool
	defer func() {
		if r := recover(); r != nil {
			gotPanic = true
		}
	}()

	fakeT := &MockTestingT{}
	m := new(Mock).Test(fakeT)
	m.On(http.MethodGet, "https://test.com/foo", nil).RespondOK(nil)

	test := mustNewRequest(http.NewRequest(http.MethodPut, "https://test.com/foo", http.NoBody))

	// Test
	got := m.Requested(test)

	// Assertions
	assert.True(t, gotPanic)
	assert.Equal(t, 1, fakeT.failNowCount)
	assert.Nil(t, got)
}

func TestMock_Requested_FailToFindRepeatableMatch(t *testing.T) {
	// Setup
	var gotPanic bool
	defer func() {
		if r := recover(); r != nil {
			gotPanic = true
		}
	}()

	fakeT := &MockTestingT{}
	m := new(Mock).Test(fakeT)
	m.On(http.MethodPut, "https://test.com/foo", nil).RespondOK(nil).Once()

	test := mustNewRequest(http.NewRequest(http.MethodPut, "https://test.com/foo", http.NoBody))

	// Test
	_ = m.Requested(test)
	got := m.Requested(test)

	// Assertions
	assert.True(t, gotPanic)
	assert.Equal(t, 1, fakeT.failNowCount)
	assert.Nil(t, got)
}

func TestMock_Requested_FailToFindClosestRequest(t *testing.T) {
	// Setup
	var gotPanic bool
	defer func() {
		if r := recover(); r != nil {
			gotPanic = true
		}
	}()

	fakeT := &MockTestingT{}
	m := new(Mock).Test(fakeT)

	test := mustNewRequest(http.NewRequest(http.MethodPut, "https://test.com/foo", http.NoBody))

	// Test
	got := m.Requested(test)

	// Assertions
	assert.True(t, gotPanic)
	assert.Equal(t, 1, fakeT.failNowCount)
	assert.Nil(t, got)
}

func TestMock_Requested(t *testing.T) {
	// Setup
	m := new(Mock).Test(t)
	wantReq := m.On(http.MethodGet, "https://test.com/foo", nil)
	wantResp := wantReq.RespondOK(nil)

	test := mustNewRequest(http.NewRequest(http.MethodGet, "https://test.com/foo", http.NoBody))

	// Test
	got := m.Requested(test)

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

	test := mustNewRequest(http.NewRequest(http.MethodGet, "https://test.com/foo", http.NoBody))

	// Test
	got := m.Requested(test)

	// Assertions
	assert.Equal(t, wantResp, got)
	assert.Equal(t, got.parent, wantReq)
	assert.Equal(t, -1, got.parent.repeatability)
	assert.Equal(t, 1, got.parent.totalRequests)
}

func TestMock_RequestedTimes(t *testing.T) {
	// Setup
	m := new(Mock).Test(t)
	wantReq := m.On(http.MethodGet, "https://test.com/foo", nil).Times(4)
	wantResp := wantReq.RespondOK(nil)

	test := mustNewRequest(http.NewRequest(http.MethodGet, "https://test.com/foo", http.NoBody))

	// Test
	got := m.Requested(test)

	// Assertions
	assert.Equal(t, wantResp, got)
	assert.Equal(t, got.parent, wantReq)
	assert.Equal(t, 3, got.parent.repeatability)
	assert.Equal(t, 1, got.parent.totalRequests)
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

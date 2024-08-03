package httpmock

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// tHelper is a minimal interface that expects a type to satisfy the
// testing.TB.Helper method.
type tHelper interface {
	Helper()
}

// Mock is the workhorse used to track activity of a server's request.
// For an example of its usage, refer to the README.
type Mock struct {
	// Represents the requests that are expected to be received.
	ExpectedRequests []*Request

	// Holds the requests that were made to a mocked handler or server.
	Requests []Request

	// test is an optional variable that holds the test struct, to be used when
	// an invalid mock request was made.
	test mock.TestingT

	mutex sync.Mutex
}

// On starts a description of an expectation of the specified Request being
// received.
//
//	Mock.On(http.MethodDelete, "/some/path/1234")
func (m *Mock) On(method string, URL string, body []byte) *Request {
	parsedURL, err := url.Parse(URL)
	if err != nil {
		m.fail(fmt.Sprintf("failed to parse url. Error: %v\n", err))
	}

	r := newRequest(
		m,
		method,
		parsedURL,
		body,
	)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.ExpectedRequests = append(m.ExpectedRequests, r)
	return r
}

// Test sets the test struct variable of the mock object.
func (m *Mock) Test(t mock.TestingT) *Mock {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.test = t
	return m
}

// fail the current test with the given formatted format and args. In the case
// that a testing object was defined, it uses the test APIs for failing a test;
// otherwise, it uses panic.
func (m *Mock) fail(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.test == nil {
		panic(fmt.Sprintf(format, args...))
	}
	m.test.Errorf(format, args...)
	m.test.FailNow()
}

// expectedRequests provides a safe mechanism for viewing and modifying the list
// of expected Requests.
func (m *Mock) expectedRequests() []*Request {
	return append([]*Request{}, m.ExpectedRequests...)
}

// expectedRequests provides a safe mechanism for viewing and modifying the list
// of received Requests.
func (m *Mock) requests() []Request {
	return append([]Request{}, m.Requests...)
}

// findExpectedRequest finds the first Request that exactly matches a received
// request and does not have its repeatability disabled.
func (m *Mock) findExpectedRequest(actual *http.Request) (int, *Request) {
	var expectedRequest *Request
	for i, er := range m.ExpectedRequests {
		if _, d := er.diff(actual); d != 0 {
			continue
		}

		expectedRequest = er
		if er.repeatability > -1 {
			return i, er
		}
	}

	return -1, expectedRequest
}

// findClosestRequest finds the first request that most closely matches a
// received request.
//
// This method should only be used if there is no exact match of a received
// request to the list of expected Requests. If a closest match is found, it is
// returned, along with a formatted string of the differences.
func (m *Mock) findClosestRequest(other *http.Request) (*Request, string) {
	var bestMatch matchCandidate

	for _, request := range m.expectedRequests() {
		errInfo, diffCount := request.diff(other)
		tempCandidate := matchCandidate{
			request:   request,
			mismatch:  errInfo,
			diffCount: diffCount,
		}
		if tempCandidate.isBetterMatchThan(bestMatch) {
			bestMatch = tempCandidate
		}
	}

	return bestMatch.request, bestMatch.mismatch
}

// Requested tells the mock that a request has been received and gets a response
// to return. Panics if the request is unexpected (i.e. not preceded by
// appropriate .On .Respond() calls)
func (m *Mock) Requested(r *http.Request) *Response {
	m.mutex.Lock()

	requestBody, err := readHTTPRequestBody(r)
	if err != nil {
		m.mutex.Unlock()
		m.fail("\nassert: httpmock: Failed to read requested body. Error: %v", err)
	}

	found, request := m.findExpectedRequest(r)
	if found < 0 {
		// Expected request found, but has already been requested with repeatable times
		if request != nil {
			m.mutex.Unlock()
			m.fail("\nassert: httpmock: The request has been called over %d times.\n\tEither do one more Mock.On(%q, %q), or remove extra request.", request.totalRequests, r.Method, r.URL.String())
		}
		// We have to fail here - because we don't know what to do for the
		// response. This is becuase:
		//
		//	a) This is a totally unexpected request
		//	b) The arguments are not what was expected, or
		//	c) The deveoper has forgotten to add an accompanying On...Respond pair
		closestRequest, mismatch := m.findClosestRequest(r)
		m.mutex.Unlock()

		if closestRequest != nil {
			tempRequest := &Request{
				parent: m,
				method: r.Method,
				url:    r.URL,
				body:   requestBody,
			}

			tmp := "\t" + strings.Join(strings.Split(tempRequest.String(), "\n"), "\n\t")
			closest := "\t" + strings.Join(strings.Split(closestRequest.String(), "\n"), "\n\t")

			m.fail("\n\nhttpmock: Unexpected Request\n-----------------------------\n\n%s\n\nThe closest request I have is: \n\n%s\nDiff: %s\n",
				tmp,
				closest,
				strings.TrimSpace(mismatch),
			)
		} else {
			m.fail("\nassert: httpmock: I don't know what to return because the request was unexpected.\n\tEither do Mock.On(%q, %q), or remove the request.\n", r.Method, r.URL.String())
		}
	}

	if request.repeatability == 1 {
		request.repeatability = -1
	} else if request.repeatability > 1 {
		request.repeatability--
	}
	request.totalRequests++

	// Add a clean request to received request list
	newRequest := newRequest(m, r.Method, r.URL, requestBody)
	if request.response != nil {
		newResponse := *request.response
		newRequest.response = &newResponse
	}
	m.Requests = append(m.Requests, *newRequest)
	m.mutex.Unlock()

	return request.response
}

// matchCandidate holds details about possible Request matches for a received
// request.
type matchCandidate struct {
	// Matched Request
	request *Request

	// Formatted string showing differences
	mismatch string

	// Number of differences between match candidate and received request.
	diffCount int
}

// isBetterMatchThan compares two matchCandidates to determine whether the
// referenced candidate is better than the other candidate.
func (mc matchCandidate) isBetterMatchThan(other matchCandidate) bool {
	if mc.request == nil {
		return false
	} else if other.request == nil {
		return true
	}

	if mc.diffCount > other.diffCount {
		return false
	} else if mc.diffCount < other.diffCount {
		return true
	}

	if mc.request.repeatability > 0 && other.request.repeatability <= 0 {
		return true
	}

	return false
}

// AssertExpectations assert that everything specified with On and Respond was
// in fact requested as expected. Requests may have occurred in any order.
func (m *Mock) AssertExpectations(t mock.TestingT) bool {
	if th, ok := t.(tHelper); ok {
		th.Helper()
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	var failedExpectations int

	// Iterate through each expectation
	expectedRequests := m.expectedRequests()
	for _, expectedRequest := range expectedRequests {
		if satisfied, reason := m.checkExpectation(expectedRequest); !satisfied {
			failedExpectations++
			t.Logf(reason)
		}
	}

	if failedExpectations != 0 {
		t.Errorf("FAIL: %d out of %d expectation(s) were met.\n\tThe code you are testing needs to make %d more requests(s).", len(expectedRequests)-failedExpectations, len(expectedRequests), failedExpectations)
	}

	return failedExpectations == 0
}

// AssertNumberOfRequests asserts that the request was made expectedRequests times.
//
// This assertion behaves a bit differently than other assertions. There are a few
// parts of the request that are ignored when calculating, including:
//   - URL username/password information
//   - URL query parameters
//   - URL fragment
func (m *Mock) AssertNumberOfRequests(t mock.TestingT, method string, path string, expectedRequests int) bool {
	if th, ok := t.(tHelper); ok {
		th.Helper()
	}

	// Remove parts of the URL for the purposes of general comparison
	u, err := url.Parse(path)
	if err != nil {
		t.Errorf("FAIL: unable to parse path %q into URL: %v", path, err)
		t.FailNow()
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	u.RawFragment = ""
	path = u.String()

	m.mutex.Lock()
	defer m.mutex.Unlock()
	var actualRequests int
	for _, request := range m.requests() {
		if request.method != method {
			continue
		}

		rURL := *request.url
		rURL.User = nil
		rURL.RawQuery = ""
		rURL.Fragment = ""
		if rURL.String() != path {
			continue
		}

		actualRequests++
	}

	return assert.Equal(t, expectedRequests, actualRequests)
}

// AssertRequested asserts that the request was received.
func (m *Mock) AssertRequested(t mock.TestingT, method string, path string, body []byte) bool {
	if th, ok := t.(tHelper); ok {
		th.Helper()
	}

	u, err := url.Parse(path)
	if err != nil {
		t.Errorf("FAIL: unable to parse path %q into URL: %v", path, err)
		t.FailNow()
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	if !m.checkWasRequested(method, u, body) {
		tempRequest := newRequest(m, method, u, body)
		v := "\t" + strings.Join(strings.Split(tempRequest.String(), "\n"), "\n\t")
		return assert.Fail(
			t,
			"Should have requested with the given constraints",
			fmt.Sprintf("Expected to have been requested with\n%v\nbut no actual requests happened", v),
		)
	}
	return true
}

// AssertRequested asserts that the request was not received.
func (m *Mock) AssertNotRequested(t mock.TestingT, method string, path string, body []byte) bool {
	if th, ok := t.(tHelper); ok {
		th.Helper()
	}

	u, err := url.Parse(path)
	if err != nil {
		t.Errorf("FAIL: unable to parse path %q into URL: %v", path, err)
		t.FailNow()
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.checkWasRequested(method, u, body) {
		tempRequest := newRequest(m, method, u, body)
		v := "\t" + strings.Join(strings.Split(tempRequest.String(), "\n"), "\n\t")
		return assert.Fail(
			t,
			"Should not have been requested with the given constraints",
			fmt.Sprintf("Expected not to have been requested with\n%v\nbut actually it was.", v),
		)
	}
	return true
}

// checkExpectation checks whether an expected Request was received, and
// whether it received the expected number of times.
func (m *Mock) checkExpectation(request *Request) (bool, string) {
	if (!m.checkWasRequested(request.method, request.url, request.body) && request.totalRequests == 0) || (request.repeatability > 0) {
		return false, fmt.Sprintf("FAIL:\t%s %s\n\t(%d) %s", request.method, request.url, len(request.body), trimBody(request.body))
	}
	return true, fmt.Sprintf("PASS:\t%s %s\n\t(%d) %s", request.method, request.url, len(request.body), trimBody(request.body))
}

// checkWasRequested checks whether a set of request parameters was received.
func (m *Mock) checkWasRequested(method string, URL *url.URL, body []byte) bool {
	tempHTTPRequest := &http.Request{
		Method: method,
		URL:    URL,
		Body:   io.NopCloser(bytes.NewReader(body)),
	}
	for _, request := range m.requests() {
		if _, d := request.diff(tempHTTPRequest); d == 0 {
			return true
		}
	}
	return false
}

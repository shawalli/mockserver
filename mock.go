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

type tHelper interface {
	Helper()
}

type Mock struct {
	ExpectedRequests []*Request

	Requests []Request

	test mock.TestingT

	mutex sync.Mutex
}

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

func (m *Mock) Test(t mock.TestingT) *Mock {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.test = t
	return m
}

func (m *Mock) fail(format string, args ...interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.test == nil {
		panic(fmt.Sprintf(format, args...))
	}
	m.test.Errorf(format, args...)
	m.test.FailNow()
}

func (m *Mock) expectedRequests() []*Request {
	return append([]*Request{}, m.ExpectedRequests...)
}

func (m *Mock) requests() []Request {
	return append([]Request{}, m.Requests...)
}

// https://goplay.tools/snippet/_1Iu9dcSHKt
func (m *Mock) findExpectedRequest(actual *http.Request) (int, *Request) {
	var expectedRequest *Request
	for i, er := range m.ExpectedRequests {
		if _, d := er.diffMethod(actual); d != 0 {
			continue
		}

		if _, d := er.diffURL(actual); d != 0 {
			continue
		}

		if _, d := er.diffBody(actual); d != 0 {
			continue
		}

		expectedRequest = er
		if er.repeatability > -1 {
			return i, er
		}
	}

	return -1, expectedRequest
}

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

func (m *Mock) Requested(r *http.Request) *Response {
	m.mutex.Lock()

	requestBody, err := readHTTPRequestBody(r)
	if err != nil {
		m.mutex.Unlock()
		m.fail("\nassert: httpmock: Failed to read requested body. Error: %v", err)
	}

	found, request := m.findExpectedRequest(r)

	if found < 0 {
		// expected request not found, but has already been requested with repeatable times
		if request != nil {
			m.mutex.Unlock()
			m.fail("\nassert: httpmock: The request has been called over %d times.\n\tEither do one more Mock.On(%q, %q), or remove extra request.", request.totalRequests, r.Method, r.URL.String())
		}

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

	// add the request
	nr := newRequest(m, r.Method, r.URL, requestBody)
	m.Requests = append(m.Requests, *nr)
	m.mutex.Unlock()

	return request.response
}

type matchCandidate struct {
	request   *Request
	mismatch  string
	diffCount int
}

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

func (m *Mock) AssertExpectations(t mock.TestingT) bool {
	if th, ok := t.(tHelper); ok {
		th.Helper()
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	var failedExpectations int

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
	path = u.String()

	m.mutex.Lock()
	defer m.mutex.Unlock()
	var actualRequests int
	for _, request := range m.requests() {
		if request.method != method {
			continue
		}

		ru := *request.url
		ru.User = nil
		ru.RawQuery = ""
		ru.Fragment = ""
		if ru.String() != path {
			continue
		}

		actualRequests++
	}

	return assert.Equal(t, expectedRequests, actualRequests)
}

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

func (m *Mock) checkExpectation(request *Request) (bool, string) {
	if (!m.checkWasRequested(request.method, request.url, request.body) && request.totalRequests == 0) || (request.repeatability > 0) {
		return false, fmt.Sprintf("FAIL:\t%s %s\n\t(%d) %s", request.method, request.url, len(request.body), bodyFragment(request.body))
	}
	return true, fmt.Sprintf("PASS:\t%s %s\n\t(%d) %s", request.method, request.url, len(request.body), bodyFragment(request.body))
}

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

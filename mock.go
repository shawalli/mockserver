package httpmock

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/stretchr/testify/mock"
)

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

// https://goplay.tools/snippet/_1Iu9dcSHKt
func (m *Mock) findExpectedRequest(actual *http.Request) (int, *Request) {
	var expectedRequest *Request
	for i, er := range m.ExpectedRequests {
		if er.method != AnyMethod && er.method != actual.Method {
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

package httpmock

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var (
	ErrReadBody = errors.New("error reading body")

	AnyMethod = "mock.AnyMethod"

	cmpoptSortMaps                  = cmpopts.SortMaps(func(a, b string) bool { return a < b })
	cmpoptSortSlices                = cmpopts.SortSlices(func(a, b string) bool { return a < b })
	cmpoptIgnoreURLRawQuery         = cmpopts.IgnoreFields(url.URL{}, "RawQuery")
	cmpoptIgnoreURLUnexportedFields = cmpopts.IgnoreUnexported(url.URL{})

	fmtMissing  = "(Missing)"
	fmtNotEqual = "!="
	fmtEqual    = "=="
)

// Request represents a HTTP request and is used for setting expectations,
// as well as recording activity.
type Request struct {
	parent *Mock

	// The HTTP method that was or will be requested.
	method string

	// The URL that was or will be requested, including query parameters and
	// fragment.
	url *url.URL

	// The body that was or will be requested.
	body []byte

	// Holds the parts of the response that should be returned when setting
	// this request is received.
	response *Response

	// The number of times to return the response when setting expectations.
	// 0 means to always return the value.
	repeatability int

	// Amount of times this request has been received.
	totalRequests int
}

func newRequest(parent *Mock, method string, URL *url.URL, body []byte) *Request {
	return &Request{
		parent: parent,
		method: method,
		url:    URL,
		body:   body,
	}
}

// lock is a convenience method to lock the parent Mock's mutex.
func (r *Request) lock() {
	r.parent.mutex.Lock()
}

// unlock is a convenience method to unlock the parent Mock's mutex.
func (r *Request) unlock() {
	r.parent.mutex.Unlock()
}

// Respond specifies the response arguments for the expectation.
//
//	Mock.On(http.GetMethod, "/some/path").Respond(http.StatusInternalServerError, nil)
func (r *Request) Respond(statusCode int, body []byte) *Response {
	resp := newResponse(
		r,
		statusCode,
		body,
	)

	r.lock()
	defer r.unlock()

	r.response = resp

	return resp
}

// RespondOK is a convenience method that sets the status code as 200 and
// the provided body.
//
//	Mock.On(http.GetMethod, "/some/path").RespondOK([]byte(`{"foo", "bar"}`)])
func (r *Request) RespondOK(body []byte) *Response {
	return r.Respond(http.StatusOK, body)
}

// RespondNoContent is a convenience method that sets the status code as 204.
//
//	Mock.On(http.MethodDelete, "/some/path/1234").RespondNoContent()
func (r *Request) RespondNoContent() *Response {
	return r.Respond(http.StatusNoContent, nil)
}

// Once indicates that the mock should only return the response once.
//
//	Mock.On(http.MethodDelete, "/some/path/1234").Once()
func (r *Request) Once() *Request {
	return r.Times(1)
}

// Twice indicates that the mock should only return the response twice.
//
//	Mock.On(http.MethodDelete, "/some/path/1234").Twice()
func (r *Request) Twice() *Request {
	return r.Times(2)
}

// Times indicates that the mock should only return the indicated number
// of times.
//
//	Mock.On(http.MethodDelete, "/some/path/1234").Times(5)
func (r *Request) Times(i int) *Request {
	r.lock()
	defer r.unlock()

	r.repeatability = i
	return r
}

// readHTTPRequestBody reads the body of a HTTP request and resets the
// request's body so that it may be read again afterward.
func readHTTPRequestBody(received *http.Request) ([]byte, error) {
	// Read request body and reset it for the next comparison
	body, err := io.ReadAll(received.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadBody, err)
	}
	received.Body.Close()
	received.Body = io.NopCloser(bytes.NewBuffer(body))

	return body, nil
}

// diffMissing is a convenience function to provide a standard string if a
// string is found to be empty.
func diffMissing(v string) (string, bool) {
	if v == "" {
		return fmtMissing, false
	}
	return v, true
}

// diffMethod detects differences between a Request's HTTP method and a HTTP
// request's HTTP method. It responds with a formatted string of the difference
// and the calculated number of differences.
func (r *Request) diffMethod(received *http.Request) (string, int) {
	var output string
	var differences int

	expected := r.method
	if r.method == "" {
		expected = fmtMissing
	} else if r.method == AnyMethod {
		expected = "(AnyMethod)"
	}

	actual := received.Method
	if received.Method == "" {
		actual = fmtMissing
	}

	if (r.method == AnyMethod && received.Method != "") || ((r.method == received.Method) && (r.method != "")) {
		output = fmt.Sprintf("\t%d: PASS:  %s == %s\n", 0, actual, expected)
	} else {
		output = fmt.Sprintf("\t%d: FAIL:  %s != %s\n", 0, actual, expected)
		differences++
	}

	return output, differences
}

// diffQuery detects differences between a Request's query parameters and a
// HTTP request's query parameters. It responds with a formatted string of the
// differences and the calculated number of differences.
//
// Query Logic:
//   - If Request query is empty, don't compare query parameters at all
//   - Otherwise, only compare query parameters found in Request; ignore query
//     parameters in HTTP request that are not enumerated in the Request.
func (r *Request) diffQuery(received *http.Request) (string, int) {
	var output string
	var differences int

	rQuery := r.url.Query()
	if len(rQuery) == 0 {
		return output, differences
	}

	oQuery := received.URL.Query()
	oFilteredQuery := url.Values{}
	for k := range rQuery {
		if v, ok := oQuery[k]; ok {
			oFilteredQuery[k] = v
		}
	}

	e, _ := diffMissing(rQuery.Encode())

	a, aok := diffMissing(oFilteredQuery.Encode())
	a2, a2ok := diffMissing(oQuery.Encode())
	if !aok && !a2ok {
		// No filtered-query or full-query available
		a2 = ""
	} else if !aok && a2ok {
		// No filtered-query available
		a2 = fmt.Sprintf(" ((%s))", a2)
	} else {
		// Filtered-query and full-query both available
		a2 = fmt.Sprintf(" (%s)", a2)
	}
	a = fmt.Sprintf("%s%s", a, a2)

	eq := fmtEqual
	if !cmp.Equal(rQuery, oFilteredQuery, cmpoptSortMaps, cmpoptSortSlices) {
		eq = fmtNotEqual
		differences++
	}

	output = fmt.Sprintf("\t\t     Query:  %s %s %s\n", a, eq, e)

	return output, differences
}

// diffURL detects differences between a Request's URL and an
// HTTP request's URL. It responds with a formatted string of the
// differences and the calculated number of differences.
//
// Ignored URL Fields:
//   - .Opaque
//   - .User
//   - .RawPath
//   - .OmitHost
//   - .ForceQuery
//   - .RawFragment
func (r *Request) diffURL(received *http.Request) (string, int) {
	var output string
	var differences int

	expected, eok := diffMissing(r.url.String())
	actual, aok := diffMissing(received.URL.String())
	if !eok || !aok {
		output = fmt.Sprintf("\t%d: FAIL:  %s == %s\n", 1, actual, expected)
		differences++
		return output, differences
	}

	var schemeFmt, hostFmt, pathFmt, queryFmt, fragmentFmt string

	e, eok := diffMissing(r.url.Scheme)
	a, aok := diffMissing(received.URL.Scheme)
	if eok || aok {
		eq := fmtNotEqual
		if cmp.Equal(r.url.Scheme, received.URL.Scheme) {
			eq = fmtEqual
		}
		schemeFmt = fmt.Sprintf("\t\t    Scheme:  %s %s %s\n", a, eq, e)
	}

	e, eok = diffMissing(r.url.Host)
	a, aok = diffMissing(received.URL.Host)
	if eok || aok {
		eq := fmtNotEqual
		if cmp.Equal(r.url.Host, received.URL.Host) {
			eq = fmtEqual
		}
		hostFmt = fmt.Sprintf("\t\t      Host:  %s %s %s\n", a, eq, e)
	}

	e, eok = diffMissing(r.url.Path)
	a, aok = diffMissing(received.URL.Path)
	if eok || aok {
		eq := fmtNotEqual
		if cmp.Equal(r.url.Path, received.URL.Path) {
			eq = fmtEqual
		}
		pathFmt = fmt.Sprintf("\t\t      Path:  %s %s %s\n", a, eq, e)
	}

	queryFmt, queryDifferences := r.diffQuery(received)

	e, eok = diffMissing(r.url.Fragment)
	a, aok = diffMissing(received.URL.Fragment)
	if eok || aok {
		eq := fmtNotEqual
		if cmp.Equal(r.url.Fragment, received.URL.Fragment) {
			eq = fmtEqual
		}
		fragmentFmt = fmt.Sprintf("\t\t  Fragment:  %s %s %s\n", a, eq, e)
	}

	if cmp.Equal(*r.url, *received.URL, cmpoptIgnoreURLRawQuery, cmpoptIgnoreURLUnexportedFields) && queryDifferences == 0 {
		output = fmt.Sprintf("\t%d: PASS:  %s == %s\n", 1, received.URL.String(), r.url.String())
		output += schemeFmt
		output += hostFmt
		output += pathFmt
		output += queryFmt
		output += fragmentFmt
	} else {
		output = fmt.Sprintf("\t%d: FAIL:  %s == %s\n", 1, received.URL.String(), r.url.String())
		output += schemeFmt
		output += hostFmt
		output += pathFmt
		output += queryFmt
		output += fragmentFmt

		differences++
	}

	return output, differences
}

// trimBody concatenates a body larger than 1024 bytes and appends an ellipses.
func trimBody(body []byte) string {
	o := fmtMissing
	olen := len(body)
	if olen > 1024 {
		o = fmt.Sprintf("%s...", string(body[:1024]))
	} else if olen > 0 {
		o = string(body)
	}
	return o
}

// diffBody detects differences between a Request's body and a HTTP request's
// body. It responds with a formatted string of the differences and the
// calculated number of differences.
func (r *Request) diffBody(received *http.Request) (string, int) {
	var output string
	var differences int

	otherBody, err := readHTTPRequestBody(received)
	if err != nil {
		return err.Error(), 1
	}

	e := trimBody(r.body)
	elen := len(r.body)
	a := trimBody(otherBody)
	alen := len(otherBody)

	eq := fmtEqual
	if elen == 0 && alen == 0 {
		output = fmt.Sprintf("\t%d: PASS:  (0) %s == (0) %s\n", 2, e, a)
		return output, differences
	} else if (elen == 0 && alen > 0) || (elen > 0 && alen == 0) || !cmp.Equal(string(r.body), string(otherBody)) {
		output = fmt.Sprintf("\t%d: FAIL:\n", 2)
		eq = fmtNotEqual
		differences++
	} else {
		output = fmt.Sprintf("\t%d: PASS:\n", 2)
	}
	output = fmt.Sprintf("%s\t\t (%d) %s\n\n\t\t    %s\n\n\t\t (%d) %s\n", output, elen, e, eq, alen, a)

	return output, differences
}

// diff detects differences between a Request and a HTTP request. It responds
// with a formatted string of the differences and the calculated number of
// differences.
func (r *Request) diff(received *http.Request) (string, int) {
	output := "\n"
	var differences int

	o, d := r.diffMethod(received)
	output += o
	differences += d

	o, d = r.diffURL(received)
	output += o
	differences += d

	o, d = r.diffBody(received)
	output += o
	differences += d

	return output, differences
}

// String computes a formatted string representing a Request.
func (r *Request) String() string {
	var output []string

	e := r.method
	if r.method == "" {
		e = fmtMissing
	} else if r.method == AnyMethod {
		e = "(AnyMethod)"
	}
	output = append(output, fmt.Sprintf("Method: %s", e))

	if e = r.url.String(); e == "" {
		output = append(output, fmt.Sprintf("URL: %s", fmtMissing))
	} else {
		output = append(output, fmt.Sprintf("URL: %s", e))

		e, eok := diffMissing(r.url.Scheme)
		if !eok {
			e = fmtMissing
		}
		output = append(output, fmt.Sprintf("\tScheme: %s", e))

		e, eok = diffMissing(r.url.Host)
		if !eok {
			e = fmtMissing
		}
		output = append(output, fmt.Sprintf("\tHost: %s", e))

		e, eok = diffMissing(r.url.Path)
		if !eok {
			e = fmtMissing
		}
		output = append(output, fmt.Sprintf("\tPath: %s", e))

		e, eok = diffMissing(r.url.RawQuery)
		if !eok {
			e = fmtMissing
		}
		output = append(output, fmt.Sprintf("\tQuery: %s", e))

		e, eok = diffMissing(r.url.Fragment)
		if !eok {
			e = fmtMissing
		}
		output = append(output, fmt.Sprintf("\tFragment: %s", e))
	}

	e = trimBody(r.body)
	elen := len(r.body)
	output = append(output, fmt.Sprintf("Body: (%d) %s", elen, e))

	return strings.Join(output, "\n")
}

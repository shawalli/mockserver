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

type Request struct {
	parent *Mock

	method string
	url    *url.URL
	body   []byte

	response *Response

	repeatability int
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

func (r *Request) lock() {
	r.parent.mutex.Lock()
}

func (r *Request) unlock() {
	r.parent.mutex.Unlock()
}

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

func (r *Request) RespondOK(body []byte) *Response {
	return r.Respond(http.StatusOK, body)
}

func (r *Request) RespondNoContent() *Response {
	return r.Respond(http.StatusNoContent, nil)
}

func (r *Request) Once() *Request {
	return r.Times(1)
}

func (r *Request) Twice() *Request {
	return r.Times(2)
}

func (r *Request) Times(i int) *Request {
	r.lock()
	defer r.unlock()

	r.repeatability = i
	return r
}

func readHTTPRequestBody(r *http.Request) ([]byte, error) {
	// Read request body and reset it for the next comparison
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadBody, err)
	}
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	return body, nil
}

func diffMissing(v string) (string, bool) {
	if v == "" {
		return fmtMissing, false
	}
	return v, true
}

func (r *Request) diffMethod(other *http.Request) (string, int) {
	var output string
	var differences int

	expected := r.method
	if r.method == "" {
		expected = fmtMissing
	} else if r.method == AnyMethod {
		expected = "(AnyMethod)"
	}

	actual := other.Method
	if other.Method == "" {
		actual = fmtMissing
	}

	if (r.method == AnyMethod && other.Method != "") || ((r.method == other.Method) && (r.method != "")) {
		output = fmt.Sprintf("\t%d: PASS:  %s == %s\n", 0, actual, expected)
	} else {
		output = fmt.Sprintf("\t%d: FAIL:  %s != %s\n", 0, actual, expected)
		differences++
	}

	return output, differences
}

// Query logic:
// - if request query is empty, don't compare queries
// - if request query is not empty, only compare keys found in request query
func (r *Request) diffQuery(other *http.Request) (string, int) {
	var output string
	var differences int

	rQuery := r.url.Query()
	if len(rQuery) == 0 {
		return output, differences
	}

	oQuery := other.URL.Query()
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

func (r *Request) diffURL(other *http.Request) (string, int) {
	var output string
	var differences int

	expected, eok := diffMissing(r.url.String())
	actual, aok := diffMissing(other.URL.String())
	if !eok || !aok {
		output = fmt.Sprintf("\t%d: FAIL:  %s == %s\n", 1, actual, expected)
		differences++
		return output, differences
	}

	var schemeFmt, hostFmt, pathFmt, queryFmt, fragmentFmt string

	e, eok := diffMissing(r.url.Scheme)
	a, aok := diffMissing(other.URL.Scheme)
	if eok || aok {
		eq := fmtNotEqual
		if cmp.Equal(r.url.Scheme, other.URL.Scheme) {
			eq = fmtEqual
		}
		schemeFmt = fmt.Sprintf("\t\t    Scheme:  %s %s %s\n", a, eq, e)
	}

	e, eok = diffMissing(r.url.Host)
	a, aok = diffMissing(other.URL.Host)
	if eok || aok {
		eq := fmtNotEqual
		if cmp.Equal(r.url.Host, other.URL.Host) {
			eq = fmtEqual
		}
		hostFmt = fmt.Sprintf("\t\t      Host:  %s %s %s\n", a, eq, e)
	}

	e, eok = diffMissing(r.url.Path)
	a, aok = diffMissing(other.URL.Path)
	if eok || aok {
		eq := fmtNotEqual
		if cmp.Equal(r.url.Path, other.URL.Path) {
			eq = fmtEqual
		}
		pathFmt = fmt.Sprintf("\t\t      Path:  %s %s %s\n", a, eq, e)
	}

	queryFmt, queryDifferences := r.diffQuery(other)

	e, eok = diffMissing(r.url.Fragment)
	a, aok = diffMissing(other.URL.Fragment)
	if eok || aok {
		eq := fmtNotEqual
		if cmp.Equal(r.url.Fragment, other.URL.Fragment) {
			eq = fmtEqual
		}
		fragmentFmt = fmt.Sprintf("\t\t  Fragment:  %s %s %s\n", a, eq, e)
	}

	if cmp.Equal(*r.url, *other.URL, cmpoptIgnoreURLRawQuery, cmpoptIgnoreURLUnexportedFields) && queryDifferences == 0 {
		output = fmt.Sprintf("\t%d: PASS:  %s == %s\n", 1, other.URL.String(), r.url.String())
		output += schemeFmt
		output += hostFmt
		output += pathFmt
		output += queryFmt
		output += fragmentFmt
	} else {
		output = fmt.Sprintf("\t%d: FAIL:  %s == %s\n", 1, other.URL.String(), r.url.String())
		output += schemeFmt
		output += hostFmt
		output += pathFmt
		output += queryFmt
		output += fragmentFmt

		differences++
	}

	return output, differences
}

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

func (r *Request) diffBody(other *http.Request) (string, int) {
	var output string
	var differences int

	// Read request body and reset it for the next comparison
	otherBody, err := readHTTPRequestBody(other)
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

func (r *Request) diff(other *http.Request) (string, int) {
	output := "\n"
	var differences int

	o, d := r.diffMethod(other)
	output += o
	differences += d

	o, d = r.diffURL(other)
	output += o
	differences += d

	o, d = r.diffBody(other)
	output += o
	differences += d

	return output, differences
}

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

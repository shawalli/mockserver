# httpmock - `testify` for HTTP Requests

This package provides a mocking interface in the spirit of [`stretchr/testify/mock`](https://github.com/stretchr/testify/tree/master/mock) for HTTP requests.

```go
package yours

import (
	"net/http"
	"testing"
	"github.com/shawalli/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestSomething(t *testing.T) {
	// Setup default test server and handler to log requests and return expected responses
	// You may also create your own test server, handler, and mock to manage this
	ts := httpmock.NewServer()
	defer ts.Close()
	// Set as recoverable to log panics rather than propagate out from the server
	// goroutine to the parent process
	ts.Recoverable()

	// Configure request mocks
	expectBearerToken := func(received *http.Request) (output string, differences int) {
		if _, ok := received.Header["Authorization"]; !ok {
			output = "FAIL:  missing header Authorization"
			differences = 1
			return
		}
		val := received.Header.Get("Authorization")
		if !strings.HasPrefix(val, "Bearer ") {
			output = fmt.Sprintf("FAIL:  header Authorization=%v != Bearer", val)
			differences = 1
			return
		}
		output = fmt.Sprintf("PASS:  header Authorization=%v == Bearer", val)
		return
	}
	ts.On(http.MethodPatch, "/foo/1234", []byte(`{"bar": "baz"}`)).
		Matches(expectBearerToken).
		RespondOK([]byte(`Success!`)).
		Once()

	// Test application code
	tc := ts.Client()
	req, err := http.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("%s/foo/1234", ts.URL),
		io.NopCloser(strings.NewReader(`{"bar": "baz"}`)),
	)
	req.Header.Add("Authorization", "Bearer jkel3450d")
	resp, err := tc.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request! %v", err)
	}

	// Assert application expectations
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body! %v", err)
	}
	assert.Equal(t, "Success!", string(respBody))

	// Assert httpmock expectations
	ts.Mock.AssertExpectations(t)
	ts.Mock.AssertNumberOfRequests(t, http.MethodPatch, "/foo/1234", 1)
}
```

You can also use `Mock` directly and implement your own test server. To do so,
you should wire up your handler so that the request is passed to
`Mock.Requested(r)`, and respond using the returned `Response`'s `Write(w)`
method.

## Installation

To install `httpmock`, use `go get`:

```shell
go get github.com/shawalli/httpmock
```

## Todo

- [x] Extend `httptest.Server` to provide a single implementation
- [ ] Request URL matcher
- [x] Request matcher functions
- [x] Request header matching (implemented with matcher functions feature)

## License

This project is licensed under the terms of the MIT license.

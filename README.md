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
			output = fmt.Sprintf("FAIL:  header Authorization: %q != Bearer", val)
			differences = 1
			return
		}
		output = fmt.Sprintf("PASS:  header Authorization: %q == Bearer", val)
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

## Features

### `httpmock.Mock`

#### On

The `On()` method is the primary mechanism for configuring mocks. It is primarily designed for `httpmock.Mock`. To support common chaining patterns found in both `httpmock` and `testify/mock`, the `On()` method may also be found on common structs, such as `httpmock.Server` and `httpmock.Response`. In these cases, the `On()` method is just a convenience wrapper to register an expected request against the underlying `httpmock.Mock` object.

```go
Mock.On(http.MethodPost, "/some/path/1234").RespondOK()
Server.On(http.MethodPost, "/some/path/1234").RespondOK()
Mock.
	On(http.MethodPost, "/some/path/1234").
	RespondOK().
	On(http.MethodDelete, "/some/path/1234").
	RespondNoContent()
```

#### AnyMethod

Use `httpmock.AnyMethod` to indicate the expected request can contain any valid HTTP method.

```go
Mock.On(httpmock.AnyMethod, "/some/path", nil)
```

#### AnyBody

Use `httpmock.AnyBody` to indicate the expected request can contain any body, or no body at all.

```go
Mock.On(http.MethodPost, "/some/path/1234", httpmock.AnyBody)
```

### `httpmock.Request`

#### Matches

Use `httpmock.Request.Matches()` to perform more complex matching for an expected request. For example, expect that a request should have an Authorization header that uses a bearer token.

```go
expectBearerToken := func(received *http.Request) (output string, differences int) {
	if _, ok := received.Header["Authorization"]; !ok {
		output = "FAIL:  missing header Authorization"
		differences = 1
		return
	}
	val := received.Header.Get("Authorization")
	if !strings.HasPrefix(val, "Bearer ") {
		output = fmt.Sprintf("FAIL:  header Authorization: %q != Bearer", val)
		differences = 1
		return
	}
	output = fmt.Sprintf("PASS:  header Authorization: %q == Bearer", val)
	return
}
Mock.On(http.MethodPost, "/some/path/1234", nil).Matches(expectBearerToken)
```

**On Formatting**: For readability, try to conform to the following pattern when formatting your output:

```
FAIL:  <actual> != <expected>
PASS:  <actual> == <expected>
```

The diff formatting will take care of tabs, newlines, and match-indices for you, so please do not include those formatters.

#### Times, Once, Twice

Just like `testify/mock`, `httpmock` assumes that an expected request may be matched in perpetuity by default. This
assumption may be altered with the `httpmock.Request.Times()` method. `Times()` takes an integer that indicates the
number of times an expected request should match. After the configured number of times, an expected request will not
match even if it would match otherwise.

Additionally, two convenience methods are available to simplify common configurations: `Once()` and `Twice()`. They
behave as one would expect.

```go
Mock.On(http.MethodDelete, "/some/path/1234").Once().RespondNoContent()
Mock.On(http.MethodDelete, "/some/path/1234").RespondNoContent().Once()
```

**Note**: To support chaining, these methods may also be found on the `httpmock.Response` struct as convenience wrappers into the underlying `httpmock.Request` object.

#### Respond, RespondOK, RespondNoContent

`httpmock` provides a basic method to register desired responses to a request with the `httpmock.Request.Respond()`
method. It takes a status code and response body.

Additionally, two convenience methods are available to simplify common patterns:

- `RespondOK()` - This method responds with a 200 status code and allows for a custom body.
- `RespondNoContent()` - This responds with a 204 status code and does not take a body, since 204 indicates that the
response contains no content.

```go
Mock.On(http.MethodPost, "/some/path", []byte("spam")).RespondOK([]byte(`{"id": "1234"}`))
Mock.On(http.MethodDelete, "/some/path/1234").RespondNoContent()
Mock.On(http.MethodGet, "/some/path/1234").Respond(http.StatusNotFound, nil)
Mock.On(http.MethodGet, "/some/path/1234").Respond(http.StatusNotFound, []byte(`{"error": "path resource not found"}`))
```

In the future, more convenience methods may be added if they are common, clearly defined, and enhance the readability
and simplification of the mock response configuration.

### `httpmock.Server`

#### Recoverable, IsRecoverable

`httpmock.Server` is a glorified version of `httptest.Server` with a default handler. With both server types, the
server runs as a goroutine. One can use `Recoverable()` to indicate that an unmatched request should not cause the
server to panic outside of the server goroutine and into the main process.

If writing a custom handler, the handler should react to a panic based on the server's `IsRecoverable()` response.

## Installation

To install `httpmock`, use `go get`:

```shell
go get github.com/shawalli/httpmock
```

## Troubleshooting

### Nil pointer dereference while reading `http.Request`

If using `httpmock.Server` or `httptest.Server`, an `http.Request` with no body must use `http.NoBody` instead of
`nil`. This is due to the fact that the test server is not actually sending and receiving a request, but rather mocking
a request. Using `http.NoBody` indicates to the `net/http` package that the request has no body but is still a valid
`io.Reader`.

## Todo

- [x] Extend `httptest.Server` to provide a single implementation
- [ ] Request URL matcher
- [x] Request matcher functions
- [x] Request header matching (implemented with matcher functions feature)
- [ ] Response custom function

## License

This project is licensed under the terms of the MIT license.

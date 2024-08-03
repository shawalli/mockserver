# httpmock - `testify` for HTTP Requests

This package provides a mocking interface in the spirit of [`stretchr/testify/mock`](https://github.com/stretchr/testify/tree/master/mock) for HTTP requests.

```go
package yours

import (
    "net/http"
    "testing"
    "github.com/shawalli/httpmock"
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
    s.Mock.On(
        http.MethodPatch,
        fmt.Sprintf("%s/foo/1234?preview=true", server.URL),
        []byte{`{"bar": "baz"}`}).
    RespondOK([]byte(`Success!`)).
    Once()

    // Test application code
    c := ts.Client()
    res, err := c.Patch("/foo/1234?preview=true", io.NopCloser(strings.NewReader(`{"bar": "baz"}`)))

    // Assert application code
    ...

    // Assert httpmock expectations
    s.Mock.AssertExpectations(t)
    s.Mock.AssertNumberOfRequests(t, http.MethodPatch, "/foo/1234", 1)
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
- [ ] Request header matching

## License

This project is licensed under the terms of the MIT license.

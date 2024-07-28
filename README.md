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
    // Optionally use t for failing instead of panic
    m := new(httpmock.Mock).Test(t)

    // Setup test server to log requests anre return expected responses
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        request := m.Requested(r)
        request.WriteResponse(w)
    })
    defer ts.Close()

    // Configure request mocks
    m.On(http.MethodPut, fmt.Sprintf("%s/foo/1234", server.URL, []byte{`{"bar": "baz"}`}).ReturnStatusNoContent().Once()

    // Application code
    c := server.Client()
    res, err := c.Put("/foo/1234", io.NopCloser(strings.NewReader("{{"bar": "baz"}}")))

    // Assert application code
    ...

    // Assert httpmock
    ...
}
```

## Installation

To install `httpmock`, use `go get`:

```shell
go get github.com/shawalli/httpmock
```

## Todo

- [ ] Extend `httptest.Server` to provide a single implementation
- [ ] Request header matching
- [ ] Request URL matcher

## License

This project is licensed under the terms of the MIT license.

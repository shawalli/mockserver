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
        response := m.Requested(r)
        if _, err := response.Write(w); err != nil {
            t.Fatalf("Failed to write test-server response: %v", err)
        }
    })
    defer ts.Close()

    // Configure request mocks
    m.On(
        http.MethodPatch,
        fmt.Sprintf("%s/foo/1234?preview=true", server.URL),
        []byte{`{"bar": "baz"}`},
    ).Respond(
        http.StatusOK,
        []byte(`Success!`),
    ).Once()

    // Application code
    c := server.Client()
    res, err := c.Patch("/foo/1234?preview=true", io.NopCloser(strings.NewReader(`Success!`)))

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

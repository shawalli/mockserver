package httpmock

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

// Server simplifies the orchestration of a [Mock] inside a handler and server.
// It wraps the stdlib [httptest.Server] implementation and provides a
// handler to log requests and write configured responses.
type Server struct {
	*httptest.Server

	Mock *Mock

	// Whether or not panics should be caught in the server goroutine or
	// allowed to propagate to the parent process. If false, the panic will be
	// printed and a 404 will be returned to the client.
	ignorePanic bool
}

// makeHandler creates a standard [http.HandlerFunc] that may be used by a
// regular or TLS [Server] to log requests and write configured responses.
func makeHandler(s *Server) http.HandlerFunc {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rc := recover(); rc != nil {
					if s.IsRecoverable() {
						fmt.Printf("%v\n", rc)

						w.WriteHeader(http.StatusNotFound)
					} else {
						panic(rc)
					}

				}
			}()

			response := s.Mock.Requested(r)
			if _, err := response.Write(w, r); err != nil {
				s.Mock.fail("failed to write response for request:\n%s\nwith error: %v", response.parent.String(), err)
			}
		},
	)
}

// NewServer creates a new [Server] and associated [Mock].
func NewServer() *Server {
	s := &Server{Mock: new(Mock)}
	s.Server = httptest.NewServer(http.HandlerFunc(makeHandler(s)))

	return s
}

// NewServer creates a new [Server], configured for TLS, and associated [Mock].
func NewTLSServer() *Server {
	s := &Server{Mock: new(Mock)}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(makeHandler(s)))

	return s
}

// NotRecoverable sets a [Server] as not recoverable, so that panics are allowed
// to propagate to the main process. With the default handler, panics are caught
// and printed to stdout, with a final 404 returned to the client.
//
// 404 was chosen rather than 500 due to panics almost always occurring when a
// matching [Request] cannot be found.
func (s *Server) NotRecoverable() *Server {
	s.ignorePanic = true
	return s
}

// IsRecoverable returns whether or not the [Server] is considered recoverable.
func (s *Server) IsRecoverable() bool {
	return !s.ignorePanic
}

// On is a convenience method to invoke the [Mock.On] method.
//
//	Server.On(http.MethodDelete, "/some/path/1234")
func (s *Server) On(method string, URL string, body []byte) *Request {
	return s.Mock.On(method, URL, body)
}

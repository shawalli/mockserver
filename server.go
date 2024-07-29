package httpmock

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

type Server struct {
	*httptest.Server

	Mock *Mock

	recoverable bool
}

func makeHandler(s *Server) http.HandlerFunc {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rc := recover(); rc != nil {
					if s.recoverable {
						fmt.Printf("%v\n", rc)

						w.WriteHeader(http.StatusNotFound)
					} else {
						panic(rc)
					}

				}
			}()

			response := s.Mock.Requested(r)
			if _, err := response.Write(w); err != nil {
				s.Mock.fail(fmt.Sprintf("failed to write response for request:\n%s\nwith error: %v", response.parent.String(), err))
			}
		},
	)
}

func NewServer() *Server {
	s := &Server{Mock: new(Mock)}
	s.Server = httptest.NewServer(http.HandlerFunc(makeHandler(s)))

	return s
}

func NewTLSServer() *Server {
	s := &Server{Mock: new(Mock)}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(makeHandler(s)))

	return s
}

func (s *Server) Recoverable() *Server {
	s.recoverable = true
	return s
}

func (s *Server) On(method string, URL string, body []byte) *Request {
	return s.Mock.On(method, URL, body)
}

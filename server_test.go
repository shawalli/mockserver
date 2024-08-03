package httpmock

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewServer(t *testing.T) {
	// Test
	s := NewServer()
	defer s.Close()

	// Assertions
	assert.NotNil(t, s.Mock)
	assert.NotNil(t, s.Server)
	if s.Server != nil {
		// Equivalent to s != TLSServer
		assert.Nil(t, s.Server.TLS)
		// Equivalent to the server having been Start()'ed already
		assert.NotEmpty(t, s.Server.URL)
	}
}

func Test_NewTLSServer(t *testing.T) {
	// Test
	s := NewTLSServer()

	// Assertions
	assert.NotNil(t, s.Mock)
	assert.NotNil(t, s.Server)
	if s.Server != nil {
		// Equivalent to s == TLSServer
		assert.NotNil(t, s.Server.TLS)
		// Equivalent to the server having been Start()'ed already
		assert.NotEmpty(t, s.Server.URL)
	}
}

func TestServer_handler_NoMatch(t *testing.T) {
	// Setup
	s := NewServer().Recoverable()
	defer s.Close()
	s.On(http.MethodGet, "/foo/1234", nil).RespondOK([]byte(`Hello World!`))

	// Test
	request := mustNewRequest(
		http.NewRequest(
			http.MethodDelete,
			fmt.Sprintf("%s/foo/1234", s.URL),
			http.NoBody,
		),
	)
	got, err := s.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}

	// Assertions
	assert.NotNil(t, got)
	assert.Equal(t, http.StatusNotFound, got.StatusCode)
	s.Mock.AssertNotRequested(t, http.MethodDelete, fmt.Sprintf("%s/foo/1234", s.URL), nil)
}

func TestServer_handler_AssertRequested(t *testing.T) {
	// Setup
	s := NewServer().Recoverable()
	defer s.Close()
	s.On(http.MethodGet, "/foo/1234", nil).RespondOK([]byte(`Hello World!`))

	// Test
	request := mustNewRequest(
		http.NewRequest(
			http.MethodGet,
			fmt.Sprintf("%s/foo/1234", s.URL),
			http.NoBody,
		),
	)
	got, err := s.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}

	gotBody, err := io.ReadAll(got.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer got.Body.Close()

	// Assertions
	assert.NotNil(t, got)
	assert.Equal(t, http.StatusOK, got.StatusCode)
	assert.Equal(t, []byte(`Hello World!`), gotBody)
	s.Mock.AssertRequested(t, http.MethodGet, "/foo/1234", nil)
}

func TestServer_handler_AssertNotRequested(t *testing.T) {
	// Setup
	s := NewServer().Recoverable()
	defer s.Close()
	s.On(http.MethodGet, "/foo/1234", nil).RespondOK([]byte(`Hello World!`))

	// Test and Assertions
	s.Mock.AssertNotRequested(t, http.MethodDelete, fmt.Sprintf("%s/foo/1234", s.URL), nil)
}

func TestServer_handler_AssertExpectations(t *testing.T) {
	// Setup
	s := NewServer().Recoverable()
	defer s.Close()
	s.On(http.MethodGet, "/foo/1234", nil).Respond(http.StatusNotFound, nil).Twice()
	s.On(http.MethodPut, "/foo/1234", []byte(`Hello World!`)).RespondNoContent()
	s.On(http.MethodGet, "/foo/1234", nil).RespondOK([]byte(`Hello World!`)).Once()
	s.On(http.MethodDelete, "/foo/1234", nil).RespondNoContent()
	s.On(http.MethodGet, "/foo/1234", nil).Respond(http.StatusNotFound, nil)

	// Test and Assertions
	getRequest1 := mustNewRequest(http.NewRequest(http.MethodGet, fmt.Sprintf("%s/foo/1234", s.URL), http.NoBody))
	got, err := s.Client().Do(getRequest1)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	assert.Equal(t, http.StatusNotFound, got.StatusCode)

	getRequest2 := mustNewRequest(http.NewRequest(http.MethodGet, fmt.Sprintf("%s/foo/1234", s.URL), http.NoBody))
	got, err = s.Client().Do(getRequest2)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	assert.Equal(t, http.StatusNotFound, got.StatusCode)

	putRequest1 := mustNewRequest(http.NewRequest(http.MethodPut, fmt.Sprintf("%s/foo/1234", s.URL), strings.NewReader(`Hello World!`)))
	got, err = s.Client().Do(putRequest1)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	assert.Equal(t, http.StatusNoContent, got.StatusCode)

	getRequest3 := mustNewRequest(http.NewRequest(http.MethodGet, fmt.Sprintf("%s/foo/1234", s.URL), http.NoBody))
	got, err = s.Client().Do(getRequest3)
	if err != nil {
		t.Fatal(err)
	}
	gotBody, err := io.ReadAll(got.Body)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	assert.Equal(t, []byte(`Hello World!`), gotBody)
	assert.Equal(t, http.StatusOK, got.StatusCode)

	deleteRequest1 := mustNewRequest(http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/foo/1234", s.URL), http.NoBody))
	got, err = s.Client().Do(deleteRequest1)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	assert.Equal(t, http.StatusNoContent, got.StatusCode)

	getRequest4 := mustNewRequest(http.NewRequest(http.MethodGet, fmt.Sprintf("%s/foo/1234", s.URL), http.NoBody))
	got, err = s.Client().Do(getRequest4)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	assert.Equal(t, http.StatusNotFound, got.StatusCode)

	s.Mock.AssertExpectations(t)
}

func TestServer_handler_AssertNumberOfRequests(t *testing.T) {
	// Setup
	s := NewServer().Recoverable()
	defer s.Close()
	s.On(http.MethodGet, "/foo/1234", nil).Respond(http.StatusNotFound, nil).Twice()
	s.On(http.MethodPut, "/foo/1234", []byte(`Hello World!`)).RespondNoContent()
	s.On(http.MethodGet, "/foo/1234", nil).RespondOK([]byte(`Hello World!`)).Once()
	s.On(http.MethodDelete, "/foo/1234", nil).RespondNoContent()
	s.On(http.MethodDelete, "/bars", nil).Respond(http.StatusBadRequest, nil)
	s.On(http.MethodGet, "/foo/1234", nil).Respond(http.StatusNotFound, nil)

	// Test and Assertions
	getRequest1 := mustNewRequest(http.NewRequest(http.MethodGet, fmt.Sprintf("%s/foo/1234", s.URL), http.NoBody))
	got, err := s.Client().Do(getRequest1)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	assert.Equal(t, http.StatusNotFound, got.StatusCode)

	getRequest2 := mustNewRequest(http.NewRequest(http.MethodGet, fmt.Sprintf("%s/foo/1234", s.URL), http.NoBody))
	got, err = s.Client().Do(getRequest2)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	assert.Equal(t, http.StatusNotFound, got.StatusCode)

	putRequest1 := mustNewRequest(http.NewRequest(http.MethodPut, fmt.Sprintf("%s/foo/1234", s.URL), strings.NewReader(`Hello World!`)))
	got, err = s.Client().Do(putRequest1)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	assert.Equal(t, http.StatusNoContent, got.StatusCode)

	getRequest3 := mustNewRequest(http.NewRequest(http.MethodGet, fmt.Sprintf("%s/foo/1234", s.URL), http.NoBody))
	got, err = s.Client().Do(getRequest3)
	if err != nil {
		t.Fatal(err)
	}
	gotBody, err := io.ReadAll(got.Body)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	assert.Equal(t, []byte(`Hello World!`), gotBody)
	assert.Equal(t, http.StatusOK, got.StatusCode)

	deleteRequest1 := mustNewRequest(http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/foo/1234", s.URL), http.NoBody))
	got, err = s.Client().Do(deleteRequest1)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	assert.Equal(t, http.StatusNoContent, got.StatusCode)

	deleteRequest2 := mustNewRequest(http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/bars", s.URL), http.NoBody))
	got, err = s.Client().Do(deleteRequest2)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	assert.Equal(t, http.StatusBadRequest, got.StatusCode)

	getRequest4 := mustNewRequest(http.NewRequest(http.MethodGet, fmt.Sprintf("%s/foo/1234", s.URL), http.NoBody))
	got, err = s.Client().Do(getRequest4)
	if err != nil {
		t.Fatal(err)
	}
	got.Body.Close()
	assert.Equal(t, http.StatusNotFound, got.StatusCode)

	s.Mock.AssertNumberOfRequests(t, http.MethodGet, "/foo/1234", 4)
	s.Mock.AssertNumberOfRequests(t, http.MethodPut, "/foo/1234", 1)
	s.Mock.AssertNumberOfRequests(t, http.MethodDelete, "/bars", 1)
	s.Mock.AssertNumberOfRequests(t, http.MethodDelete, "/foo/1234", 1)
}

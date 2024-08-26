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

func TestServer_NotRecoverable(t *testing.T) {
	// Setup
	s := NewServer()
	defer s.Close()

	// Test
	s.NotRecoverable()

	// Assert
	assert.True(t, s.ignorePanic)
}

func TestServer_defaultHandler_NoMatch(t *testing.T) {
	// Setup
	s := NewServer()
	defer s.Close()
	s.On(http.MethodGet, "/foo/1234", nil).RespondOK([]byte(testBody))

	// Test
	test := mustNewRequest(http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/foo/1234", s.URL), http.NoBody))
	got, err := s.Client().Do(test)
	if err != nil {
		t.Fatal(err)
	}

	// Assertions
	assert.NotNil(t, got)
	assert.Equal(t, http.StatusNotFound, got.StatusCode)
	s.Mock.AssertNotRequested(t, http.MethodDelete, fmt.Sprintf("%s/foo/1234", s.URL), nil)
}

func TestServer_defaultHandler_AssertRequested(t *testing.T) {
	// Setup
	s := NewServer()
	defer s.Close()
	s.On(http.MethodGet, "/foo/1234", nil).RespondOK([]byte(testBody))

	// Test
	test := mustNewRequest(http.NewRequest(http.MethodGet, fmt.Sprintf("%s/foo/1234", s.URL), http.NoBody))
	got, err := s.Client().Do(test)
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
	assert.Equal(t, []byte(testBody), gotBody)
	s.Mock.AssertRequested(t, http.MethodGet, "/foo/1234", nil)
}

func TestServer_defaultHandler_AssertNotRequested(t *testing.T) {
	// Setup
	s := NewServer()
	defer s.Close()
	s.On(http.MethodGet, "/foo/1234", nil).RespondOK([]byte(testBody))

	// Test and Assertions
	s.Mock.AssertNotRequested(t, http.MethodDelete, fmt.Sprintf("%s/foo/1234", s.URL), nil)
}

func TestServer_defaultHandler_AssertExpectations(t *testing.T) {
	// Setup
	s := NewServer()
	defer s.Close()
	s.On(http.MethodGet, "/foo/1234", nil).Respond(http.StatusNotFound, nil).Twice()
	s.On(http.MethodPut, "/foo/1234", []byte(testBody)).RespondNoContent()
	s.On(http.MethodGet, "/foo/1234", nil).RespondOK([]byte(testBody)).Once()
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

	putRequest1 := mustNewRequest(http.NewRequest(http.MethodPut, fmt.Sprintf("%s/foo/1234", s.URL), strings.NewReader(testBody)))
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
	assert.Equal(t, []byte(testBody), gotBody)
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

func TestServer_defaultHandler_AssertNumberOfRequests(t *testing.T) {
	// Setup
	s := NewServer()
	defer s.Close()
	s.On(http.MethodGet, "/foo/1234", nil).Respond(http.StatusNotFound, nil).Twice()
	s.On(http.MethodPut, "/foo/1234", []byte(testBody)).RespondNoContent()
	s.On(http.MethodGet, "/foo/1234", nil).RespondOK([]byte(testBody)).Once()
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

	putRequest1 := mustNewRequest(http.NewRequest(http.MethodPut, fmt.Sprintf("%s/foo/1234", s.URL), strings.NewReader(testBody)))
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
	assert.Equal(t, []byte(testBody), gotBody)
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

// TestSomething is the example given in the documentation.
//
// Let's keep it as a real test to ensure it actually works!
func TestSomething(t *testing.T) {
	// Setup default test server and handler to log requests and return expected responses.
	// You may also create your own test server, handler, and mock to manage this.
	ts := NewServer()
	defer ts.Close()

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
	if err != nil {
		t.Fatalf("Failed to create request! %v", err)
	}
	req.Header.Add("Authorization", "Bearer jkel3450d")
	resp, err := tc.Do(req)
	if err != nil {
		t.Fatalf("Failed to do request! %v", err)
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

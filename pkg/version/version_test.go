// version package unit tests
package version

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPHandlerNonGET(t *testing.T) {
	req, err := http.NewRequest("POST", "/version", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HTTPHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestHTTPHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/version", nil)
	if err != nil {
		t.Fatal(err)
	}

	Version = "v5.5.5-test"
	Build = "the-buildkite-test-value"

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(HTTPHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `{"version":"v5.5.5-test","build":"the-buildkite-test-value"}`
	if rr.Body.String() != expected {
		t.Errorf("Handler returned unexpected values: got %v want %v", rr.Body.String(), expected)
	}
}

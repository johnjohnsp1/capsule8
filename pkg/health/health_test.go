// health package unit tests
package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConnectionURLHealthyError(t *testing.T) {
	// Create a server to make sure nothing reachable is at that URL
	// Then shut it down to get a connection error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	ts.Close()

	testUrl := ConnectionURL(ts.URL)
	err := testUrl.Healthy()
	if err == nil {
		t.Errorf("Healthy() did not return the expected http response error: connection refused. Instead returned no error.")
	}
}

func TestConnectionURLHealthyNonStatusOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected 'GET' request, got '%s'", r.Method)
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()
	testUrl := ConnectionURL(ts.URL)
	err := testUrl.Healthy()
	if err == nil {
		t.Errorf("Healthy() did not return the expected http response error: 403 Forbidden. Instead returned no error.")
	}
}

func TestConnectionURLHealthyStatusOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected 'GET' request, got '%s'", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	testUrl := ConnectionURL(ts.URL)
	err := testUrl.Healthy()
	if err != nil {
		t.Errorf("Healthy() expected no error, instead returned: '%s'.", err)
	}
}

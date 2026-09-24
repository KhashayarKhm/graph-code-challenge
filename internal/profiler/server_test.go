package profiler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerServesProfiles(t *testing.T) {
	t.Parallel()

	handler := newHandler()

	for _, path := range []string{"/debug/pprof/", "/debug/pprof/heap", "/debug/pprof/goroutine"} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if recorder.Body.Len() == 0 {
				t.Fatal("response body is empty")
			}
		})
	}
}

func TestHandlerDoesNotExposeApplicationRoutes(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	newHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

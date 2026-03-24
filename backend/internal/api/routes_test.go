package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFallbackHandlerReturnsNotImplemented(t *testing.T) {
	r := NewRouter()
	req := httptest.NewRequest(http.MethodGet, "/not-yet-implemented", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", rr.Code)
	}
}

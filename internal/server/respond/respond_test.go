package respond

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBind(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		var dest struct {
			Name string `json:"name"`
		}
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Ada"}`))
		rec := httptest.NewRecorder()
		if !Bind(rec, req, &dest) {
			t.Fatalf("Bind false, body %s", rec.Body.String())
		}
		if dest.Name != "Ada" {
			t.Fatalf("got %q", dest.Name)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		var dest struct{}
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{`))
		rec := httptest.NewRecorder()
		if Bind(rec, req, &dest) {
			t.Fatal("want false")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"error":"invalid json"`) {
			t.Fatalf("body %s", rec.Body.String())
		}
	})
}

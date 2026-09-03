package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authkit "github.com/erizkiatama/ansara-gym/internal/auth"
)

func TestRequireTrainer(t *testing.T) {
	h := NewHandler(nil, authkit.NewTokens("abcdefghijklmnopqrstuvwxyz012345", time.Hour), nil)
	token, err := h.tokens.Issue("trainer-42")
	if err != nil {
		t.Fatal(err)
	}

	wrapped := h.RequireTrainer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := authkit.TrainerIDFromContext(r.Context())
		if !ok {
			t.Error("missing trainer id on context")
		}
		_, _ = w.Write([]byte(id))
	}))

	t.Run("valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "trainer-42" {
			t.Fatalf("body %q", rec.Body.String())
		}
	})

	t.Run("missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("forged", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJmb3JnZWQifQ.not-a-real-sig")
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d", rec.Code)
		}
	})
}

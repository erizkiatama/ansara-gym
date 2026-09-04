package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	authkit "github.com/erizkiatama/ansara-gym/internal/auth"
	persist "github.com/erizkiatama/ansara-gym/internal/client"
	authhttp "github.com/erizkiatama/ansara-gym/internal/server/auth"
	"github.com/go-chi/chi/v5"
)

const (
	trainerA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	trainerB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

type memStore struct {
	mu      sync.Mutex
	n       int
	clients map[string]persist.Client
}

func newMemStore() *memStore {
	return &memStore{clients: map[string]persist.Client{}}
}

func (m *memStore) nextID() string {
	m.n++
	return fmt.Sprintf("cccccccc-cccc-cccc-cccc-%012x", m.n)
}

func (m *memStore) Insert(_ context.Context, trainerID string, client persist.Client) (persist.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Unix(1, 0).UTC()
	row := persist.Client{
		ID:        m.nextID(),
		TrainerID: trainerID,
		Name:      client.Name,
		Notes:     client.Notes,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.clients[row.ID] = row
	return row, nil
}

func (m *memStore) List(_ context.Context, trainerID string) ([]persist.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]persist.Client, 0)
	for _, row := range m.clients {
		if row.TrainerID == trainerID {
			out = append(out, row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *memStore) Get(_ context.Context, trainerID, id string) (persist.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.clients[id]
	if !ok || row.TrainerID != trainerID {
		return persist.Client{}, persist.ErrNotFound
	}
	return row, nil
}

func (m *memStore) Update(_ context.Context, trainerID, id string, client persist.Client) (persist.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.clients[id]
	if !ok || row.TrainerID != trainerID {
		return persist.Client{}, persist.ErrNotFound
	}
	row.Name = client.Name
	row.Notes = client.Notes
	row.UpdatedAt = time.Unix(2, 0).UTC()
	m.clients[id] = row
	return row, nil
}

func (m *memStore) Delete(_ context.Context, trainerID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.clients[id]
	if !ok || row.TrainerID != trainerID {
		return persist.ErrNotFound
	}
	delete(m.clients, id)
	return nil
}

func testMux(store Repository) (http.Handler, *authkit.Tokens) {
	tokens := authkit.NewTokens("abcdefghijklmnopqrstuvwxyz012345", time.Hour)
	authH := authhttp.NewHandler(slog.New(slog.DiscardHandler), tokens, nil)
	h := NewHandler(slog.New(slog.DiscardHandler), store)

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(authH.RequireTrainer)
		r.Post("/v1/clients", h.Create)
		r.Get("/v1/clients", h.List)
		r.Get("/v1/clients/{id}", h.Get)
		r.Put("/v1/clients/{id}", h.Update)
		r.Delete("/v1/clients/{id}", h.Delete)
	})
	return r, tokens
}

func bearer(t *testing.T, tokens *authkit.Tokens, trainerID string) string {
	t.Helper()
	tok, err := tokens.Issue(trainerID)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func doJSON(t *testing.T, mux http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestClientsCRUDAndTenancy(t *testing.T) {
	store := newMemStore()
	mux, tokens := testMux(store)
	tokenA := bearer(t, tokens, trainerA)
	tokenB := bearer(t, tokens, trainerB)

	t.Run("unauthenticated", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodGet, "/v1/clients", "", "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("empty list is array", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodGet, "/v1/clients", tokenA, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		var got listResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Clients == nil || len(got.Clients) != 0 {
			t.Fatalf("got %#v", got.Clients)
		}
	})

	rec := doJSON(t, mux, http.MethodPost, "/v1/clients", tokenA, `{"name":"  Rina  ","notes":"  ACL on left knee  "}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d body %s", rec.Code, rec.Body.String())
	}
	var created clientResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Name != "Rina" || created.Notes != "ACL on left knee" || created.ID == "" {
		t.Fatalf("created %#v", created)
	}

	rec = doJSON(t, mux, http.MethodPost, "/v1/clients", tokenB, `{"name":"Budi","notes":"wrist is weaker than other people"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create B status %d body %s", rec.Code, rec.Body.String())
	}
	var other clientResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &other); err != nil {
		t.Fatal(err)
	}

	t.Run("list is trainer scoped", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodGet, "/v1/clients", tokenA, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		var got listResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Clients) != 1 || got.Clients[0].ID != created.ID {
			t.Fatalf("got %#v", got.Clients)
		}
	})

	t.Run("cross tenant get put delete are 404", func(t *testing.T) {
		path := "/v1/clients/" + other.ID
		if rec := doJSON(t, mux, http.MethodGet, path, tokenA, ""); rec.Code != http.StatusNotFound {
			t.Fatalf("get status %d body %s", rec.Code, rec.Body.String())
		}
		if rec := doJSON(t, mux, http.MethodPut, path, tokenA, `{"name":"Nope","notes":""}`); rec.Code != http.StatusNotFound {
			t.Fatalf("put status %d body %s", rec.Code, rec.Body.String())
		}
		if rec := doJSON(t, mux, http.MethodDelete, path, tokenA, ""); rec.Code != http.StatusNotFound {
			t.Fatalf("delete status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("put replaces name and notes", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodPut, "/v1/clients/"+created.ID, tokenA, `{"name":"Rina Putri","notes":""}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		var got clientResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Name != "Rina Putri" || got.Notes != "" {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("get own", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodGet, "/v1/clients/"+created.ID, tokenA, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodGet, "/v1/clients/not-a-uuid", tokenA, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("delete own and missing", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodDelete, "/v1/clients/"+created.ID, tokenA, "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		rec = doJSON(t, mux, http.MethodGet, "/v1/clients/"+created.ID, tokenA, "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
}

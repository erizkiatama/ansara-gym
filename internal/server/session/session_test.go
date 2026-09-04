package session

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
	authhttp "github.com/erizkiatama/ansara-gym/internal/server/auth"
	persist "github.com/erizkiatama/ansara-gym/internal/session"
	"github.com/go-chi/chi/v5"
)

const (
	trainerA     = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	trainerB     = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	clientA      = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	clientB      = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	catalogBench = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	catalogSquat = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	unknownEx    = "00000000-0000-0000-0000-000000000001"
)

type memStore struct {
	mu       sync.Mutex
	n        int
	clients  map[string]string
	catalog  map[string]struct{}
	sessions map[string]persist.Session
}

func newMemStore() *memStore {
	return &memStore{
		clients: map[string]string{
			clientA: trainerA,
			clientB: trainerB,
		},
		catalog: map[string]struct{}{
			catalogBench: {},
			catalogSquat: {},
		},
		sessions: map[string]persist.Session{},
	}
}

func (m *memStore) nextID() string {
	m.n++
	return fmt.Sprintf("99999999-9999-9999-9999-%012x", m.n)
}

func (m *memStore) Insert(_ context.Context, trainerID, clientID string, session persist.Session) (persist.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.clients[clientID] != trainerID {
		return persist.Session{}, persist.ErrNotFound
	}
	for _, ex := range session.Exercises {
		if _, ok := m.catalog[ex.ExerciseID]; !ok {
			return persist.Session{}, persist.ErrUnknownExercise
		}
	}

	now := time.Unix(1, 0).UTC()
	out := persist.Session{
		ID:          m.nextID(),
		ClientID:    clientID,
		TrainerID:   trainerID,
		SessionDate: session.SessionDate,
		Notes:       session.Notes,
		Exercises:   make([]persist.SessionExercise, 0, len(session.Exercises)),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	for _, ex := range session.Exercises {
		card := persist.SessionExercise{
			ID:         m.nextID(),
			SessionID:  out.ID,
			ExerciseID: ex.ExerciseID,
			OrderIndex: ex.OrderIndex,
			Notes:      ex.Notes,
			Sets:       make([]persist.Set, 0, len(ex.Sets)),
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		for _, st := range ex.Sets {
			card.Sets = append(card.Sets, persist.Set{
				ID:                m.nextID(),
				SessionExerciseID: card.ID,
				SetNumber:         st.SetNumber,
				Reps:              st.Reps,
				Weight:            st.Weight,
				RPE:               st.RPE,
				IsWarmup:          st.IsWarmup,
				CreatedAt:         now,
				UpdatedAt:         now,
			})
		}
		out.Exercises = append(out.Exercises, card)
	}
	m.sessions[out.ID] = out
	return out, nil
}

func (m *memStore) ownedClient(trainerID, clientID string) bool {
	return m.clients[clientID] == trainerID
}

func (m *memStore) List(_ context.Context, trainerID, clientID string, params persist.ListParams) ([]persist.Session, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.ownedClient(trainerID, clientID) {
		return nil, false, persist.ErrNotFound
	}

	rows := make([]persist.Session, 0)
	for _, row := range m.sessions {
		if row.TrainerID != trainerID || row.ClientID != clientID {
			continue
		}
		if params.BeforeID != "" {
			if !sessionBefore(row, params.BeforeDate, params.BeforeID) {
				continue
			}
		}
		header := row
		header.Exercises = nil
		rows = append(rows, header)
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].SessionDate.Equal(rows[j].SessionDate) {
			return rows[i].SessionDate.After(rows[j].SessionDate)
		}
		return rows[i].ID > rows[j].ID
	})
	hasMore := len(rows) > params.Limit
	if hasMore {
		rows = rows[:params.Limit]
	}
	return rows, hasMore, nil
}

func sessionBefore(row persist.Session, beforeDate time.Time, beforeID string) bool {
	if row.SessionDate.Before(beforeDate) {
		return true
	}
	if row.SessionDate.After(beforeDate) {
		return false
	}
	return row.ID < beforeID
}

func (m *memStore) Get(_ context.Context, trainerID, clientID, sessionID string) (persist.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.ownedClient(trainerID, clientID) {
		return persist.Session{}, persist.ErrNotFound
	}
	row, ok := m.sessions[sessionID]
	if !ok || row.TrainerID != trainerID || row.ClientID != clientID {
		return persist.Session{}, persist.ErrNotFound
	}
	return row, nil
}

func (m *memStore) Progress(_ context.Context, trainerID, clientID, exerciseID string) ([]persist.ProgressPoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.ownedClient(trainerID, clientID) {
		return nil, persist.ErrNotFound
	}

	points := make([]persist.ProgressPoint, 0)
	for _, row := range m.sessions {
		if row.TrainerID != trainerID || row.ClientID != clientID {
			continue
		}
		var max float64
		var any bool
		for _, ex := range row.Exercises {
			if ex.ExerciseID != exerciseID {
				continue
			}
			for _, st := range ex.Sets {
				if st.IsWarmup {
					continue
				}
				if !any || st.Weight > max {
					max = st.Weight
					any = true
				}
			}
		}
		if any {
			points = append(points, persist.ProgressPoint{
				SessionID:   row.ID,
				SessionDate: row.SessionDate,
				MaxWeight:   max,
			})
		}
	}
	sort.Slice(points, func(i, j int) bool {
		if !points[i].SessionDate.Equal(points[j].SessionDate) {
			return points[i].SessionDate.Before(points[j].SessionDate)
		}
		return points[i].SessionID < points[j].SessionID
	})
	return points, nil
}

func testMux(store Repository) (http.Handler, *authkit.Tokens) {
	tokens := authkit.NewTokens("abcdefghijklmnopqrstuvwxyz012345", time.Hour)
	authH := authhttp.NewHandler(slog.New(slog.DiscardHandler), tokens, nil)
	h := NewHandler(slog.New(slog.DiscardHandler), store)

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(authH.RequireTrainer)
		r.Post("/v1/clients/{id}/sessions", h.Create)
		r.Get("/v1/clients/{id}/sessions", h.List)
		r.Get("/v1/clients/{id}/sessions/{sessionId}", h.Get)
		r.Get("/v1/clients/{id}/exercises/{exerciseId}/progress", h.Progress)
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

func TestCreateSession(t *testing.T) {
	store := newMemStore()
	mux, tokens := testMux(store)
	tokenA := bearer(t, tokens, trainerA)
	tokenB := bearer(t, tokens, trainerB)
	pathA := "/v1/clients/" + clientA + "/sessions"
	pathB := "/v1/clients/" + clientB + "/sessions"

	t.Run("unauthenticated", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodPost, pathA, "", `{"session_date":"2026-09-07"}`)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodPost, pathA, tokenA, `{`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid client id", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodPost, "/v1/clients/not-a-uuid/sessions", tokenA, `{"session_date":"2026-09-07"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("cross tenant client is 404", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodPost, pathB, tokenA, `{"session_date":"2026-09-07"}`)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("empty session calendar slot", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodPost, pathA, tokenA, `{"session_date":"2026-09-07","notes":"  upper  "}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		var got sessionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.ID == "" || got.SessionDate != "2026-09-07" || got.Notes != "upper" {
			t.Fatalf("got %#v", got)
		}
		if got.Exercises == nil || len(got.Exercises) != 0 {
			t.Fatalf("exercises %#v", got.Exercises)
		}
	})

	t.Run("exercises without sets", func(t *testing.T) {
		body := fmt.Sprintf(`{"session_date":"2026-09-07","exercises":[{"exercise_id":%q,"order_index":0},{"exercise_id":%q,"order_index":1}]}`, catalogSquat, catalogBench)
		rec := doJSON(t, mux, http.MethodPost, pathA, tokenA, body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		var got sessionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Exercises) != 2 || got.Exercises[0].ID == "" || got.Exercises[1].ID == "" {
			t.Fatalf("got %#v", got.Exercises)
		}
		if got.Exercises[0].Sets == nil || len(got.Exercises[0].Sets) != 0 {
			t.Fatalf("sets %#v", got.Exercises[0].Sets)
		}
	})

	t.Run("exercise with first set and mixed empty", func(t *testing.T) {
		body := fmt.Sprintf(`{
			"session_date":"2026-09-07",
			"exercises":[
				{"exercise_id":%q,"order_index":0,"sets":[{"set_number":1,"reps":8,"weight":80,"rpe":7,"is_warmup":false}]},
				{"exercise_id":%q,"order_index":1,"sets":[]}
			]
		}`, catalogBench, catalogSquat)
		rec := doJSON(t, mux, http.MethodPost, pathA, tokenA, body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		var got sessionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Exercises) != 2 {
			t.Fatalf("got %#v", got.Exercises)
		}
		if len(got.Exercises[0].Sets) != 1 || got.Exercises[0].Sets[0].ID == "" {
			t.Fatalf("sets %#v", got.Exercises[0].Sets)
		}
		set := got.Exercises[0].Sets[0]
		if set.SetNumber != 1 || set.Reps != 8 || set.Weight != 80 || set.RPE == nil || *set.RPE != 7 || set.IsWarmup {
			t.Fatalf("set %#v", set)
		}
		if len(got.Exercises[1].Sets) != 0 {
			t.Fatalf("squat sets %#v", got.Exercises[1].Sets)
		}
	})

	t.Run("unknown exercise_id is 400", func(t *testing.T) {
		store.mu.Lock()
		before := len(store.sessions)
		store.mu.Unlock()

		body := fmt.Sprintf(`{"session_date":"2026-09-07","exercises":[{"exercise_id":%q,"order_index":0}]}`, unknownEx)
		rec := doJSON(t, mux, http.MethodPost, pathA, tokenA, body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"unknown exercise_id"`)) {
			t.Fatalf("body %s", rec.Body.String())
		}

		store.mu.Lock()
		after := len(store.sessions)
		store.mu.Unlock()
		if after != before {
			t.Fatalf("mem store leaked a session, before=%d after=%d", before, after)
		}
	})

	t.Run("other trainer can create for own client", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodPost, pathB, tokenB, `{"session_date":"2026-09-09"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing session_date", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodPost, pathA, tokenA, `{"notes":"no date"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
}

func TestCreateSessionValidationErrors(t *testing.T) {
	store := newMemStore()
	mux, tokens := testMux(store)
	tokenA := bearer(t, tokens, trainerA)
	pathA := "/v1/clients/" + clientA + "/sessions"

	cases := []struct {
		name string
		body string
	}{
		{"bad date", `{"session_date":"07-09-2026"}`},
		{"duplicate order", fmt.Sprintf(`{"session_date":"2026-09-07","exercises":[{"exercise_id":%q,"order_index":0},{"exercise_id":%q,"order_index":0}]}`, catalogBench, catalogSquat)},
		{"bad set number", fmt.Sprintf(`{"session_date":"2026-09-07","exercises":[{"exercise_id":%q,"order_index":0,"sets":[{"set_number":0,"reps":8,"weight":80}]}]}`, catalogBench)},
		{"negative weight", fmt.Sprintf(`{"session_date":"2026-09-07","exercises":[{"exercise_id":%q,"order_index":0,"sets":[{"set_number":1,"reps":8,"weight":-1}]}]}`, catalogBench)},
		{"rpe high", fmt.Sprintf(`{"session_date":"2026-09-07","exercises":[{"exercise_id":%q,"order_index":0,"sets":[{"set_number":1,"reps":8,"weight":80,"rpe":11}]}]}`, catalogBench)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doJSON(t, mux, http.MethodPost, pathA, tokenA, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestListGetProgress(t *testing.T) {
	mem := newMemStore()
	mux, tokens := testMux(mem)
	tokenA := bearer(t, tokens, trainerA)
	tokenB := bearer(t, tokens, trainerB)
	pathA := "/v1/clients/" + clientA + "/sessions"
	pathB := "/v1/clients/" + clientB + "/sessions"

	t.Run("unauthenticated list", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodGet, pathA, "", "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("cross tenant list is 404", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodGet, pathB, tokenA, "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("empty list is array", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodGet, pathA, tokenA, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		var got listResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Sessions == nil || len(got.Sessions) != 0 || got.Next != nil {
			t.Fatalf("got %#v", got)
		}
	})

	sep11 := doJSON(t, mux, http.MethodPost, pathA, tokenA, fmt.Sprintf(
		`{"session_date":"2026-09-11","notes":"fri","exercises":[{"exercise_id":%q,"order_index":0,"sets":[{"set_number":1,"reps":5,"weight":60,"is_warmup":true},{"set_number":2,"reps":5,"weight":80,"is_warmup":false}]}]}`,
		catalogBench,
	))
	if sep11.Code != http.StatusCreated {
		t.Fatalf("sep11 %d %s", sep11.Code, sep11.Body.String())
	}
	var fri sessionResponse
	if err := json.Unmarshal(sep11.Body.Bytes(), &fri); err != nil {
		t.Fatal(err)
	}

	sep9 := doJSON(t, mux, http.MethodPost, pathA, tokenA, `{"session_date":"2026-09-09","notes":"wed"}`)
	if sep9.Code != http.StatusCreated {
		t.Fatalf("sep9 %d %s", sep9.Code, sep9.Body.String())
	}
	var wed sessionResponse
	if err := json.Unmarshal(sep9.Body.Bytes(), &wed); err != nil {
		t.Fatal(err)
	}

	sep7 := doJSON(t, mux, http.MethodPost, pathA, tokenA, fmt.Sprintf(
		`{"session_date":"2026-09-07","notes":"mon","exercises":[{"exercise_id":%q,"order_index":0,"sets":[{"set_number":1,"reps":5,"weight":70,"is_warmup":false}]}]}`,
		catalogBench,
	))
	if sep7.Code != http.StatusCreated {
		t.Fatalf("sep7 %d %s", sep7.Code, sep7.Body.String())
	}

	doJSON(t, mux, http.MethodPost, pathB, tokenB, `{"session_date":"2026-09-11"}`)

	t.Run("keyset pages newest first", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodGet, pathA+"?limit=2", tokenA, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		var page1 listResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &page1); err != nil {
			t.Fatal(err)
		}
		if len(page1.Sessions) != 2 || page1.Sessions[0].SessionDate != "2026-09-11" || page1.Sessions[1].SessionDate != "2026-09-09" {
			t.Fatalf("page1 %#v", page1.Sessions)
		}
		if page1.Next == nil || page1.Next.BeforeID != wed.ID {
			t.Fatalf("next %#v", page1.Next)
		}

		next := pathA + "?limit=2&before_date=" + page1.Next.BeforeDate + "&before_id=" + page1.Next.BeforeID
		rec = doJSON(t, mux, http.MethodGet, next, tokenA, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("page2 status %d body %s", rec.Code, rec.Body.String())
		}
		var page2 listResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &page2); err != nil {
			t.Fatal(err)
		}
		if len(page2.Sessions) != 1 || page2.Sessions[0].SessionDate != "2026-09-07" || page2.Next != nil {
			t.Fatalf("page2 %#v", page2)
		}
	})

	t.Run("get own session graph", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodGet, pathA+"/"+fri.ID, tokenA, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		var got sessionResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != fri.ID || len(got.Exercises) != 1 || len(got.Exercises[0].Sets) != 2 {
			t.Fatalf("got %#v", got)
		}
	})

	t.Run("get cross tenant session is 404", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodGet, pathA+"/"+fri.ID, tokenB, "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		rec = doJSON(t, mux, http.MethodGet, pathB+"/"+fri.ID, tokenB, "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("session on other client %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("progress skips warmups and unknown exercise", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodGet, "/v1/clients/"+clientA+"/exercises/"+catalogBench+"/progress", tokenA, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		var got progressResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Points) != 2 {
			t.Fatalf("points %#v", got.Points)
		}
		if got.Points[0].SessionDate != "2026-09-07" || got.Points[0].MaxWeight != 70 {
			t.Fatalf("mon %#v", got.Points[0])
		}
		if got.Points[1].SessionDate != "2026-09-11" || got.Points[1].MaxWeight != 80 {
			t.Fatalf("fri %#v", got.Points[1])
		}

		rec = doJSON(t, mux, http.MethodGet, "/v1/clients/"+clientA+"/exercises/"+unknownEx+"/progress", tokenA, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("unknown status %d %s", rec.Code, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Points == nil || len(got.Points) != 0 {
			t.Fatalf("unknown points %#v", got.Points)
		}
	})

	t.Run("progress cross tenant is 404", func(t *testing.T) {
		rec := doJSON(t, mux, http.MethodGet, "/v1/clients/"+clientB+"/exercises/"+catalogBench+"/progress", tokenA, "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
	})
}

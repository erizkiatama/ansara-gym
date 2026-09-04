package session

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	store "github.com/erizkiatama/ansara-gym/internal/session"
)

func TestValidateSession(t *testing.T) {
	day := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	bench := "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	squat := "ffffffff-ffff-ffff-ffff-ffffffffffff"

	if err := validateSession(store.Session{SessionDate: day}); err != nil {
		t.Fatal(err)
	}
	if err := validateSession(store.Session{}); err == nil {
		t.Fatal("want error for missing session_date")
	}

	ok := store.Session{
		SessionDate: day,
		Exercises: []store.SessionExercise{
			{ExerciseID: bench, OrderIndex: 0},
			{ExerciseID: squat, OrderIndex: 1, Sets: []store.Set{
				{SetNumber: 1, Reps: 8, Weight: 80, RPE: sql.NullFloat64{Float64: 7, Valid: true}},
			}},
		},
	}
	if err := validateSession(ok); err != nil {
		t.Fatal(err)
	}

	dupOrder := ok
	dupOrder.Exercises = []store.SessionExercise{
		{ExerciseID: bench, OrderIndex: 0},
		{ExerciseID: squat, OrderIndex: 0},
	}
	if err := validateSession(dupOrder); err == nil {
		t.Fatal("want duplicate order_index")
	}
}

func TestParseSessionDate(t *testing.T) {
	got, err := parseSessionDate("2026-09-07")
	if err != nil {
		t.Fatal(err)
	}
	if got.Format("2006-01-02") != "2026-09-07" {
		t.Fatalf("got %v", got)
	}
	if _, err := parseSessionDate("07/09/2026"); err == nil {
		t.Fatal("want error")
	}
	zero, err := parseSessionDate("")
	if err != nil || !zero.IsZero() {
		t.Fatalf("empty: %v %v", zero, err)
	}
}

func TestParseListPage(t *testing.T) {
	first, err := parseListPage(httptest.NewRequest(http.MethodGet, "/v1/clients/x/sessions", nil))
	if err != nil {
		t.Fatal(err)
	}
	if first.Limit != defaultListLimit || first.BeforeID != "" {
		t.Fatalf("got %#v", first)
	}

	ok, err := parseListPage(httptest.NewRequest(http.MethodGet, "/v1/clients/x/sessions?limit=2&before_date=2026-09-07&before_id=11111111-1111-1111-1111-111111111111", nil))
	if err != nil {
		t.Fatal(err)
	}
	if ok.Limit != 2 || ok.BeforeID != "11111111-1111-1111-1111-111111111111" || ok.BeforeDate.Format("2006-01-02") != "2026-09-07" {
		t.Fatalf("got %#v", ok)
	}

	cases := []string{
		"/v1/clients/x/sessions?limit=0",
		"/v1/clients/x/sessions?limit=101",
		"/v1/clients/x/sessions?before_date=2026-09-07",
		"/v1/clients/x/sessions?before_id=11111111-1111-1111-1111-111111111111",
		"/v1/clients/x/sessions?before_date=07-09-2026&before_id=11111111-1111-1111-1111-111111111111",
		"/v1/clients/x/sessions?before_date=2026-09-07&before_id=nope",
	}
	for _, u := range cases {
		if _, err := parseListPage(httptest.NewRequest(http.MethodGet, u, nil)); err == nil {
			t.Errorf("%s: want error", u)
		}
	}
}

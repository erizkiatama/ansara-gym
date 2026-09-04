package session

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	store "github.com/erizkiatama/ansara-gym/internal/session"
	"github.com/erizkiatama/ansara-gym/internal/utils"
)

const maxWeightKg = 9999.99

func validateSession(session store.Session) error {
	if session.SessionDate.IsZero() {
		return errors.New("session_date is required")
	}

	seenOrder := make(map[int]struct{}, len(session.Exercises))
	for _, ex := range session.Exercises {
		if ex.ExerciseID == "" {
			return errors.New("exercise_id is required")
		}
		if !utils.ValidID(ex.ExerciseID) {
			return errors.New("invalid exercise_id")
		}
		if ex.OrderIndex < 0 {
			return errors.New("order_index must be >= 0")
		}
		if _, dup := seenOrder[ex.OrderIndex]; dup {
			return errors.New("duplicate order_index")
		}
		seenOrder[ex.OrderIndex] = struct{}{}

		if err := validateSets(ex.Sets); err != nil {
			return err
		}
	}
	return nil
}

func validateSets(sets []store.Set) error {
	seenNumber := make(map[int]struct{}, len(sets))
	for _, st := range sets {
		if st.SetNumber <= 0 {
			return errors.New("set_number must be greater than 0")
		}
		if _, dup := seenNumber[st.SetNumber]; dup {
			return errors.New("duplicate set_number")
		}
		seenNumber[st.SetNumber] = struct{}{}
		if st.Reps < 0 {
			return errors.New("reps must be >= 0")
		}
		if math.IsNaN(st.Weight) || math.IsInf(st.Weight, 0) || st.Weight < 0 {
			return errors.New("weight must be >= 0")
		}
		if st.Weight > maxWeightKg {
			return errors.New("weight is too large")
		}
		if st.RPE.Valid && (st.RPE.Float64 < 1 || st.RPE.Float64 > 10) {
			return errors.New("rpe must be between 1 and 10")
		}
	}
	return nil
}

func parseSessionDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", s)
}

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

func parseListPage(r *http.Request) (store.ListParams, error) {
	params := store.ListParams{Limit: defaultListLimit}

	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxListLimit {
			return store.ListParams{}, errors.New("limit must be between 1 and 100")
		}
		params.Limit = n
	}

	beforeDate := strings.TrimSpace(r.URL.Query().Get("before_date"))
	beforeID := strings.TrimSpace(r.URL.Query().Get("before_id"))
	if beforeDate == "" && beforeID == "" {
		return params, nil
	}
	if beforeDate == "" || beforeID == "" {
		return store.ListParams{}, errors.New("before_date and before_id are required together")
	}
	day, err := time.Parse("2006-01-02", beforeDate)
	if err != nil {
		return store.ListParams{}, errors.New("invalid before_date")
	}
	if !utils.ValidID(beforeID) {
		return store.ListParams{}, errors.New("invalid before_id")
	}
	params.BeforeDate = day
	params.BeforeID = beforeID
	return params, nil
}

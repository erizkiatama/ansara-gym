package respond

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

const maxJSONBodyBytes = 1 << 16

// Bind decodes the JSON body into dest. On failure it writes
// 400 {"error":"invalid json"} and returns false.
func Bind(w http.ResponseWriter, r *http.Request, dest any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		Error(w, http.StatusBadRequest, "invalid json")
		return false
	}
	return true
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json response", "err", err)
	}
}

func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

package client

import (
	"database/sql"
	"testing"

	store "github.com/erizkiatama/ansara-gym/internal/client"
)

func TestValidateClient(t *testing.T) {
	if err := validateClient(store.Client{Name: "Rina"}); err != nil {
		t.Fatal(err)
	}
	if err := validateClient(store.Client{Name: ""}); err == nil {
		t.Fatal("want error for empty name")
	}
	if err := validateClient(store.Client{
		Name:  "Rina",
		Notes: sql.NullString{String: "ACL on left knee", Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestValidID(t *testing.T) {
	if !validID("11111111-1111-1111-1111-111111111111") {
		t.Fatal("want valid")
	}
	if !validID("018f3a2b-9c4d-7e80-8f12-3456789abcde") {
		t.Fatal("want valid uuidv7-shaped")
	}
	for _, in := range []string{"", "not-a-uuid", "111111111111111111111111111111111111", "11111111-1111-1111-1111-11111111111g"} {
		if validID(in) {
			t.Errorf("validID(%q): want false", in)
		}
	}
}

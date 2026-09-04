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

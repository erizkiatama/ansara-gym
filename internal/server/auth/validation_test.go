package auth

import (
	"testing"

	store "github.com/erizkiatama/ansara-gym/internal/trainer"
)

func TestValidateSignup(t *testing.T) {
	ok := store.Trainer{Email: "ada@example.com", Name: "Ada"}
	if err := validateSignup(ok, "correct-horse"); err != nil {
		t.Fatal(err)
	}

	if err := validateSignup(store.Trainer{Email: "", Name: "Ada"}, "correct-horse"); err == nil {
		t.Fatal("want error for empty email")
	}
	if err := validateSignup(store.Trainer{Email: "no-at", Name: "Ada"}, "correct-horse"); err == nil {
		t.Fatal("want error for invalid email")
	}
	if err := validateSignup(store.Trainer{Email: "ada@example.com", Name: ""}, "correct-horse"); err == nil {
		t.Fatal("want error for empty name")
	}
	if err := validateSignup(ok, "short"); err == nil {
		t.Fatal("want error for short password")
	}
}

func TestValidateEmail(t *testing.T) {
	if err := validateEmail("foo.bar@example.com"); err != nil {
		t.Fatal(err)
	}
	for _, in := range []string{"", "no-at", "@x.com", "x@", "a@b c.com"} {
		if err := validateEmail(in); err == nil {
			t.Errorf("validateEmail(%q): want error", in)
		}
	}
}

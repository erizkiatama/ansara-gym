package auth

import "testing"

func TestNormalizeEmail(t *testing.T) {
	got, err := normalizeEmail("  Foo.Bar@Example.COM ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "foo.bar@example.com" {
		t.Fatalf("got %q", got)
	}

	for _, in := range []string{"", "no-at", "@x.com", "x@", "a@b c.com"} {
		if _, err := normalizeEmail(in); err == nil {
			t.Errorf("normalizeEmail(%q): want error", in)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if err := validatePassword("short"); err == nil {
		t.Fatal("want error for short password")
	}
	if err := validatePassword("long-enough"); err != nil {
		t.Fatal(err)
	}
}

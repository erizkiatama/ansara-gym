package auth

import (
	"testing"
	"time"
)

const testSecret = "abcdefghijklmnopqrstuvwxyz012345"

func TestTokensIssueVerify(t *testing.T) {
	toks := NewTokens(testSecret, time.Hour)
	raw, err := toks.Issue("trainer-1")
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	id, err := toks.Verify(raw)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if id != "trainer-1" {
		t.Fatalf("subject: got %q", id)
	}
}

func TestTokensRejectsWrongSecret(t *testing.T) {
	raw, err := NewTokens(testSecret, time.Hour).Issue("trainer-1")
	if err != nil {
		t.Fatal(err)
	}
	other := NewTokens("ABCDEFGHIJKLMNOPQRSTUVWXYZ012345", time.Hour)
	if _, err := other.Verify(raw); err != ErrInvalidToken {
		t.Fatalf("got %v, want ErrInvalidToken", err)
	}
}

func TestTokensRejectsExpired(t *testing.T) {
	toks := NewTokens(testSecret, time.Hour)
	toks.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	raw, err := toks.Issue("trainer-1")
	if err != nil {
		t.Fatal(err)
	}
	fresh := NewTokens(testSecret, time.Hour)
	if _, err := fresh.Verify(raw); err != ErrInvalidToken {
		t.Fatalf("expired: got %v, want ErrInvalidToken", err)
	}
}

func TestTokensRejectsGarbage(t *testing.T) {
	toks := NewTokens(testSecret, time.Hour)
	for _, raw := range []string{"", "not.a.jwt", "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ4In0."} {
		if _, err := toks.Verify(raw); err != ErrInvalidToken {
			t.Errorf("Verify(%q): got %v, want ErrInvalidToken", raw, err)
		}
	}
}

func TestTrainerIDContext(t *testing.T) {
	ctx := WithTrainerID(t.Context(), "abc")
	id, ok := TrainerIDFromContext(ctx)
	if !ok || id != "abc" {
		t.Fatalf("got %q %v", id, ok)
	}
	if _, ok := TrainerIDFromContext(t.Context()); ok {
		t.Fatal("empty context should miss")
	}
}

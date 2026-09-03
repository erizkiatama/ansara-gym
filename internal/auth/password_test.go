package auth

import (
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestHashCompareRoundTrip(t *testing.T) {
	hash, err := Hash("correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected PHC prefix: %s", hash)
	}
	if err := Compare(hash, "correct horse battery"); err != nil {
		t.Fatalf("Compare same password: %v", err)
	}
	if err := Compare(hash, "wrong password"); err != ErrPassword {
		t.Fatalf("Compare wrong password: got %v, want ErrPassword", err)
	}
}

func TestHashRejectsEmpty(t *testing.T) {
	if _, err := Hash(""); err != ErrPassword {
		t.Fatalf("got %v, want ErrPassword", err)
	}
}

func TestCompareMalformedHash(t *testing.T) {
	cases := []string{
		"",
		"not-a-hash",
		"$bcrypt$v=19$m=19456,t=2,p=1$YWJj$YWJj",
		"$argon2id$v=19$m=99999999,t=2,p=1$YWJjZGVmZ2hpamtsbW5vcA$YWJjZGVmZ2hpamtsbW5vcGFiY2RlZmdoaWprbG1ub3A",
		"$argon2id$v=19$m=19456,t=2,p=1$not-base64!$also-bad",
	}
	for _, encoded := range cases {
		if err := Compare(encoded, "x"); err != ErrInvalidHash {
			t.Errorf("Compare(%q): got %v, want ErrInvalidHash", encoded, err)
		}
	}
}

func TestCompareUsesStoredParams(t *testing.T) {
	// Hand-built PHC with weaker params than today's constants. Compare must
	// recompute with m/t/p from the string, not the package defaults.
	password := "legacy"
	salt := []byte("0123456789abcdef")
	key := argon2.IDKey([]byte(password), salt, 1, 8*1024, 1, 32)
	encoded := encodePHC(19, 8*1024, 1, 1, salt, key)

	if err := Compare(encoded, password); err != nil {
		t.Fatalf("legacy params should verify: %v", err)
	}
}

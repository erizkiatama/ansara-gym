package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// OWASP Argon2id baseline (2024): 19 MiB, 2 iterations, 1 thread.
const (
	argonTime    uint32 = 2
	argonMemory  uint32 = 19 * 1024
	argonThreads uint8  = 1
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16

	// Caps on values parsed from a stored PHC string so a crafted hash cannot
	// allocate gigabytes during Compare (e.g. after a DB write).
	maxArgonMemory  uint32 = 64 * 1024
	maxArgonTime    uint32 = 10
	maxArgonThreads uint8  = 4
)

var (
	ErrInvalidHash = errors.New("invalid password hash")
	ErrPassword    = errors.New("password mismatch")
)

var (
	dummyOnce sync.Once
	dummyPHC  string
)

// Hash returns an Argon2id PHC string. Params are embedded so old hashes
// still verify after we bump constants.
func Hash(password string) (string, error) {
	if password == "" {
		return "", ErrPassword
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return encodePHC(argon2.Version, argonMemory, argonTime, argonThreads, salt, key), nil
}

// Compare checks password against a PHC hash using the params stored in the
// string (not today's constants). Timing-safe on the derived key.
func Compare(encoded, password string) error {
	memory, timeCost, threads, salt, key, err := parsePHC(encoded)
	if err != nil {
		return err
	}
	computed := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, uint32(len(key)))
	if subtle.ConstantTimeCompare(key, computed) != 1 {
		return ErrPassword
	}
	return nil
}

// DummyCompare runs a full Argon2id verify against a throwaway hash so a
// missing-email login takes about as long as a real miss. Ignore the error.
func DummyCompare(password string) {
	dummyOnce.Do(func() {
		h, err := Hash("timing-dummy")
		if err != nil {
			return
		}
		dummyPHC = h
	})
	if dummyPHC == "" {
		return
	}
	_ = Compare(dummyPHC, password)
}

func encodePHC(version int, memory, timeCost uint32, threads uint8, salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		version, memory, timeCost, threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

func parsePHC(encoded string) (memory, timeCost uint32, threads uint8, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeCost, &threads); err != nil {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}
	if memory == 0 || memory > maxArgonMemory || timeCost == 0 || timeCost > maxArgonTime || threads == 0 || threads > maxArgonThreads {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return 0, 0, 0, nil, nil, ErrInvalidHash
	}
	return memory, timeCost, threads, salt, key, nil
}

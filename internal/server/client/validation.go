package client

import (
	"errors"

	store "github.com/erizkiatama/ansara-gym/internal/client"
)

func validateClient(client store.Client) error {
	if client.Name == "" {
		return errors.New("name is required")
	}

	return nil
}

func validID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i := 0; i < 36; i++ {
		c := id[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !isHex(c) {
				return false
			}
		}
	}
	return true
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

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

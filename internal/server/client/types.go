package client

import (
	"context"

	store "github.com/erizkiatama/ansara-gym/internal/client"
)

// Repository is the slice of client persistence this HTTP package needs.
// Every method takes trainerID; tenancy is enforced in the repo, not the handler.
type Repository interface {
	Insert(ctx context.Context, trainerID string, client store.Client) (store.Client, error)
	List(ctx context.Context, trainerID string) ([]store.Client, error)
	Get(ctx context.Context, trainerID, id string) (store.Client, error)
	Update(ctx context.Context, trainerID, id string, client store.Client) (store.Client, error)
	Delete(ctx context.Context, trainerID, id string) error
}

type clientRequest struct {
	Name  string `json:"name"`
	Notes string `json:"notes"`
}

type clientResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Notes string `json:"notes,omitempty"`
}

type listResponse struct {
	Clients []clientResponse `json:"clients"`
}

func toResponse(row store.Client) clientResponse {
	notes := ""
	if row.Notes.Valid {
		notes = row.Notes.String
	}
	return clientResponse{
		ID:    row.ID,
		Name:  row.Name,
		Notes: notes,
	}
}

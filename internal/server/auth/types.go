package auth

import (
	"context"

	store "github.com/erizkiatama/ansara-gym/internal/trainer"
)

// Repository is the slice of trainer persistence this HTTP package needs.
type Repository interface {
	Insert(ctx context.Context, trainer store.Trainer) (store.Trainer, error)
	GetByEmail(ctx context.Context, email string) (store.Trainer, error)
	GetByID(ctx context.Context, id string) (store.Trainer, error)
}

type signupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type trainerResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type tokenResponse struct {
	Token   string          `json:"token"`
	Trainer trainerResponse `json:"trainer"`
}

func toResponse(row store.Trainer) trainerResponse {
	return trainerResponse{ID: row.ID, Email: row.Email, Name: row.Name}
}

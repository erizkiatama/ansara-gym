package auth

import (
	"context"

	"github.com/erizkiatama/ansara-gym/internal/trainer"
)

// TrainerStore is the slice of trainer persistence this HTTP package needs.
type TrainerStore interface {
	Insert(ctx context.Context, email, passwordHash, name string) (trainer.Trainer, error)
	GetByEmail(ctx context.Context, email string) (trainer.Trainer, error)
	GetByID(ctx context.Context, id string) (trainer.Trainer, error)
}

var _ TrainerStore = (*trainer.Repo)(nil)

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

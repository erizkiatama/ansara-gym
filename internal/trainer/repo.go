package trainer

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	ErrNotFound   = errors.New("trainer not found")
	ErrEmailTaken = errors.New("email already registered")
)

type Trainer struct {
	ID           string    `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Name         string    `db:"name"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

type Repo struct {
	db *sqlx.DB
}

func NewRepo(db *sqlx.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Insert(ctx context.Context, trainer Trainer) (Trainer, error) {
	var t Trainer
	err := r.db.QueryRowxContext(ctx, `
		INSERT INTO trainers (email, password_hash, name)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) DO NOTHING
		RETURNING id, email, password_hash, name, created_at, updated_at
	`, trainer.Email, trainer.PasswordHash, trainer.Name).StructScan(&t)
	if errors.Is(err, sql.ErrNoRows) {
		return Trainer{}, ErrEmailTaken
	}
	if err != nil {
		return Trainer{}, err
	}
	return t, nil
}

func (r *Repo) GetByID(ctx context.Context, id string) (Trainer, error) {
	var t Trainer
	err := r.db.GetContext(ctx, &t, `
		SELECT id, email, password_hash, name, created_at, updated_at
		FROM trainers
		WHERE id = $1
	`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Trainer{}, ErrNotFound
	}
	if err != nil {
		return Trainer{}, err
	}
	return t, nil
}

func (r *Repo) GetByEmail(ctx context.Context, email string) (Trainer, error) {
	var t Trainer
	err := r.db.GetContext(ctx, &t, `
		SELECT id, email, password_hash, name, created_at, updated_at
		FROM trainers
		WHERE email = $1
	`, email)
	if errors.Is(err, sql.ErrNoRows) {
		return Trainer{}, ErrNotFound
	}
	if err != nil {
		return Trainer{}, err
	}
	return t, nil
}

package client

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("client not found")

// Client is a trainer-owned roster row. Notes is NULL when empty.
type Client struct {
	ID        string         `db:"id"`
	TrainerID string         `db:"trainer_id"`
	Name      string         `db:"name"`
	Notes     sql.NullString `db:"notes"`
	CreatedAt time.Time      `db:"created_at"`
	UpdatedAt time.Time      `db:"updated_at"`
}

type Repo struct {
	db *sqlx.DB
}

func NewRepo(db *sqlx.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Insert(ctx context.Context, trainerID string, client Client) (Client, error) {
	var c Client
	err := r.db.QueryRowxContext(ctx, `
		INSERT INTO clients (trainer_id, name, notes)
		VALUES ($1, $2, $3)
		RETURNING id, trainer_id, name, notes, created_at, updated_at
	`, trainerID, client.Name, client.Notes).StructScan(&c)
	if err != nil {
		return Client{}, err
	}
	return c, nil
}

// List returns every client for trainerID in one query (no session embed).
func (r *Repo) List(ctx context.Context, trainerID string) ([]Client, error) {
	var rows []Client
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, trainer_id, name, notes, created_at, updated_at
		FROM clients
		WHERE trainer_id = $1
		ORDER BY id
	`, trainerID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []Client{}
	}
	return rows, nil
}

func (r *Repo) Get(ctx context.Context, trainerID, id string) (Client, error) {
	var c Client
	err := r.db.GetContext(ctx, &c, `
		SELECT id, trainer_id, name, notes, created_at, updated_at
		FROM clients
		WHERE trainer_id = $1 AND id = $2
	`, trainerID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Client{}, ErrNotFound
	}
	if err != nil {
		return Client{}, err
	}
	return c, nil
}

func (r *Repo) Update(ctx context.Context, trainerID, id string, client Client) (Client, error) {
	var c Client
	err := r.db.QueryRowxContext(ctx, `
		UPDATE clients
		SET name = $3, notes = $4, updated_at = now()
		WHERE trainer_id = $1 AND id = $2
		RETURNING id, trainer_id, name, notes, created_at, updated_at
	`, trainerID, id, client.Name, client.Notes).StructScan(&c)
	if errors.Is(err, sql.ErrNoRows) {
		return Client{}, ErrNotFound
	}
	if err != nil {
		return Client{}, err
	}
	return c, nil
}

func (r *Repo) Delete(ctx context.Context, trainerID, id string) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM clients
		WHERE trainer_id = $1 AND id = $2
	`, trainerID, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

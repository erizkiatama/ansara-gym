-- +goose Up

ALTER TABLE clients ADD COLUMN notes text;

-- +goose Down

ALTER TABLE clients DROP COLUMN notes;

package auth

import "context"

type trainerIDKey struct{}

func WithTrainerID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, trainerIDKey{}, id)
}

func TrainerIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(trainerIDKey{}).(string)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}

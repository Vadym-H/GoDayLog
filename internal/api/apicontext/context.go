package apicontext

import (
	"context"

	"github.com/Vadym-H/GoDayLog/internal/domain"
)

type ctxKey int

const identityKey ctxKey = 0

func WithIdentity(ctx context.Context, id domain.Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

func IdentityFromCtx(ctx context.Context) (domain.Identity, bool) {
	id, ok := ctx.Value(identityKey).(domain.Identity)
	return id, ok
}

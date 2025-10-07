package auth

import (
	"context"
	"errors"
)

type noopVerifier struct{}

func newNoopVerifier(_ Config) Verifier {
	return noopVerifier{}
}

func (noopVerifier) Verify(_ context.Context, token string) (AuthenticatedUser, error) {
	if token == "" {
		return AuthenticatedUser{}, errors.New("token must not be empty")
	}
	return AuthenticatedUser{UserID: token, Token: token}, nil
}

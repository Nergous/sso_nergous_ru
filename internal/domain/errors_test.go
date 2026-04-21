package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"sso/internal/domain"
)

func TestErrors_AreDistinct(t *testing.T) {
	all := []error{
		domain.ErrUserNotFound,
		domain.ErrUserAlreadyExists,
		domain.ErrAppNotFound,
		domain.ErrAppAlreadyExists,
		domain.ErrInvalidCredentials,
		domain.ErrInvalidToken,
		domain.ErrTokenExpired,
		domain.ErrPasswordMismatch,
		domain.ErrPermissionDenied,
		domain.ErrValidationFailed,
	}
	for i, a := range all {
		for j, b := range all {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Fatalf("expected %v and %v to be distinct", a, b)
			}
		}
	}
}

func TestErrors_WrappedDetectableByErrorsIs(t *testing.T) {
	wrapped := fmt.Errorf("auth.Login: %w", domain.ErrInvalidCredentials)
	if !errors.Is(wrapped, domain.ErrInvalidCredentials) {
		t.Fatal("errors.Is must match through wrap")
	}
}

package errmap_test

import (
	"errors"
	"fmt"
	"testing"

	"sso/internal/domain"
	"sso/internal/transport/grpc/errmap"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToV1_MapsDomainErrors(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want codes.Code
	}{
		{"invalid creds", domain.ErrInvalidCredentials, codes.Unauthenticated},
		{"user not found", domain.ErrUserNotFound, codes.NotFound},
		{"already exists", domain.ErrUserAlreadyExists, codes.AlreadyExists},
		{"permission", domain.ErrPermissionDenied, codes.PermissionDenied},
		{"validation", domain.ErrValidationFailed, codes.InvalidArgument},
		{"unknown", errors.New("boom"), codes.Internal},
		{"nil", nil, codes.OK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := errmap.ToV1(c.in)
			if c.in == nil {
				if got != nil {
					t.Fatalf("nil → %v", got)
				}
				return
			}
			if s, _ := status.FromError(got); s.Code() != c.want {
				t.Fatalf("want %v, got %v", c.want, s.Code())
			}
		})
	}
}

func TestToV1_HidesRawMessage(t *testing.T) {
	wrapped := fmt.Errorf("repo: SELECT failed: %w", domain.ErrUserNotFound)
	got := errmap.ToV1(wrapped)
	s, _ := status.FromError(got)
	if s.Message() == wrapped.Error() {
		t.Fatal("raw error leaked")
	}
}

func TestToV2_AttachesErrorInfo(t *testing.T) {
	got := errmap.ToV2(domain.ErrUserNotFound)
	s, _ := status.FromError(got)
	if s.Code() != codes.NotFound {
		t.Fatalf("code: %v", s.Code())
	}
	found := false
	for _, d := range s.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			if info.GetReason() == "USER_NOT_FOUND" && info.GetDomain() == "sso.nergous.ru" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("missing ErrorInfo{reason: USER_NOT_FOUND, domain: sso.nergous.ru}")
	}
}

func TestToV2_UnknownErrorYieldsInternalNoInfo(t *testing.T) {
	got := errmap.ToV2(errors.New("raw db error"))
	s, _ := status.FromError(got)
	if s.Code() != codes.Internal {
		t.Fatalf("want Internal, got %v", s.Code())
	}
	if s.Message() == "raw db error" {
		t.Fatal("raw error leaked to client")
	}
}

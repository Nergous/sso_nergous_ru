package interceptors

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestTimeoutInterceptor_AppliesDeadline(t *testing.T) {
	interceptor := TimeoutUnaryInterceptor(50 * time.Millisecond)

	handler := func(ctx context.Context, req any) (any, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			return nil, errors.New("expected deadline on ctx")
		}
		if until := time.Until(deadline); until <= 0 || until > 60*time.Millisecond {
			return nil, errors.New("unexpected deadline")
		}
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/X/Y"}, handler)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected resp: %v", resp)
	}
}

func TestTimeoutInterceptor_RespectsExistingShorterDeadline(t *testing.T) {
	interceptor := TimeoutUnaryInterceptor(1 * time.Second)

	parent, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	handler := func(ctx context.Context, req any) (any, error) {
		deadline, _ := ctx.Deadline()
		if until := time.Until(deadline); until > 15*time.Millisecond {
			return nil, errors.New("deadline got extended beyond parent")
		}
		return nil, nil
	}

	if _, err := interceptor(parent, nil, &grpc.UnaryServerInfo{FullMethod: "/X/Y"}, handler); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

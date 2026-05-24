package grpcauth

import (
	"context"
	"log/slog"
	"sso/internal/kernel/actor"
	"sso/internal/platform/crypto/jwt"
	"sso/internal/session"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	errUnauthenticated   = status.Error(codes.Unauthenticated, "unauthenticated")
	errMissingMD         = status.Error(codes.Unauthenticated, "missing metadata")
	errMissingAuthHeader = status.Error(codes.Unauthenticated, "missing authorization header")
	errBadAuthScheme     = status.Error(codes.Unauthenticated, "authorization scheme must be Bearer")
	errEmptyToken        = status.Error(codes.Unauthenticated, "empty bearer token")
)

type Interceptor struct {
	verifier   jwt.Verifier
	sessions   session.Repository
	log        *slog.Logger
	publicRPCs map[string]struct{}
	now        func() time.Time
}

func NewInterceptor(
	v jwt.Verifier,
	s session.Repository,
	log *slog.Logger,
	publicRPCs []string,
) *Interceptor {
	set := make(map[string]struct{}, len(publicRPCs))
	for _, m := range publicRPCs {
		set[m] = struct{}{}
	}
	return &Interceptor{verifier: v, sessions: s, log: log, publicRPCs: set, now: time.Now}
}

func (i *Interceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, public := i.publicRPCs[info.FullMethod]; public {
			return handler(ctx, req)
		}

		token, err := bearerFromCtx(ctx)
		if err != nil {
			i.log.WarnContext(ctx, "grpcauth: bearer extract", "method", info.FullMethod, "err", err)
			return nil, errUnauthenticated
		}

		claims, err := i.verifier.Verify(token)
		if err != nil {
			i.log.WarnContext(ctx, "grpcauth: verify", "method", info.FullMethod, "err", err)
			return nil, errUnauthenticated
		}

		var kind actor.Kind
		switch claims.SubjectType {
		case jwt.SubjectTypeUser:
			kind = actor.KindUser
		case jwt.SubjectTypeServiceAccount:
			kind = actor.KindServiceAccount
		default:
			i.log.WarnContext(ctx, "grpcauth: unknown subject_type",
				"method", info.FullMethod, "subject_type", claims.SubjectType)
			return nil, errUnauthenticated
		}

		if kind == actor.KindUser {
			sess, err := i.sessions.GetByID(ctx, session.SessionID(claims.SessionID))
			if err != nil || !sess.IsActive(i.now().UTC()) {
				if err != nil {
					i.log.WarnContext(ctx, "grpcauth: session lookup",
						"method", info.FullMethod, "session_id", claims.SessionID, "err", err)
				}
				return nil, errUnauthenticated
			}
		}

		ctx = actor.Inject(ctx, actor.Actor{
			ID:        claims.Subject,
			Kind:      kind,
			SessionID: claims.SessionID,
			IpAddress: PeerIP(ctx),
			UserAgent: UserAgentFromCtx(ctx),
		})
		return handler(ctx, req)
	}
}

func bearerFromCtx(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", errMissingMD
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", errMissingAuthHeader
	}
	raw := strings.TrimSpace(values[0])
	const prefix = "Bearer "
	if !strings.HasPrefix(raw, prefix) {
		return "", errBadAuthScheme
	}
	tok := strings.TrimSpace(raw[len(prefix):])
	if tok == "" {
		return "", errEmptyToken
	}
	return tok, nil
}

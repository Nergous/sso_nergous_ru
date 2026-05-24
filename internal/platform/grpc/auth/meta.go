package grpcauth

import (
	"context"
	"net"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// PeerIP returns the authoritative client IP from the gRPC peer.
// Returns "" when the peer has no Addr (in-process tests, mocks).
// Client-supplied IPs are intentionally ignored — only the gRPC peer
// is trusted.
func PeerIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(p.Addr.String())
	if err != nil {
		return p.Addr.String()
	}
	return host
}

// UserAgentFromCtx returns the gRPC client's User-Agent header from
// incoming metadata. Returns "" when no metadata is attached.
func UserAgentFromCtx(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if v := md.Get("user-agent"); len(v) > 0 {
		return v[0]
	}
	return ""
}

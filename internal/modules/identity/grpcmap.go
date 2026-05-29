package identity

import grpcadapter "sso/internal/modules/identity/internal/grpc"

// UserToProto re-exports the canonical User → proto mapping for sibling
// gRPC adapters (notably auth.Register, which returns
// sso.identity.v1.User as its response payload). Keeping a single
// implementation inside internal/grpc avoids duplicating the wire shape
// in every consumer.
var UserToProto = grpcadapter.UserToProto

// NewHandler re-exports the gRPC handler constructor. Used by the
// contract test in internal/contract/ to verify every RPC has a real
// body and does not fall through to the proto-generated
// UnimplementedIdentityServiceServer embed.
var NewHandler = grpcadapter.NewHandler

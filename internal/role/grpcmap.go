package role

import grpcadapter "sso/internal/role/internal/grpc"

// RoleToProto re-exports the canonical Role → proto mapping for sibling
// gRPC adapters (notably access.ListUserRoles, which embeds Role in its
// response). Keeping a single implementation inside internal/grpc avoids
// duplicating the wire shape in every consumer.
var RoleToProto = grpcadapter.RoleToProto

// NewHandler re-exports the gRPC handler constructor for the contract
// test in internal/contract/.
var NewHandler = grpcadapter.NewHandler

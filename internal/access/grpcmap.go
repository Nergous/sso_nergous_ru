package access

import grpcadapter "sso/internal/access/internal/grpc"

// NewHandler re-exports the gRPC handler constructor for the contract
// test in internal/contract/.
var NewHandler = grpcadapter.NewHandler

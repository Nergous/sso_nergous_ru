package audit

import grpcadapter "sso/internal/audit/internal/grpc"

// NewHandler re-exports the gRPC handler constructor for the contract
// test in internal/contract/.
var NewHandler = grpcadapter.NewHandler

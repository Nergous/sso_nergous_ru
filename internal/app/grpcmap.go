package app

import appgrpc "sso/internal/app/internal/grpc"

// NewHandler re-exports the gRPC handler constructor for the contract
// test in internal/contract/.
var NewHandler = appgrpc.NewHandler

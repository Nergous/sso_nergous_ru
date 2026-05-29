package service

import "errors"

// ErrPermissionDenied is returned when the caller is authenticated but
// not authorized to read the audit log. The gRPC adapter maps it to
// PERMISSION_DENIED + ERROR_REASON_PERMISSION_DENIED.
var ErrPermissionDenied = errors.New("audit: permission denied")

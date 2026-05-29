package auditx

import "sso/internal/kernel/validation"

// DefaultListPageSize / MaxListPageSize are the canonical bounds used by
// every List* RPC that paginates with the proto's ≤ 1000 cap. The
// outlier (auth.ListSessions uses a ≤ 100 cap because users typically
// have O(10) sessions) calls ClampPageSizeWithMax directly.
const (
	DefaultListPageSize = 50
	MaxListPageSize     = 1000
)

// ClampPageSize normalises an int32 page_size from the wire to an int
// using DefaultListPageSize / MaxListPageSize. raw < 0 is rejected as a
// validation.Error tagged "page_size"; 0 → default; > max → max.
func ClampPageSize(raw int32) (int, error) {
	return ClampPageSizeWithMax(raw, DefaultListPageSize, MaxListPageSize)
}

// ClampPageSizeWithMax is the variant that takes explicit default / max
// bounds. See ClampPageSize for the canonical defaults.
func ClampPageSizeWithMax(raw int32, defSize, maxSize int) (int, error) {
	p := int(raw)
	switch {
	case p < 0:
		return 0, &validation.Error{Field: "page_size", Reason: "must be ≥ 0"}
	case p == 0:
		return defSize, nil
	case p > maxSize:
		return maxSize, nil
	}
	return p, nil
}

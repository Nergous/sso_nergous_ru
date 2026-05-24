// Package cursor is a generic codec for opaque pagination tokens.
//
// Each domain defines its own narrow JSON struct (the "page token") with
// short field names — typically `c` for created_at, `i` for id, etc.
// Encode marshals it to JSON and base64url-encodes the bytes; Decode
// reverses the process. The opaque blob is what flies on the wire as
// `page_token`.
//
// Convention: nil cursor encodes to "" (the last-page marker), and ""
// decodes to nil (the first-page marker). Callers don't need to add
// their own nil checks.
//
// Errors from Decode are RAW JSON / base64 errors. Callers that need a
// validation.Error tagged "page_token" should wrap them themselves —
// keeping the codec free of validation knowledge.
package cursor

import (
	"encoding/base64"
	"encoding/json"
)

// Encode marshals v to JSON and base64url-encodes the result. A nil v
// yields "" with no error — the canonical "no more pages" wire value.
func Encode[T any](v *T) (string, error) {
	if v == nil {
		return "", nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Decode reverses Encode. Empty input yields (nil, nil) — the canonical
// "first page" wire value. Malformed input surfaces as a non-nil error;
// the caller decides whether to wrap it in a validation.Error.
func Decode[T any](s string) (*T, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	v := new(T)
	if err := json.Unmarshal(raw, v); err != nil {
		return nil, err
	}
	return v, nil
}

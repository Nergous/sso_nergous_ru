package auditx

import (
	"sso/internal/kernel/etag"
	"sso/internal/kernel/validation"
)

// EtagWildcard is the wire value used by every mutating service to mean
// "unconditional update". Use-cases convert it to a domain-level empty
// Etag before reaching the repository.
const EtagWildcard = "*"

// ParseExpectedEtag converts the wire etag ("*", a UUID, or "" when
// optional) to a domain Etag. A domain "" return value means
// "unconditional". When required=true an empty input surfaces as a
// validation.Error tagged "etag".
func ParseExpectedEtag(s string, required bool) (etag.Etag, error) {
	if s == "" {
		if required {
			return "", &validation.Error{Field: "etag", Reason: "must be provided"}
		}
		return "", nil
	}
	if s == EtagWildcard {
		return "", nil
	}
	return etag.Parse(s)
}

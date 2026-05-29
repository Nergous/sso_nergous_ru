// Package auditbus implements the audit.Emitter contract — the
// publication surface between use-cases and the audit repository.
//
// Components:
//
//	Sanitize        — redacts sensitive metadata values by key blocklist.
//	SyncEmitter     — write-through Emitter; logs and swallows repo errors.
//	RecorderEmitter — in-memory Emitter for tests.
package auditbus

import (
	"strings"

	"sso/internal/modules/audit"
)

// RedactedPlaceholder is the value that replaces a blocked metadata
// entry. The key is preserved so the audit log still shows that the
// field was present (diagnostic).
const RedactedPlaceholder = "[redacted]"

// blocklist is the set of metadata keys whose values must never end up
// in the audit log verbatim. Lookups are case-insensitive: the table
// stores already-lowered forms and Sanitize lowers each input key
// before checking.
var blocklist = map[string]struct{}{
	"password":       {},
	"password_hash":  {},
	"refresh_token":  {},
	"access_token":   {},
	"recovery_code":  {},
	"recovery_codes": {},
	"secret":         {},
	"client_secret":  {},
	"jwt":            {},
	"token":          {},
	"hash":           {},
}

// Sanitize returns a copy of `a` with metadata values for sensitive
// keys replaced by RedactedPlaceholder. Keys are matched
// case-insensitively against a fixed blocklist. All other fields are
// preserved as-is.
//
// The original aggregate is not mutated — audit events are immutable
// by design. nil input returns nil.
func Sanitize(a *audit.Audit) *audit.Audit {
	if a == nil {
		return nil
	}
	cleaned := sanitizeMetadata(a.Metadata())
	return audit.RestoreAudit(audit.RestoreAuditParams{
		ID:          a.ID(),
		EventType:   a.EventType(),
		ActorType:   a.ActorType(),
		ActorID:     a.ActorID(),
		SubjectType: a.SubjectType(),
		SubjectID:   a.SubjectID(),
		AppID:       a.AppID(),
		Outcome:     a.Outcome(),
		Reason:      a.Reason(),
		IpAddress:  a.IPAddress(),
		UserAgent:  a.UserAgent(),
		Metadata:   cleaned,
		OccurredAt: a.OccurredAt(),
	})
}

// sanitizeMetadata returns a copy of m with blocked keys redacted. nil
// / empty input returns nil (RestoreAudit handles it via maps.Clone).
func sanitizeMetadata(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if _, blocked := blocklist[strings.ToLower(k)]; blocked {
			out[k] = RedactedPlaceholder
		} else {
			out[k] = v
		}
	}
	return out
}

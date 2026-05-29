package grpcadapter

import (
	domain "sso/internal/modules/audit/internal/domain"
	usecase "sso/internal/modules/audit/internal/service"

	ssoauditv1 "github.com/Nergous/sso_protos/gen/go/sso/audit/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ----------------------------------------------------------------------------
// Domain → proto
// ----------------------------------------------------------------------------

func auditToProto(a *domain.Audit) *ssoauditv1.AuditEvent {
	if a == nil {
		return nil
	}
	return &ssoauditv1.AuditEvent{
		EventId:    a.ID().String(),
		OccurredAt: timestamppb.New(a.OccurredAt()),
		EventType:  a.EventType().String(),
		Actor:      actorToProto(a),
		Subject:    subjectToProto(a),
		Outcome:    outcomeToProto(a.Outcome()),
		Reason:     a.Reason(),
		Metadata:   copyMetadata(a.Metadata()),
	}
}

func actorToProto(a *domain.Audit) *ssoauditv1.Actor {
	var ip string
	if addr := a.IPAddress(); addr.IsValid() {
		ip = addr.String()
	}
	return &ssoauditv1.Actor{
		Type:      ssoauditv1.ActorType(uint8(a.ActorType())),
		Id:        a.ActorID().String(),
		IpAddress: ip,
		UserAgent: a.UserAgent(),
	}
}

func subjectToProto(a *domain.Audit) *ssoauditv1.Subject {
	return &ssoauditv1.Subject{
		Type:  ssoauditv1.SubjectType(uint8(a.SubjectType())),
		Id:    a.SubjectID().String(),
		AppId: a.AppID().String(),
	}
}

func outcomeToProto(o domain.AuditOutcome) ssoauditv1.AuditOutcome {
	// Numbering matches proto 1-to-1 (see subject_type.go / audit.go);
	// unknown values fall through to AUDIT_OUTCOME_UNSPECIFIED via the
	// proto generator's default — safe because the domain validates
	// outcome on NewAudit, so stored events never carry Unknown.
	return ssoauditv1.AuditOutcome(uint8(o))
}

// copyMetadata returns a shallow copy of m so the gRPC response cannot
// be used as a back-channel into the domain aggregate.
func copyMetadata(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// ----------------------------------------------------------------------------
// Proto → use-case input
// ----------------------------------------------------------------------------

func filtersFromProto(f *ssoauditv1.AuditFilters) usecase.AuditFiltersInput {
	if f == nil {
		return usecase.AuditFiltersInput{}
	}

	out := usecase.AuditFiltersInput{
		EventTypeSlugs: f.GetEventTypes(),
		ActorType:      domain.ActorType(uint8(f.GetActorType())),
		ActorID:        f.GetActorId(),
		ActorIP:        f.GetActorIp(),
		SubjectType:    domain.SubjectType(uint8(f.GetSubjectType())),
		SubjectID:      f.GetSubjectId(),
		AppID:          f.GetAppId(),
	}

	if ts := f.GetFromTime(); ts != nil && ts.IsValid() {
		t := ts.AsTime()
		out.From = &t
	}
	if ts := f.GetToTime(); ts != nil && ts.IsValid() {
		t := ts.AsTime()
		out.To = &t
	}

	if protoOutcomes := f.GetOutcomes(); len(protoOutcomes) > 0 {
		out.Outcomes = make([]domain.AuditOutcome, len(protoOutcomes))
		for i, po := range protoOutcomes {
			out.Outcomes[i] = domain.AuditOutcome(uint8(po))
		}
	}
	return out
}

package service

import (
	"context"

	"sso/internal/modules/app/internal/domain"
	"sso/internal/modules/audit"
	"sso/internal/kernel/actor"
)

// CreateAppInput is the parsed CreateAppRequest. Field validation
// (name length, link URI, slug regex) is expected upstream — applied by
// the protovalidate interceptor.
type CreateAppInput struct {
	Name string
	Slug string
	Link string
}

// CreateApp provisions a new app. Server generates id, etag, timestamps;
// status defaults to ACTIVE. Returns ErrAppAlreadyExists on uniqueness
// collision (name or slug).
func (s *Service) CreateApp(ctx context.Context, in CreateAppInput) (*domain.App, error) {
	a, err := actor.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := domain.NewAppID()
	if err != nil {
		return nil, err
	}

	target := domain.NewApp(domain.NewAppParams{
		ID:   id,
		Name: in.Name,
		Slug: in.Slug,
		Link: in.Link,
		Now:  s.now().UTC(),
	})

	aud := audit.BaseFromActor(a, audit.EventTypeAppCreateApp)
	aud.SubjectType = audit.SubjectTypeApp
	aud.SubjectID = id.String()
	aud.AppID = id.String()

	if err := s.repo.Create(ctx, target); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return nil, err
	}

	s.auditor.Success(ctx, aud)
	return target, nil
}

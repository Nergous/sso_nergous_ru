package domain

import "sso/internal/modules/app"

// AppID is a cross-context handle to the app.App aggregate, aliased here
// so the role domain package and its use-case callers can reference
// AppID / ParseAppID without importing the sibling app package
// everywhere. The alias is identity at the type-checker level — a value
// of role/internal/domain.AppID is the same type as app.AppID and needs
// no conversion at boundaries.
type AppID = app.AppID

// ParseAppID re-exports app.ParseAppID so use-cases inside this package
// can validate raw parent_app_id strings without an extra import.
var ParseAppID = app.ParseAppID

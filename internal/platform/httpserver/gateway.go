package httpserver

import (
	"context"
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	ssoaccessv1 "github.com/Nergous/sso_protos/gen/go/sso/access/v1"
	ssoappv1 "github.com/Nergous/sso_protos/gen/go/sso/app/v1"
	ssoauditv1 "github.com/Nergous/sso_protos/gen/go/sso/audit/v1"
	ssoauthv1 "github.com/Nergous/sso_protos/gen/go/sso/auth/v1"
	ssoidentityv1 "github.com/Nergous/sso_protos/gen/go/sso/identity/v1"
	ssorolesv1 "github.com/Nergous/sso_protos/gen/go/sso/roles/v1"
	ssoserviceaccountv1 "github.com/Nergous/sso_protos/gen/go/sso/serviceaccount/v1"
)

func registerGatewayHandlers(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
	type registrar struct {
		name string
		fn   func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error
	}
	regs := []registrar{
		{"identity", ssoidentityv1.RegisterIdentityServiceHandler},
		{"app", ssoappv1.RegisterAppServiceHandler},
		{"roles", ssorolesv1.RegisterRolesServiceHandler},
		{"serviceaccount", ssoserviceaccountv1.RegisterServiceAccountServiceHandler},
		{"access", ssoaccessv1.RegisterAccessServiceHandler},
		{"auth", ssoauthv1.RegisterAuthServiceHandler},
		{"audit", ssoauditv1.RegisterAuditServiceHandler},
	}
	for _, r := range regs {
		if err := r.fn(ctx, mux, conn); err != nil {
			return fmt.Errorf("register %s gateway handler: %w", r.name, err)
		}
	}
	return nil
}

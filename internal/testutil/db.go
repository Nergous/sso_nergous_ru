package testutil

import (
	"context"
	"testing"
	"time"

	ssomariadb "sso/internal/storage/mariadb"

	tcmariadb "github.com/testcontainers/testcontainers-go/modules/mariadb"
)

// NewTestStorage spins up a MariaDB testcontainer, runs migrations, and
// returns a ready Storage plus a cleanup closure. Fails the test if Docker
// is unavailable or migrations fail.
func NewTestStorage(t *testing.T) (*ssomariadb.Storage, func()) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	container, err := tcmariadb.Run(ctx,
		"mariadb:11",
		tcmariadb.WithDatabase("ssotest"),
		tcmariadb.WithUsername("sso"),
		tcmariadb.WithPassword("sso"),
	)
	if err != nil {
		t.Fatalf("failed to start mariadb container: %v", err)
	}

	dsn, err := container.ConnectionString(ctx, "parseTime=true")
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("failed to get dsn: %v", err)
	}

	storage, err := ssomariadb.NewStorage(dsn)
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("failed to open storage: %v", err)
	}

	if err := storage.Migrate(); err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("failed to migrate: %v", err)
	}

	cleanup := func() {
		_ = storage.Close()
		termCtx, termCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer termCancel()
		_ = container.Terminate(termCtx)
	}

	return storage, cleanup
}

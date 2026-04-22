package cubebox

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStoreModelSettingsLifecyclePG(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, ok := connectCubeboxTestPool(ctx, t)
	if !ok {
		return
	}
	defer pool.Close()

	tenantID := uuid.NewString()
	principalID := uuid.NewString()
	providerID := "provider_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	modelSlug := "gpt-4.1"

	if err := seedCubeboxTestActor(ctx, pool, tenantID, principalID); err != nil {
		t.Fatalf("seed actor: %v", err)
	}
	defer cleanupCubeboxTestActor(context.Background(), pool, tenantID, principalID)

	store := NewStore(pool)

	provider, err := store.UpsertModelProvider(ctx, tenantID, principalID, UpsertModelProviderInput{
		ProviderID:   providerID,
		ProviderType: "openai-compatible",
		DisplayName:  "Integration Provider",
		BaseURL:      "https://example.invalid/v1",
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("upsert provider: %v", err)
	}
	if provider.ID != providerID {
		t.Fatalf("provider id=%q want %q", provider.ID, providerID)
	}

	credential, err := store.RotateModelCredential(ctx, tenantID, principalID, RotateModelCredentialInput{
		ProviderID:   providerID,
		SecretRef:    "env://OPENAI_API_KEY",
		MaskedSecret: "sk-****",
	})
	if err != nil {
		t.Fatalf("rotate credential: %v", err)
	}
	if credential.ProviderID != providerID || !credential.Active {
		t.Fatalf("unexpected credential: %+v", credential)
	}

	selection, err := store.SelectActiveModel(ctx, tenantID, principalID, SelectActiveModelInput{
		ProviderID: providerID,
		ModelSlug:  modelSlug,
		CapabilitySummary: map[string]any{
			"streaming": true,
			"tools":     false,
		},
	})
	if err != nil {
		t.Fatalf("select active model: %v", err)
	}
	if selection.ProviderID != providerID || selection.ModelSlug != modelSlug {
		t.Fatalf("unexpected selection: %+v", selection)
	}

	health, err := store.VerifyActiveModel(ctx, tenantID, principalID)
	if err != nil {
		t.Fatalf("verify active model: %v", err)
	}
	if health.ProviderID != providerID || health.ModelSlug != modelSlug || health.Status != "healthy" {
		t.Fatalf("unexpected health: %+v", health)
	}
	if health.LatencyMS == nil || *health.LatencyMS <= 0 {
		t.Fatalf("unexpected latency: %+v", health)
	}

	snapshot, err := store.GetModelSettings(ctx, tenantID)
	if err != nil {
		t.Fatalf("get model settings: %v", err)
	}
	if len(snapshot.Providers) != 1 || snapshot.Providers[0].ID != providerID {
		t.Fatalf("unexpected providers snapshot: %+v", snapshot.Providers)
	}
	if len(snapshot.Credentials) != 1 || snapshot.Credentials[0].ProviderID != providerID {
		t.Fatalf("unexpected credentials snapshot: %+v", snapshot.Credentials)
	}
	if snapshot.Selection == nil || snapshot.Selection.ProviderID != providerID || snapshot.Selection.ModelSlug != modelSlug {
		t.Fatalf("unexpected selection snapshot: %+v", snapshot.Selection)
	}
	if snapshot.Health == nil || snapshot.Health.ProviderID != providerID || snapshot.Health.Status != "healthy" {
		t.Fatalf("unexpected health snapshot: %+v", snapshot.Health)
	}
}

func connectCubeboxTestPool(ctx context.Context, t *testing.T) (*pgxpool.Pool, bool) {
	t.Helper()

	pool, err := pgxpool.New(ctx, cubeboxTestDSN())
	if err != nil {
		t.Logf("skip postgres: %v", err)
		return nil, false
	}
	if err := pool.Ping(ctx); err != nil {
		t.Logf("skip postgres: %v", err)
		pool.Close()
		return nil, false
	}
	return pool, true
}

func cubeboxTestDSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(envOrDefault("DB_USER", "app"), envOrDefault("DB_PASSWORD", "app")),
		Host:   envOrDefault("DB_HOST", "127.0.0.1") + ":" + envOrDefault("DB_PORT", "5438"),
		Path:   "/" + envOrDefault("DB_NAME", "bugs_and_blossoms"),
	}
	q := u.Query()
	q.Set("sslmode", envOrDefault("DB_SSLMODE", "disable"))
	u.RawQuery = q.Encode()
	return u.String()
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func seedCubeboxTestActor(ctx context.Context, pool *pgxpool.Pool, tenantID string, principalID string) error {
	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
INSERT INTO iam.tenants (id, name, is_active, created_at, updated_at)
VALUES ($1::uuid, $2, true, $3, $3)
`, tenantID, "Cubebox Integration "+tenantID[:8], now); err != nil {
		return fmt.Errorf("insert tenant: %w", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO iam.principals (
  id,
  tenant_uuid,
  email,
  role_slug,
  display_name,
  status,
  kratos_identity_id,
  created_at,
  updated_at
) VALUES (
  $1::uuid,
  $2::uuid,
  $3,
  'tenant-admin',
  'Cubebox Integration Admin',
  'active',
  $4,
  $5,
  $5
)
`, principalID, tenantID, "cubebox-integration-"+tenantID[:8]+"@example.invalid", uuid.NewString(), now); err != nil {
		return fmt.Errorf("insert principal: %w", err)
	}
	return nil
}

func cleanupCubeboxTestActor(ctx context.Context, pool *pgxpool.Pool, tenantID string, principalID string) {
	_, _ = pool.Exec(ctx, `DELETE FROM iam.cubebox_model_health_checks WHERE tenant_uuid = $1::uuid`, tenantID)
	_, _ = pool.Exec(ctx, `DELETE FROM iam.cubebox_model_selections WHERE tenant_uuid = $1::uuid`, tenantID)
	_, _ = pool.Exec(ctx, `DELETE FROM iam.cubebox_model_credentials WHERE tenant_uuid = $1::uuid`, tenantID)
	_, _ = pool.Exec(ctx, `DELETE FROM iam.cubebox_model_providers WHERE tenant_uuid = $1::uuid`, tenantID)
	_, _ = pool.Exec(ctx, `DELETE FROM iam.principals WHERE id = $1::uuid AND tenant_uuid = $2::uuid`, principalID, tenantID)
	_, _ = pool.Exec(ctx, `DELETE FROM iam.tenants WHERE id = $1::uuid`, tenantID)
}

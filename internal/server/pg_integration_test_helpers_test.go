package server

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func connectTestPostgres(ctx context.Context, t *testing.T) (*pgx.Conn, string, bool) {
	t.Helper()
	dsn := dbDSNFromEnv()
	testDSN, ok := ensureTestDatabase(ctx, t, dsn)
	if !ok {
		return nil, "", false
	}
	conn, err := pgx.Connect(ctx, testDSN)
	if err != nil {
		t.Logf("skip postgres: %v", err)
		return nil, "", false
	}
	return conn, testDSN, true
}

func withUserPassword(dsn string, user string, pass string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(user, pass)
	return u.String(), nil
}

func ensureTestDatabase(ctx context.Context, t *testing.T, dsn string) (string, bool) {
	t.Helper()

	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v, true
	}

	baseURL, err := url.Parse(dsn)
	if err != nil {
		t.Logf("skip postgres: %v", err)
		return "", false
	}

	dbName := strings.TrimPrefix(baseURL.Path, "/")
	if dbName == "" {
		dbName = "postgres"
	}

	testDBName := dbName
	if !strings.HasSuffix(testDBName, "_test") {
		testDBName = testDBName + "_test"
	}
	if !validDBName(testDBName) {
		t.Logf("skip postgres: invalid test database name %q", testDBName)
		return "", false
	}

	adminURL := *baseURL
	adminURL.Path = "/postgres"

	adminConn, err := pgx.Connect(ctx, adminURL.String())
	if err != nil {
		t.Logf("skip postgres: %v", err)
		return "", false
	}
	defer adminConn.Close(ctx)

	var exists int
	err = adminConn.QueryRow(ctx, `SELECT 1 FROM pg_database WHERE datname = $1`, testDBName).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := adminConn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", testDBName)); err != nil {
			t.Logf("skip postgres: %v", err)
			return "", false
		}
	} else if err != nil {
		t.Logf("skip postgres: %v", err)
		return "", false
	}

	testURL := *baseURL
	testURL.Path = "/" + testDBName
	return testURL.String(), true
}

func validDBName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

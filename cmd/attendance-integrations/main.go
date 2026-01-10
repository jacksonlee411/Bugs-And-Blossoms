package main

import (
	"context"
	"errors"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/attendanceintegrations"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/event"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
)

func main() {
	tenantID := os.Getenv("TENANT_ID")
	if tenantID == "" {
		log.Fatal("TENANT_ID is required")
	}

	dsn := dbDSNFromEnv()
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	store := attendanceintegrations.NewPGStore(pool)

	dingTalkClientID := stringsTrimSpaceOrEmpty(os.Getenv("DINGTALK_CLIENT_ID"))
	dingTalkClientSecret := stringsTrimSpaceOrEmpty(os.Getenv("DINGTALK_CLIENT_SECRET"))
	dingTalkCorpID := stringsTrimSpaceOrEmpty(os.Getenv("DINGTALK_CORP_ID"))
	dingTalkEnabled := dingTalkClientID != "" || dingTalkClientSecret != "" || dingTalkCorpID != ""
	if dingTalkEnabled && (dingTalkClientID == "" || dingTalkClientSecret == "" || dingTalkCorpID == "") {
		log.Fatal("DingTalk enabled but missing env: DINGTALK_CLIENT_ID, DINGTALK_CLIENT_SECRET, DINGTALK_CORP_ID")
	}

	wecomCorpID := stringsTrimSpaceOrEmpty(os.Getenv("WECOM_CORP_ID"))
	wecomCorpSecret := stringsTrimSpaceOrEmpty(os.Getenv("WECOM_CORP_SECRET"))
	wecomEnabled := wecomCorpID != "" || wecomCorpSecret != ""
	if wecomEnabled && (wecomCorpID == "" || wecomCorpSecret == "") {
		log.Fatal("WeCom enabled but missing env: WECOM_CORP_ID, WECOM_CORP_SECRET")
	}

	if !dingTalkEnabled && !wecomEnabled {
		log.Fatal("no provider enabled (set DingTalk or WeCom env)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if wecomEnabled {
		intervalSeconds := getenvIntDefault("WECOM_INTERVAL_SECONDS", 30)
		lookbackSeconds := getenvIntDefault("WECOM_LOOKBACK_SECONDS", 600)
		go runWeComPoller(ctx, pool, store, tenantID, wecomCorpID, wecomCorpSecret, intervalSeconds, lookbackSeconds)
	}

	if dingTalkEnabled {
		// SDK 默认 logger 是 no-op；这里启用基础 stdout 日志，便于排障。
		logger.SetLogger(logger.NewStdTestLogger())

		cli := client.NewStreamClient(client.WithAppCredential(client.NewAppCredentialConfig(dingTalkClientID, dingTalkClientSecret)))
		cli.RegisterAllEventRouter(func(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
			h := event.NewEventHeaderFromDataFrame(df)
			if h.EventType != "attendance_check_record" {
				return event.NewSuccessResponse()
			}

			if h.EventCorpId != dingTalkCorpID {
				log.Printf("dingtalk corp mismatch: expected=%s got=%s event_id=%s", dingTalkCorpID, h.EventCorpId, h.EventId)
				return event.NewSuccessResponse()
			}

			punches, err := attendanceintegrations.BuildDingTalkAttendanceCheckRecordPunches(h.EventId, h.EventCorpId, []byte(df.Data))
			if err != nil {
				log.Printf("dingtalk parse error: event_id=%s err=%v", h.EventId, err)
				return event.NewLaterResponse()
			}

			for _, p := range punches {
				res, err := attendanceintegrations.IngestExternalPunch(ctx, store, tenantID, tenantID, p)
				if err != nil {
					log.Printf("dingtalk ingest error: tenant_id=%s event_id=%s request_id=%s external_user_id=%s err=%v", tenantID, h.EventId, p.RequestID, p.ExternalUserID, err)
					return event.NewLaterResponse()
				}
				log.Printf("dingtalk ingest: tenant_id=%s event_id=%s request_id=%s external_user_id=%s outcome=%s", tenantID, h.EventId, p.RequestID, p.ExternalUserID, res.Outcome)
			}

			return event.NewSuccessResponse()
		})

		if err := cli.Start(ctx); err != nil {
			log.Fatal(err)
		}
		defer cli.Close()
	}

	<-ctx.Done()
}

func runWeComPoller(ctx context.Context, pool *pgxpool.Pool, store attendanceintegrations.Store, tenantID string, corpID string, corpSecret string, intervalSeconds int, lookbackSeconds int) {
	if intervalSeconds <= 0 {
		intervalSeconds = 30
	}
	if lookbackSeconds <= 0 {
		lookbackSeconds = 600
	}

	client := attendanceintegrations.NewWeComClient(corpID, corpSecret, nil)
	tokenSource := attendanceintegrations.NewWeComTokenSource(client)

	runOnce := func() {
		userIDs, err := listActiveWeComUserIDs(ctx, pool, tenantID, 2000)
		if err != nil {
			log.Printf("wecom list active users error: tenant_id=%s err=%v", tenantID, err)
			return
		}
		if len(userIDs) == 0 {
			return
		}

		end := time.Now().UTC().Unix()
		start := end - int64(lookbackSeconds)
		if start <= 0 {
			start = 1
		}

		const batchSize = 100
		for i := 0; i < len(userIDs); i += batchSize {
			j := i + batchSize
			if j > len(userIDs) {
				j = len(userIDs)
			}
			batch := userIDs[i:j]

			token, err := tokenSource.Token(ctx)
			if err != nil {
				log.Printf("wecom token error: tenant_id=%s err=%v", tenantID, err)
				return
			}

			records, err := client.GetCheckinData(ctx, token, start, end, batch)
			if err != nil {
				var apiErr attendanceintegrations.WeComAPIError
				if errors.As(err, &apiErr) && (apiErr.Code == 40014 || apiErr.Code == 42001) {
					tokenSource.Invalidate()
				}
				log.Printf("wecom getcheckindata error: tenant_id=%s err=%v", tenantID, err)
				return
			}

			punches, err := attendanceintegrations.BuildWeComCheckinPunches(records)
			if err != nil {
				log.Printf("wecom build punches error: tenant_id=%s err=%v", tenantID, err)
				continue
			}

			for _, p := range punches {
				res, err := attendanceintegrations.IngestExternalPunch(ctx, store, tenantID, tenantID, p)
				if err != nil {
					log.Printf("wecom ingest error: tenant_id=%s request_id=%s external_user_id=%s err=%v", tenantID, p.RequestID, p.ExternalUserID, err)
					continue
				}
				log.Printf("wecom ingest: tenant_id=%s request_id=%s external_user_id=%s outcome=%s", tenantID, p.RequestID, p.ExternalUserID, res.Outcome)
			}
		}
	}

	runOnce()
	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runOnce()
		}
	}
}

func listActiveWeComUserIDs(ctx context.Context, pool *pgxpool.Pool, tenantID string, limit int) ([]string, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 200
	}
	if limit > 5000 {
		limit = 5000
	}

	rows, err := tx.Query(ctx, `
SELECT external_user_id
FROM person.external_identity_links
WHERE tenant_id = $1::uuid
  AND provider = 'WECOM'
  AND status = 'active'
ORDER BY last_seen_at DESC, external_user_id ASC
LIMIT $2::int
`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		u = stringsTrimSpaceOrEmpty(u)
		if u != "" {
			out = append(out, u)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func getenvIntDefault(key string, def int) int {
	raw := stringsTrimSpaceOrEmpty(os.Getenv(key))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

func stringsTrimSpaceOrEmpty(v string) string {
	return strings.TrimSpace(v)
}

func dbDSNFromEnv() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}

	host := getenvDefault("DB_HOST", "127.0.0.1")
	port := getenvDefault("DB_PORT", "5438")
	user := getenvDefault("DB_USER", "app")
	pass := getenvDefault("DB_PASSWORD", "app")
	name := getenvDefault("DB_NAME", "bugs_and_blossoms")
	sslmode := getenvDefault("DB_SSLMODE", "disable")

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, pass),
		Host:   host + ":" + port,
		Path:   "/" + name,
	}
	q := u.Query()
	q.Set("sslmode", sslmode)
	u.RawQuery = q.Encode()
	return u.String()
}

func getenvDefault(key string, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

package main

import (
	"context"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

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

	dingTalkClientID := os.Getenv("DINGTALK_CLIENT_ID")
	dingTalkClientSecret := os.Getenv("DINGTALK_CLIENT_SECRET")
	dingTalkCorpID := os.Getenv("DINGTALK_CORP_ID")
	if dingTalkClientID == "" || dingTalkClientSecret == "" || dingTalkCorpID == "" {
		log.Fatal("DINGTALK_CLIENT_ID, DINGTALK_CLIENT_SECRET, DINGTALK_CORP_ID are required")
	}

	// SDK 默认 logger 是 no-op；这里启用基础 stdout 日志，便于排障。
	logger.SetLogger(logger.NewStdTestLogger())

	store := attendanceintegrations.NewPGStore(pool)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	<-ctx.Done()
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

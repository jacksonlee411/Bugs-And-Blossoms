package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	defaultCubeboxFileRoot = ".local/cubebox/files"
	cubeboxStorageProvider = "localfs"
	cubeboxLinkRole        = "conversation_attachment"
)

var (
	cubeboxFileIDPattern = regexp.MustCompile(`^file_[0-9a-f-]{36}$`)
	cubeboxSHA256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type cubeboxBackfillOptions struct {
	URL     string
	Tenant  string
	DryRun  bool
	Timeout time.Duration
}

type cubeboxVerifyOptions struct {
	URL     string
	Tenant  string
	Timeout time.Duration
}

type cubeboxImportOptions struct {
	URL     string
	Tenant  string
	DryRun  bool
	RootDir string
	Timeout time.Duration
}

type cubeboxBackfillSummary struct {
	TenantID string `json:"tenant_id"`

	AssistantConversations int64 `json:"assistant_conversations"`
	CubeboxConversations   int64 `json:"cubebox_conversations"`

	AssistantTurns int64 `json:"assistant_turns"`
	CubeboxTurns   int64 `json:"cubebox_turns"`

	AssistantTasks int64 `json:"assistant_tasks"`
	CubeboxTasks   int64 `json:"cubebox_tasks"`

	AssistantIdempotency int64 `json:"assistant_idempotency"`
	CubeboxIdempotency   int64 `json:"cubebox_idempotency"`

	AssistantStateTransitions int64 `json:"assistant_state_transitions"`
	CubeboxStateTransitions   int64 `json:"cubebox_state_transitions"`

	AssistantTaskEvents int64 `json:"assistant_task_events"`
	CubeboxTaskEvents   int64 `json:"cubebox_task_events"`

	AssistantDispatchOutbox int64 `json:"assistant_task_dispatch_outbox"`
	CubeboxDispatchOutbox   int64 `json:"cubebox_task_dispatch_outbox"`
}

type cubeboxVerifySummary struct {
	TenantID     string   `json:"tenant_id"`
	Counts       any      `json:"counts"`
	TaskSnapshot any      `json:"task_snapshot"`
	Issues       []string `json:"issues"`
}

type cubeboxTaskSnapshotCheck struct {
	SourceRows           int64 `json:"source_rows"`
	TargetRows           int64 `json:"target_rows"`
	NullKnowledgeDigest  int64 `json:"null_knowledge_snapshot_digest"`
	NullRouteCatalog     int64 `json:"null_route_catalog_version"`
	NullResolverContract int64 `json:"null_resolver_contract_version"`
	NullContextTemplate  int64 `json:"null_context_template_version"`
	NullReplyGuidance    int64 `json:"null_reply_guidance_version"`
	NullPolicyContext    int64 `json:"null_policy_context_digest"`
	NullEffectivePolicy  int64 `json:"null_effective_policy_version"`
	NullResolvedSetID    int64 `json:"null_resolved_setid"`
	NullSetIDSource      int64 `json:"null_setid_source"`
	NullPrecheckDigest   int64 `json:"null_precheck_projection_digest"`
	NullMutationPolicy   int64 `json:"null_mutation_policy_version"`
}

type cubeboxFileIndex struct {
	Items []cubeboxLocalFileRecord `json:"items"`
}

type cubeboxLocalFileRecord struct {
	FileID         string `json:"file_id"`
	TenantID       string `json:"tenant_id"`
	ConversationID string `json:"conversation_id,omitempty"`
	FileName       string `json:"file_name"`
	MediaType      string `json:"media_type"`
	SizeBytes      int64  `json:"size_bytes"`
	SHA256         string `json:"sha256"`
	StorageKey     string `json:"storage_key"`
	UploadedBy     string `json:"uploaded_by"`
	UploadedAt     string `json:"uploaded_at"`
}

type cubeboxValidatedFileRecord struct {
	Record        cubeboxLocalFileRecord
	UploadedAt    time.Time
	StoragePath   string
	StorageSize   int64
	StorageSHA256 string
}

type cubeboxFileImportSummary struct {
	TenantID      string   `json:"tenant_id"`
	FilesImported int      `json:"files_imported"`
	LinksImported int      `json:"links_imported"`
	Issues        []string `json:"issues,omitempty"`
}

type cubeboxFileVerifySummary struct {
	TenantID             string   `json:"tenant_id"`
	IndexRecords         int      `json:"index_records"`
	IndexConversationRef int      `json:"index_conversation_refs"`
	DBFiles              int64    `json:"db_files"`
	DBLinks              int64    `json:"db_links"`
	Issues               []string `json:"issues,omitempty"`
}

func cubeboxBackfillAssistant(args []string) {
	opts := parseCubeboxBackfillOptions(args)
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, opts.URL)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	_ = tryEnsureRole(ctx, conn, "app_nobypassrls")

	tenantIDs, err := cubeboxResolveTenantIDs(ctx, conn, opts.Tenant)
	if err != nil {
		fatal(err)
	}
	if len(tenantIDs) == 0 {
		fatalf("no tenant selected for cubebox backfill")
	}

	summaries := make([]cubeboxBackfillSummary, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		summary, err := cubeboxBackfillTenant(ctx, conn, tenantID, opts.DryRun)
		if err != nil {
			fatal(fmt.Errorf("cubebox backfill tenant %s: %w", tenantID, err))
		}
		summaries = append(summaries, summary)
	}
	writeJSONToStdout(map[string]any{
		"dry_run":     opts.DryRun,
		"tenants":     summaries,
		"executed_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func cubeboxVerifyBackfill(args []string) {
	opts := parseCubeboxVerifyOptions(args, "cubebox-verify-backfill")
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, opts.URL)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	_ = tryEnsureRole(ctx, conn, "app_nobypassrls")

	tenantIDs, err := cubeboxResolveTenantIDs(ctx, conn, opts.Tenant)
	if err != nil {
		fatal(err)
	}
	if len(tenantIDs) == 0 {
		fatalf("no tenant selected for cubebox verify")
	}

	summaries := make([]cubeboxVerifySummary, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		summary, err := cubeboxVerifyBackfillTenant(ctx, conn, tenantID)
		if err != nil {
			fatal(fmt.Errorf("cubebox verify tenant %s: %w", tenantID, err))
		}
		if len(summary.Issues) != 0 {
			fatal(fmt.Errorf("cubebox verify tenant %s failed: %s", tenantID, strings.Join(summary.Issues, "; ")))
		}
		summaries = append(summaries, summary)
	}
	writeJSONToStdout(map[string]any{
		"tenants":     summaries,
		"executed_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func cubeboxImportLocalFiles(args []string) {
	opts := parseCubeboxImportOptions(args)
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, opts.URL)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	_ = tryEnsureRole(ctx, conn, "app_nobypassrls")

	index, err := cubeboxReadFileIndex(opts.RootDir)
	if err != nil {
		fatal(err)
	}

	tenantIDs, err := cubeboxResolveTenantIDs(ctx, conn, opts.Tenant)
	if err != nil {
		fatal(err)
	}
	if len(tenantIDs) == 0 {
		fatalf("no tenant selected for cubebox local file import")
	}

	summaries := make([]cubeboxFileImportSummary, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		records, issues := cubeboxValidateTenantFileRecords(index.Items, tenantID)
		if len(issues) != 0 {
			fatal(fmt.Errorf("cubebox local file import tenant %s validation failed: %s", tenantID, strings.Join(issues, "; ")))
		}
		validated, err := cubeboxValidateStorageObjects(records, opts.RootDir)
		if err != nil {
			fatal(fmt.Errorf("cubebox local file import tenant %s storage validation failed: %w", tenantID, err))
		}
		summary, err := cubeboxImportTenantFiles(ctx, conn, tenantID, validated, opts.DryRun)
		if err != nil {
			fatal(fmt.Errorf("cubebox local file import tenant %s failed: %w", tenantID, err))
		}
		summaries = append(summaries, summary)
	}

	writeJSONToStdout(map[string]any{
		"dry_run":     opts.DryRun,
		"root_dir":    opts.RootDir,
		"tenants":     summaries,
		"executed_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func cubeboxVerifyFileImport(args []string) {
	opts := parseCubeboxImportOptions(args)
	opts.DryRun = false

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, opts.URL)
	if err != nil {
		fatal(err)
	}
	defer conn.Close(context.Background())

	_ = tryEnsureRole(ctx, conn, "app_nobypassrls")

	index, err := cubeboxReadFileIndex(opts.RootDir)
	if err != nil {
		fatal(err)
	}

	tenantIDs, err := cubeboxResolveTenantIDs(ctx, conn, opts.Tenant)
	if err != nil {
		fatal(err)
	}
	if len(tenantIDs) == 0 {
		fatalf("no tenant selected for cubebox file verify")
	}

	summaries := make([]cubeboxFileVerifySummary, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		records, issues := cubeboxValidateTenantFileRecords(index.Items, tenantID)
		if len(issues) != 0 {
			fatal(fmt.Errorf("cubebox verify file import tenant %s validation failed: %s", tenantID, strings.Join(issues, "; ")))
		}
		validated, err := cubeboxValidateStorageObjects(records, opts.RootDir)
		if err != nil {
			fatal(fmt.Errorf("cubebox verify file import tenant %s storage validation failed: %w", tenantID, err))
		}
		summary, err := cubeboxVerifyTenantFiles(ctx, conn, tenantID, validated)
		if err != nil {
			fatal(fmt.Errorf("cubebox verify file import tenant %s failed: %w", tenantID, err))
		}
		if len(summary.Issues) != 0 {
			fatal(fmt.Errorf("cubebox verify file import tenant %s failed: %s", tenantID, strings.Join(summary.Issues, "; ")))
		}
		summaries = append(summaries, summary)
	}

	writeJSONToStdout(map[string]any{
		"root_dir":    opts.RootDir,
		"tenants":     summaries,
		"executed_at": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func parseCubeboxBackfillOptions(args []string) cubeboxBackfillOptions {
	fs := flag.NewFlagSet("cubebox-backfill-assistant", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts cubeboxBackfillOptions
	fs.StringVar(&opts.URL, "url", "", "postgres connection string")
	fs.StringVar(&opts.Tenant, "tenant", "", "tenant uuid; omit for all tenants with assistant data")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "validate and roll back instead of committing")
	fs.DurationVar(&opts.Timeout, "timeout", 2*time.Minute, "command timeout")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if strings.TrimSpace(opts.URL) == "" {
		fatalf("missing --url")
	}
	return opts
}

func parseCubeboxVerifyOptions(args []string, name string) cubeboxVerifyOptions {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts cubeboxVerifyOptions
	fs.StringVar(&opts.URL, "url", "", "postgres connection string")
	fs.StringVar(&opts.Tenant, "tenant", "", "tenant uuid; omit for all tenants with assistant data")
	fs.DurationVar(&opts.Timeout, "timeout", 2*time.Minute, "command timeout")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if strings.TrimSpace(opts.URL) == "" {
		fatalf("missing --url")
	}
	return opts
}

func parseCubeboxImportOptions(args []string) cubeboxImportOptions {
	fs := flag.NewFlagSet("cubebox-import-local-files", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts cubeboxImportOptions
	fs.StringVar(&opts.URL, "url", "", "postgres connection string")
	fs.StringVar(&opts.Tenant, "tenant", "", "tenant uuid; omit for all tenants found in index.json")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "validate and roll back instead of committing")
	fs.StringVar(&opts.RootDir, "root", defaultCubeboxFileRoot, "cubebox local file root directory")
	fs.DurationVar(&opts.Timeout, "timeout", 2*time.Minute, "command timeout")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if strings.TrimSpace(opts.URL) == "" {
		fatalf("missing --url")
	}
	if strings.TrimSpace(opts.RootDir) == "" {
		fatalf("missing --root")
	}
	return opts
}

func cubeboxResolveTenantIDs(ctx context.Context, conn *pgx.Conn, requestedTenant string) ([]string, error) {
	requestedTenant = strings.TrimSpace(requestedTenant)
	if requestedTenant != "" {
		if _, err := uuid.Parse(requestedTenant); err != nil {
			return nil, fmt.Errorf("invalid tenant uuid %q: %w", requestedTenant, err)
		}
		return []string{requestedTenant}, nil
	}

	rows, err := conn.Query(ctx, `
SELECT tenant_uuid::text
FROM (
  SELECT tenant_uuid FROM iam.assistant_conversations
  UNION
  SELECT tenant_uuid FROM iam.assistant_tasks
) AS tenants
ORDER BY tenant_uuid::text
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tenantIDs []string
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			return nil, err
		}
		tenantIDs = append(tenantIDs, tenantID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tenantIDs, nil
}

func cubeboxResolveTenantIDsFromIndex(items []cubeboxLocalFileRecord, requestedTenant string) ([]string, error) {
	requestedTenant = strings.TrimSpace(requestedTenant)
	if requestedTenant != "" {
		if _, err := uuid.Parse(requestedTenant); err != nil {
			return nil, fmt.Errorf("invalid tenant uuid %q: %w", requestedTenant, err)
		}
		return []string{requestedTenant}, nil
	}

	seen := make(map[string]struct{})
	tenantIDs := make([]string, 0)
	for _, item := range items {
		tenantID := strings.TrimSpace(item.TenantID)
		if tenantID == "" {
			return nil, errors.New("index.json contains empty tenant_id")
		}
		if _, err := uuid.Parse(tenantID); err != nil {
			return nil, fmt.Errorf("index.json contains invalid tenant_id %q: %w", tenantID, err)
		}
		if _, ok := seen[tenantID]; ok {
			continue
		}
		seen[tenantID] = struct{}{}
		tenantIDs = append(tenantIDs, tenantID)
	}
	sort.Strings(tenantIDs)
	return tenantIDs, nil
}

func cubeboxBackfillTenant(ctx context.Context, conn *pgx.Conn, tenantID string, dryRun bool) (cubeboxBackfillSummary, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return cubeboxBackfillSummary{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	_ = trySetRole(ctx, tx, "app_nobypassrls")
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return cubeboxBackfillSummary{}, err
	}

	if err := cubeboxBackfillBaseEntities(ctx, tx, tenantID); err != nil {
		return cubeboxBackfillSummary{}, err
	}
	if err := cubeboxRebuildAppendOnlyChains(ctx, tx, tenantID); err != nil {
		return cubeboxBackfillSummary{}, err
	}

	summary, err := cubeboxCollectBackfillSummary(ctx, tx, tenantID)
	if err != nil {
		return cubeboxBackfillSummary{}, err
	}
	if issues := cubeboxBackfillCountIssues(summary); len(issues) != 0 {
		return cubeboxBackfillSummary{}, fmt.Errorf("%s", strings.Join(issues, "; "))
	}

	if dryRun {
		return summary, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxBackfillSummary{}, err
	}
	return summary, nil
}

func cubeboxBackfillBaseEntities(ctx context.Context, tx pgx.Tx, tenantID string) error {
	stmts := []string{
		`
INSERT INTO iam.cubebox_conversations (
  tenant_uuid, conversation_id, actor_id, actor_role, state, current_phase, created_at, updated_at
)
SELECT tenant_uuid, conversation_id, actor_id, actor_role, state, current_phase, created_at, updated_at
FROM iam.assistant_conversations
WHERE tenant_uuid = $1::uuid
ON CONFLICT (tenant_uuid, conversation_id) DO UPDATE
SET actor_id = EXCLUDED.actor_id,
    actor_role = EXCLUDED.actor_role,
    state = EXCLUDED.state,
    current_phase = EXCLUDED.current_phase,
    created_at = EXCLUDED.created_at,
    updated_at = EXCLUDED.updated_at;
`,
		`
INSERT INTO iam.cubebox_turns (
  tenant_uuid, conversation_id, turn_id, user_input, state, phase, risk_tier, request_id, trace_id,
  policy_version, composition_version, mapping_version, intent_json, plan_json, candidates_json,
  candidate_options, resolved_candidate_id, selected_candidate_id, ambiguity_count, confidence,
  resolution_source, route_decision_json, clarification_json, dry_run_json, pending_draft_summary,
  missing_fields, commit_result_json, commit_reply, error_code, created_at, updated_at
)
SELECT tenant_uuid, conversation_id, turn_id, user_input, state, phase, risk_tier, request_id, trace_id,
       policy_version, composition_version, mapping_version, intent_json, plan_json, candidates_json,
       candidate_options, resolved_candidate_id, selected_candidate_id, ambiguity_count, confidence,
       resolution_source, route_decision_json, clarification_json, dry_run_json, pending_draft_summary,
       missing_fields, commit_result_json, commit_reply, error_code, created_at, updated_at
FROM iam.assistant_turns
WHERE tenant_uuid = $1::uuid
ON CONFLICT (tenant_uuid, conversation_id, turn_id) DO UPDATE
SET user_input = EXCLUDED.user_input,
    state = EXCLUDED.state,
    phase = EXCLUDED.phase,
    risk_tier = EXCLUDED.risk_tier,
    request_id = EXCLUDED.request_id,
    trace_id = EXCLUDED.trace_id,
    policy_version = EXCLUDED.policy_version,
    composition_version = EXCLUDED.composition_version,
    mapping_version = EXCLUDED.mapping_version,
    intent_json = EXCLUDED.intent_json,
    plan_json = EXCLUDED.plan_json,
    candidates_json = EXCLUDED.candidates_json,
    candidate_options = EXCLUDED.candidate_options,
    resolved_candidate_id = EXCLUDED.resolved_candidate_id,
    selected_candidate_id = EXCLUDED.selected_candidate_id,
    ambiguity_count = EXCLUDED.ambiguity_count,
    confidence = EXCLUDED.confidence,
    resolution_source = EXCLUDED.resolution_source,
    route_decision_json = EXCLUDED.route_decision_json,
    clarification_json = EXCLUDED.clarification_json,
    dry_run_json = EXCLUDED.dry_run_json,
    pending_draft_summary = EXCLUDED.pending_draft_summary,
    missing_fields = EXCLUDED.missing_fields,
    commit_result_json = EXCLUDED.commit_result_json,
    commit_reply = EXCLUDED.commit_reply,
    error_code = EXCLUDED.error_code,
    created_at = EXCLUDED.created_at,
    updated_at = EXCLUDED.updated_at;
`,
		`
INSERT INTO iam.cubebox_tasks (
  tenant_uuid, task_id, conversation_id, turn_id, task_type, request_id, request_hash, workflow_id,
  status, dispatch_status, dispatch_attempt, dispatch_deadline_at, attempt, max_attempts,
  last_error_code, trace_id, intent_schema_version, compiler_contract_version, capability_map_version,
  skill_manifest_digest, context_hash, intent_hash, plan_hash,
  knowledge_snapshot_digest, route_catalog_version, resolver_contract_version, context_template_version,
  reply_guidance_version, policy_context_digest, effective_policy_version, resolved_setid,
  setid_source, precheck_projection_digest, mutation_policy_version,
  submitted_at, cancel_requested_at, completed_at, created_at, updated_at
)
SELECT tenant_uuid, task_id, conversation_id, turn_id, task_type, request_id, request_hash, workflow_id,
       status, dispatch_status, dispatch_attempt, dispatch_deadline_at, attempt, max_attempts,
       last_error_code, trace_id, intent_schema_version, compiler_contract_version, capability_map_version,
       skill_manifest_digest, context_hash, intent_hash, plan_hash,
       NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL,
       submitted_at, cancel_requested_at, completed_at, created_at, updated_at
FROM iam.assistant_tasks
WHERE tenant_uuid = $1::uuid
ON CONFLICT (tenant_uuid, task_id) DO UPDATE
SET conversation_id = EXCLUDED.conversation_id,
    turn_id = EXCLUDED.turn_id,
    task_type = EXCLUDED.task_type,
    request_id = EXCLUDED.request_id,
    request_hash = EXCLUDED.request_hash,
    workflow_id = EXCLUDED.workflow_id,
    status = EXCLUDED.status,
    dispatch_status = EXCLUDED.dispatch_status,
    dispatch_attempt = EXCLUDED.dispatch_attempt,
    dispatch_deadline_at = EXCLUDED.dispatch_deadline_at,
    attempt = EXCLUDED.attempt,
    max_attempts = EXCLUDED.max_attempts,
    last_error_code = EXCLUDED.last_error_code,
    trace_id = EXCLUDED.trace_id,
    intent_schema_version = EXCLUDED.intent_schema_version,
    compiler_contract_version = EXCLUDED.compiler_contract_version,
    capability_map_version = EXCLUDED.capability_map_version,
    skill_manifest_digest = EXCLUDED.skill_manifest_digest,
    context_hash = EXCLUDED.context_hash,
    intent_hash = EXCLUDED.intent_hash,
    plan_hash = EXCLUDED.plan_hash,
    knowledge_snapshot_digest = EXCLUDED.knowledge_snapshot_digest,
    route_catalog_version = EXCLUDED.route_catalog_version,
    resolver_contract_version = EXCLUDED.resolver_contract_version,
    context_template_version = EXCLUDED.context_template_version,
    reply_guidance_version = EXCLUDED.reply_guidance_version,
    policy_context_digest = EXCLUDED.policy_context_digest,
    effective_policy_version = EXCLUDED.effective_policy_version,
    resolved_setid = EXCLUDED.resolved_setid,
    setid_source = EXCLUDED.setid_source,
    precheck_projection_digest = EXCLUDED.precheck_projection_digest,
    mutation_policy_version = EXCLUDED.mutation_policy_version,
    submitted_at = EXCLUDED.submitted_at,
    cancel_requested_at = EXCLUDED.cancel_requested_at,
    completed_at = EXCLUDED.completed_at,
    created_at = EXCLUDED.created_at,
    updated_at = EXCLUDED.updated_at;
`,
		`
INSERT INTO iam.cubebox_idempotency (
  tenant_uuid, conversation_id, turn_id, turn_action, request_id, request_hash, status,
  http_status, error_code, response_body, response_hash, created_at, finalized_at, expires_at
)
SELECT tenant_uuid, conversation_id, turn_id, turn_action, request_id, request_hash, status,
       http_status, error_code, response_body, response_hash, created_at, finalized_at, expires_at
FROM iam.assistant_idempotency
WHERE tenant_uuid = $1::uuid
ON CONFLICT (tenant_uuid, conversation_id, turn_id, turn_action, request_id) DO UPDATE
SET request_hash = EXCLUDED.request_hash,
    status = EXCLUDED.status,
    http_status = EXCLUDED.http_status,
    error_code = EXCLUDED.error_code,
    response_body = EXCLUDED.response_body,
    response_hash = EXCLUDED.response_hash,
    created_at = EXCLUDED.created_at,
    finalized_at = EXCLUDED.finalized_at,
    expires_at = EXCLUDED.expires_at;
`,
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(ctx, stmt, tenantID); err != nil {
			return err
		}
	}
	return nil
}

func cubeboxRebuildAppendOnlyChains(ctx context.Context, tx pgx.Tx, tenantID string) error {
	deleteStmts := []string{
		`DELETE FROM iam.cubebox_state_transitions WHERE tenant_uuid = $1::uuid;`,
		`DELETE FROM iam.cubebox_task_events WHERE tenant_uuid = $1::uuid;`,
		`DELETE FROM iam.cubebox_task_dispatch_outbox WHERE tenant_uuid = $1::uuid;`,
	}
	for _, stmt := range deleteStmts {
		if _, err := tx.Exec(ctx, stmt, tenantID); err != nil {
			return err
		}
	}

	insertStmts := []string{
		`
INSERT INTO iam.cubebox_state_transitions (
  tenant_uuid, conversation_id, turn_id, turn_action, request_id, trace_id,
  from_state, to_state, from_phase, to_phase, reason_code, actor_id, changed_at
)
SELECT tenant_uuid, conversation_id, turn_id, turn_action, request_id, trace_id,
       from_state, to_state, from_phase, to_phase, reason_code, actor_id, changed_at
FROM iam.assistant_state_transitions
WHERE tenant_uuid = $1::uuid
ORDER BY changed_at ASC, id ASC;
`,
		`
INSERT INTO iam.cubebox_task_events (
  tenant_uuid, task_id, from_status, to_status, event_type, error_code, payload, occurred_at
)
SELECT tenant_uuid, task_id, from_status, to_status, event_type, error_code, payload, occurred_at
FROM iam.assistant_task_events
WHERE tenant_uuid = $1::uuid
ORDER BY occurred_at ASC, id ASC;
`,
		`
INSERT INTO iam.cubebox_task_dispatch_outbox (
  tenant_uuid, task_id, workflow_id, status, attempt, next_retry_at, created_at, updated_at
)
SELECT tenant_uuid, task_id, workflow_id, status, attempt, next_retry_at, created_at, updated_at
FROM iam.assistant_task_dispatch_outbox
WHERE tenant_uuid = $1::uuid
ORDER BY created_at ASC, id ASC;
`,
	}
	for _, stmt := range insertStmts {
		if _, err := tx.Exec(ctx, stmt, tenantID); err != nil {
			return err
		}
	}
	return nil
}

func cubeboxCollectBackfillSummary(ctx context.Context, tx pgx.Tx, tenantID string) (cubeboxBackfillSummary, error) {
	summary := cubeboxBackfillSummary{TenantID: tenantID}
	countTargets := []struct {
		query string
		dest  *int64
	}{
		{`SELECT count(*) FROM iam.assistant_conversations WHERE tenant_uuid = $1::uuid;`, &summary.AssistantConversations},
		{`SELECT count(*) FROM iam.cubebox_conversations WHERE tenant_uuid = $1::uuid;`, &summary.CubeboxConversations},
		{`SELECT count(*) FROM iam.assistant_turns WHERE tenant_uuid = $1::uuid;`, &summary.AssistantTurns},
		{`SELECT count(*) FROM iam.cubebox_turns WHERE tenant_uuid = $1::uuid;`, &summary.CubeboxTurns},
		{`SELECT count(*) FROM iam.assistant_tasks WHERE tenant_uuid = $1::uuid;`, &summary.AssistantTasks},
		{`SELECT count(*) FROM iam.cubebox_tasks WHERE tenant_uuid = $1::uuid;`, &summary.CubeboxTasks},
		{`SELECT count(*) FROM iam.assistant_idempotency WHERE tenant_uuid = $1::uuid;`, &summary.AssistantIdempotency},
		{`SELECT count(*) FROM iam.cubebox_idempotency WHERE tenant_uuid = $1::uuid;`, &summary.CubeboxIdempotency},
		{`SELECT count(*) FROM iam.assistant_state_transitions WHERE tenant_uuid = $1::uuid;`, &summary.AssistantStateTransitions},
		{`SELECT count(*) FROM iam.cubebox_state_transitions WHERE tenant_uuid = $1::uuid;`, &summary.CubeboxStateTransitions},
		{`SELECT count(*) FROM iam.assistant_task_events WHERE tenant_uuid = $1::uuid;`, &summary.AssistantTaskEvents},
		{`SELECT count(*) FROM iam.cubebox_task_events WHERE tenant_uuid = $1::uuid;`, &summary.CubeboxTaskEvents},
		{`SELECT count(*) FROM iam.assistant_task_dispatch_outbox WHERE tenant_uuid = $1::uuid;`, &summary.AssistantDispatchOutbox},
		{`SELECT count(*) FROM iam.cubebox_task_dispatch_outbox WHERE tenant_uuid = $1::uuid;`, &summary.CubeboxDispatchOutbox},
	}
	for _, target := range countTargets {
		if err := tx.QueryRow(ctx, target.query, tenantID).Scan(target.dest); err != nil {
			return cubeboxBackfillSummary{}, err
		}
	}
	return summary, nil
}

func cubeboxVerifyBackfillTenant(ctx context.Context, conn *pgx.Conn, tenantID string) (cubeboxVerifySummary, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return cubeboxVerifySummary{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	_ = trySetRole(ctx, tx, "app_nobypassrls")
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return cubeboxVerifySummary{}, err
	}

	counts, err := cubeboxCollectBackfillSummary(ctx, tx, tenantID)
	if err != nil {
		return cubeboxVerifySummary{}, err
	}
	taskSnapshot, err := cubeboxCollectTaskSnapshotCheck(ctx, tx, tenantID)
	if err != nil {
		return cubeboxVerifySummary{}, err
	}
	issues, err := cubeboxCollectBackfillIssues(ctx, tx, tenantID, counts, taskSnapshot)
	if err != nil {
		return cubeboxVerifySummary{}, err
	}

	return cubeboxVerifySummary{
		TenantID:     tenantID,
		Counts:       counts,
		TaskSnapshot: taskSnapshot,
		Issues:       issues,
	}, nil
}

func cubeboxCollectTaskSnapshotCheck(ctx context.Context, tx pgx.Tx, tenantID string) (cubeboxTaskSnapshotCheck, error) {
	var out cubeboxTaskSnapshotCheck
	err := tx.QueryRow(ctx, `
SELECT
  (SELECT count(*) FROM iam.assistant_tasks WHERE tenant_uuid = $1::uuid) AS source_rows,
  (SELECT count(*) FROM iam.cubebox_tasks WHERE tenant_uuid = $1::uuid) AS target_rows,
  (SELECT count(*) FROM iam.cubebox_tasks WHERE tenant_uuid = $1::uuid AND knowledge_snapshot_digest IS NULL) AS null_knowledge_snapshot_digest,
  (SELECT count(*) FROM iam.cubebox_tasks WHERE tenant_uuid = $1::uuid AND route_catalog_version IS NULL) AS null_route_catalog_version,
  (SELECT count(*) FROM iam.cubebox_tasks WHERE tenant_uuid = $1::uuid AND resolver_contract_version IS NULL) AS null_resolver_contract_version,
  (SELECT count(*) FROM iam.cubebox_tasks WHERE tenant_uuid = $1::uuid AND context_template_version IS NULL) AS null_context_template_version,
  (SELECT count(*) FROM iam.cubebox_tasks WHERE tenant_uuid = $1::uuid AND reply_guidance_version IS NULL) AS null_reply_guidance_version,
  (SELECT count(*) FROM iam.cubebox_tasks WHERE tenant_uuid = $1::uuid AND policy_context_digest IS NULL) AS null_policy_context_digest,
  (SELECT count(*) FROM iam.cubebox_tasks WHERE tenant_uuid = $1::uuid AND effective_policy_version IS NULL) AS null_effective_policy_version,
  (SELECT count(*) FROM iam.cubebox_tasks WHERE tenant_uuid = $1::uuid AND resolved_setid IS NULL) AS null_resolved_setid,
  (SELECT count(*) FROM iam.cubebox_tasks WHERE tenant_uuid = $1::uuid AND setid_source IS NULL) AS null_setid_source,
  (SELECT count(*) FROM iam.cubebox_tasks WHERE tenant_uuid = $1::uuid AND precheck_projection_digest IS NULL) AS null_precheck_projection_digest,
  (SELECT count(*) FROM iam.cubebox_tasks WHERE tenant_uuid = $1::uuid AND mutation_policy_version IS NULL) AS null_mutation_policy_version
`, tenantID).Scan(
		&out.SourceRows,
		&out.TargetRows,
		&out.NullKnowledgeDigest,
		&out.NullRouteCatalog,
		&out.NullResolverContract,
		&out.NullContextTemplate,
		&out.NullReplyGuidance,
		&out.NullPolicyContext,
		&out.NullEffectivePolicy,
		&out.NullResolvedSetID,
		&out.NullSetIDSource,
		&out.NullPrecheckDigest,
		&out.NullMutationPolicy,
	)
	return out, err
}

func cubeboxCollectBackfillIssues(ctx context.Context, tx pgx.Tx, tenantID string, counts cubeboxBackfillSummary, taskSnapshot cubeboxTaskSnapshotCheck) ([]string, error) {
	issues := cubeboxBackfillCountIssues(counts)

	if taskSnapshot.SourceRows != taskSnapshot.TargetRows {
		issues = append(issues, fmt.Sprintf("tenant %s task snapshot row count mismatch: assistant=%d cubebox=%d", tenantID, taskSnapshot.SourceRows, taskSnapshot.TargetRows))
	}
	if taskSnapshot.TargetRows != taskSnapshot.NullKnowledgeDigest ||
		taskSnapshot.TargetRows != taskSnapshot.NullRouteCatalog ||
		taskSnapshot.TargetRows != taskSnapshot.NullResolverContract ||
		taskSnapshot.TargetRows != taskSnapshot.NullContextTemplate ||
		taskSnapshot.TargetRows != taskSnapshot.NullReplyGuidance ||
		taskSnapshot.TargetRows != taskSnapshot.NullPolicyContext ||
		taskSnapshot.TargetRows != taskSnapshot.NullEffectivePolicy ||
		taskSnapshot.TargetRows != taskSnapshot.NullResolvedSetID ||
		taskSnapshot.TargetRows != taskSnapshot.NullSetIDSource ||
		taskSnapshot.TargetRows != taskSnapshot.NullPrecheckDigest ||
		taskSnapshot.TargetRows != taskSnapshot.NullMutationPolicy {
		issues = append(issues, fmt.Sprintf("tenant %s task nullable snapshot fields are not fully NULL for history rows", tenantID))
	}

	existenceChecks := []struct {
		label string
		query string
	}{
		{
			label: "conversation primary key set mismatch",
			query: `
SELECT EXISTS (
  SELECT conversation_id
  FROM iam.assistant_conversations
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT conversation_id
  FROM iam.cubebox_conversations
  WHERE tenant_uuid = $1::uuid
) OR EXISTS (
  SELECT conversation_id
  FROM iam.cubebox_conversations
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT conversation_id
  FROM iam.assistant_conversations
  WHERE tenant_uuid = $1::uuid
);`,
		},
		{
			label: "turn primary key set mismatch",
			query: `
SELECT EXISTS (
  SELECT conversation_id, turn_id
  FROM iam.assistant_turns
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT conversation_id, turn_id
  FROM iam.cubebox_turns
  WHERE tenant_uuid = $1::uuid
) OR EXISTS (
  SELECT conversation_id, turn_id
  FROM iam.cubebox_turns
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT conversation_id, turn_id
  FROM iam.assistant_turns
  WHERE tenant_uuid = $1::uuid
);`,
		},
		{
			label: "task primary key set mismatch",
			query: `
SELECT EXISTS (
  SELECT task_id
  FROM iam.assistant_tasks
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT task_id
  FROM iam.cubebox_tasks
  WHERE tenant_uuid = $1::uuid
) OR EXISTS (
  SELECT task_id
  FROM iam.cubebox_tasks
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT task_id
  FROM iam.assistant_tasks
  WHERE tenant_uuid = $1::uuid
);`,
		},
		{
			label: "idempotency primary key set mismatch",
			query: `
SELECT EXISTS (
  SELECT conversation_id, turn_id, turn_action, request_id
  FROM iam.assistant_idempotency
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT conversation_id, turn_id, turn_action, request_id
  FROM iam.cubebox_idempotency
  WHERE tenant_uuid = $1::uuid
) OR EXISTS (
  SELECT conversation_id, turn_id, turn_action, request_id
  FROM iam.cubebox_idempotency
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT conversation_id, turn_id, turn_action, request_id
  FROM iam.assistant_idempotency
  WHERE tenant_uuid = $1::uuid
);`,
		},
		{
			label: "task request/workflow/status mismatch",
			query: `
SELECT EXISTS (
  SELECT task_id, request_id, workflow_id, status, task_type, dispatch_status, last_error_code
  FROM iam.assistant_tasks
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT task_id, request_id, workflow_id, status, task_type, dispatch_status, last_error_code
  FROM iam.cubebox_tasks
  WHERE tenant_uuid = $1::uuid
) OR EXISTS (
  SELECT task_id, request_id, workflow_id, status, task_type, dispatch_status, last_error_code
  FROM iam.cubebox_tasks
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT task_id, request_id, workflow_id, status, task_type, dispatch_status, last_error_code
  FROM iam.assistant_tasks
  WHERE tenant_uuid = $1::uuid
);`,
		},
		{
			label: "append-only state transition content mismatch",
			query: `
SELECT EXISTS (
  SELECT conversation_id, COALESCE(turn_id, ''), COALESCE(turn_action, ''), request_id, trace_id, from_state, to_state, from_phase, to_phase, COALESCE(reason_code, ''), actor_id, changed_at
  FROM iam.assistant_state_transitions
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT conversation_id, COALESCE(turn_id, ''), COALESCE(turn_action, ''), request_id, trace_id, from_state, to_state, from_phase, to_phase, COALESCE(reason_code, ''), actor_id, changed_at
  FROM iam.cubebox_state_transitions
  WHERE tenant_uuid = $1::uuid
) OR EXISTS (
  SELECT conversation_id, COALESCE(turn_id, ''), COALESCE(turn_action, ''), request_id, trace_id, from_state, to_state, from_phase, to_phase, COALESCE(reason_code, ''), actor_id, changed_at
  FROM iam.cubebox_state_transitions
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT conversation_id, COALESCE(turn_id, ''), COALESCE(turn_action, ''), request_id, trace_id, from_state, to_state, from_phase, to_phase, COALESCE(reason_code, ''), actor_id, changed_at
  FROM iam.assistant_state_transitions
  WHERE tenant_uuid = $1::uuid
);`,
		},
		{
			label: "append-only task event content mismatch",
			query: `
SELECT EXISTS (
  SELECT task_id, COALESCE(from_status, ''), to_status, event_type, COALESCE(error_code, ''), COALESCE(payload::text, ''), occurred_at
  FROM iam.assistant_task_events
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT task_id, COALESCE(from_status, ''), to_status, event_type, COALESCE(error_code, ''), COALESCE(payload::text, ''), occurred_at
  FROM iam.cubebox_task_events
  WHERE tenant_uuid = $1::uuid
) OR EXISTS (
  SELECT task_id, COALESCE(from_status, ''), to_status, event_type, COALESCE(error_code, ''), COALESCE(payload::text, ''), occurred_at
  FROM iam.cubebox_task_events
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT task_id, COALESCE(from_status, ''), to_status, event_type, COALESCE(error_code, ''), COALESCE(payload::text, ''), occurred_at
  FROM iam.assistant_task_events
  WHERE tenant_uuid = $1::uuid
);`,
		},
		{
			label: "append-only dispatch outbox content mismatch",
			query: `
SELECT EXISTS (
  SELECT task_id, workflow_id, status, attempt, next_retry_at, created_at, updated_at
  FROM iam.assistant_task_dispatch_outbox
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT task_id, workflow_id, status, attempt, next_retry_at, created_at, updated_at
  FROM iam.cubebox_task_dispatch_outbox
  WHERE tenant_uuid = $1::uuid
) OR EXISTS (
  SELECT task_id, workflow_id, status, attempt, next_retry_at, created_at, updated_at
  FROM iam.cubebox_task_dispatch_outbox
  WHERE tenant_uuid = $1::uuid
  EXCEPT
  SELECT task_id, workflow_id, status, attempt, next_retry_at, created_at, updated_at
  FROM iam.assistant_task_dispatch_outbox
  WHERE tenant_uuid = $1::uuid
);`,
		},
	}

	for _, check := range existenceChecks {
		var mismatch bool
		if err := tx.QueryRow(ctx, check.query, tenantID).Scan(&mismatch); err != nil {
			return nil, err
		}
		if mismatch {
			issues = append(issues, fmt.Sprintf("tenant %s %s", tenantID, check.label))
		}
	}
	return issues, nil
}

func cubeboxBackfillCountIssues(summary cubeboxBackfillSummary) []string {
	var issues []string
	pairs := []struct {
		label     string
		assistant int64
		cubebox   int64
	}{
		{"conversations", summary.AssistantConversations, summary.CubeboxConversations},
		{"turns", summary.AssistantTurns, summary.CubeboxTurns},
		{"tasks", summary.AssistantTasks, summary.CubeboxTasks},
		{"idempotency", summary.AssistantIdempotency, summary.CubeboxIdempotency},
		{"state_transitions", summary.AssistantStateTransitions, summary.CubeboxStateTransitions},
		{"task_events", summary.AssistantTaskEvents, summary.CubeboxTaskEvents},
		{"task_dispatch_outbox", summary.AssistantDispatchOutbox, summary.CubeboxDispatchOutbox},
	}
	for _, pair := range pairs {
		if pair.assistant != pair.cubebox {
			issues = append(issues, fmt.Sprintf("tenant %s %s count mismatch: assistant=%d cubebox=%d", summary.TenantID, pair.label, pair.assistant, pair.cubebox))
		}
	}
	return issues
}

func cubeboxReadFileIndex(rootDir string) (cubeboxFileIndex, error) {
	indexPath := filepath.Join(rootDir, "index.json")
	raw, err := os.ReadFile(indexPath)
	if err != nil {
		return cubeboxFileIndex{}, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return cubeboxFileIndex{}, nil
	}
	var index cubeboxFileIndex
	if err := json.Unmarshal(raw, &index); err != nil {
		return cubeboxFileIndex{}, err
	}
	return index, nil
}

func cubeboxValidateTenantFileRecords(all []cubeboxLocalFileRecord, tenantID string) ([]cubeboxLocalFileRecord, []string) {
	items := make([]cubeboxLocalFileRecord, 0)
	var issues []string
	seenFileIDs := make(map[string]struct{})
	seenStorageKeys := make(map[string]struct{})

	for _, item := range all {
		if strings.TrimSpace(item.TenantID) != tenantID {
			continue
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].UploadedAt == items[j].UploadedAt {
			return items[i].FileID < items[j].FileID
		}
		return items[i].UploadedAt < items[j].UploadedAt
	})

	for _, item := range items {
		if _, err := uuid.Parse(item.TenantID); err != nil {
			issues = append(issues, fmt.Sprintf("file %s has invalid tenant_id", item.FileID))
		}
		if !cubeboxFileIDPattern.MatchString(item.FileID) {
			issues = append(issues, fmt.Sprintf("file %s has invalid file_id format", item.FileID))
		}
		if strings.TrimSpace(item.FileName) == "" {
			issues = append(issues, fmt.Sprintf("file %s missing file_name", item.FileID))
		}
		if strings.TrimSpace(item.MediaType) == "" {
			issues = append(issues, fmt.Sprintf("file %s missing media_type", item.FileID))
		}
		if item.SizeBytes <= 0 {
			issues = append(issues, fmt.Sprintf("file %s size_bytes must be positive", item.FileID))
		}
		if !cubeboxSHA256Pattern.MatchString(strings.TrimSpace(item.SHA256)) {
			issues = append(issues, fmt.Sprintf("file %s has invalid sha256", item.FileID))
		}
		if strings.TrimSpace(item.StorageKey) == "" {
			issues = append(issues, fmt.Sprintf("file %s missing storage_key", item.FileID))
		}
		if strings.TrimSpace(item.UploadedBy) == "" {
			issues = append(issues, fmt.Sprintf("file %s missing uploaded_by", item.FileID))
		}
		if _, err := time.Parse(time.RFC3339, strings.TrimSpace(item.UploadedAt)); err != nil {
			if _, errNano := time.Parse(time.RFC3339Nano, strings.TrimSpace(item.UploadedAt)); errNano != nil {
				issues = append(issues, fmt.Sprintf("file %s has invalid uploaded_at", item.FileID))
			}
		}
		if _, ok := seenFileIDs[item.FileID]; ok {
			issues = append(issues, fmt.Sprintf("file %s duplicated in index", item.FileID))
		}
		seenFileIDs[item.FileID] = struct{}{}
		if _, ok := seenStorageKeys[item.StorageKey]; ok {
			issues = append(issues, fmt.Sprintf("file %s duplicated storage_key %s", item.FileID, item.StorageKey))
		}
		seenStorageKeys[item.StorageKey] = struct{}{}
	}

	return items, issues
}

func cubeboxValidateStorageObjects(records []cubeboxLocalFileRecord, rootDir string) ([]cubeboxValidatedFileRecord, error) {
	validated := make([]cubeboxValidatedFileRecord, 0, len(records))
	for _, item := range records {
		uploadedAt, err := parseRFC3339Flexible(item.UploadedAt)
		if err != nil {
			return nil, fmt.Errorf("file %s invalid uploaded_at: %w", item.FileID, err)
		}
		storagePath := filepath.Join(rootDir, "objects", filepath.FromSlash(item.StorageKey))
		info, err := os.Stat(storagePath)
		if err != nil {
			return nil, fmt.Errorf("file %s storage object missing: %w", item.FileID, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("file %s storage object is a directory", item.FileID)
		}
		if info.Size() != item.SizeBytes {
			return nil, fmt.Errorf("file %s size mismatch: index=%d object=%d", item.FileID, item.SizeBytes, info.Size())
		}
		sum, err := cubeboxComputeSHA256(storagePath)
		if err != nil {
			return nil, fmt.Errorf("file %s sha256 compute failed: %w", item.FileID, err)
		}
		if sum != item.SHA256 {
			return nil, fmt.Errorf("file %s sha256 mismatch: index=%s object=%s", item.FileID, item.SHA256, sum)
		}
		validated = append(validated, cubeboxValidatedFileRecord{
			Record:        item,
			UploadedAt:    uploadedAt,
			StoragePath:   storagePath,
			StorageSize:   info.Size(),
			StorageSHA256: sum,
		})
	}
	return validated, nil
}

func cubeboxImportTenantFiles(ctx context.Context, conn *pgx.Conn, tenantID string, records []cubeboxValidatedFileRecord, dryRun bool) (cubeboxFileImportSummary, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return cubeboxFileImportSummary{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	_ = trySetRole(ctx, tx, "app_nobypassrls")
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return cubeboxFileImportSummary{}, err
	}

	if err := cubeboxEnsureTenantExists(ctx, tx, tenantID); err != nil {
		return cubeboxFileImportSummary{}, err
	}

	filesImported := 0
	linksImported := 0
	for _, item := range records {
		if err := cubeboxEnsureConversationExists(ctx, tx, tenantID, item.Record.ConversationID); err != nil {
			return cubeboxFileImportSummary{}, err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO iam.cubebox_files (
  tenant_uuid, file_id, storage_provider, storage_key, file_name, media_type,
  size_bytes, sha256, scan_status, scan_error_code, uploaded_by, uploaded_at, updated_at
)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, 'ready', NULL, $9, $10, $10)
ON CONFLICT (tenant_uuid, file_id) DO UPDATE
SET storage_provider = EXCLUDED.storage_provider,
    storage_key = EXCLUDED.storage_key,
    file_name = EXCLUDED.file_name,
    media_type = EXCLUDED.media_type,
    size_bytes = EXCLUDED.size_bytes,
    sha256 = EXCLUDED.sha256,
    scan_status = EXCLUDED.scan_status,
    scan_error_code = EXCLUDED.scan_error_code,
    uploaded_by = EXCLUDED.uploaded_by,
    uploaded_at = EXCLUDED.uploaded_at,
    updated_at = EXCLUDED.updated_at;
`, tenantID, item.Record.FileID, cubeboxStorageProvider, item.Record.StorageKey, item.Record.FileName, item.Record.MediaType, item.Record.SizeBytes, item.Record.SHA256, item.Record.UploadedBy, item.UploadedAt); err != nil {
			return cubeboxFileImportSummary{}, err
		}
		filesImported++

		if strings.TrimSpace(item.Record.ConversationID) == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO iam.cubebox_file_links (
  tenant_uuid, file_id, conversation_id, turn_id, link_role, created_by, created_at
)
VALUES ($1::uuid, $2, $3, NULL, $4, $5, $6)
ON CONFLICT (tenant_uuid, file_id, conversation_id, link_role) WHERE turn_id IS NULL
DO UPDATE SET created_by = EXCLUDED.created_by, created_at = EXCLUDED.created_at;
`, tenantID, item.Record.FileID, item.Record.ConversationID, cubeboxLinkRole, item.Record.UploadedBy, item.UploadedAt); err != nil {
			return cubeboxFileImportSummary{}, err
		}
		linksImported++
	}

	summary := cubeboxFileImportSummary{
		TenantID:      tenantID,
		FilesImported: filesImported,
		LinksImported: linksImported,
	}
	if dryRun {
		return summary, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return cubeboxFileImportSummary{}, err
	}
	return summary, nil
}

func cubeboxVerifyTenantFiles(ctx context.Context, conn *pgx.Conn, tenantID string, records []cubeboxValidatedFileRecord) (cubeboxFileVerifySummary, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return cubeboxFileVerifySummary{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	_ = trySetRole(ctx, tx, "app_nobypassrls")
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return cubeboxFileVerifySummary{}, err
	}

	summary := cubeboxFileVerifySummary{TenantID: tenantID}
	summary.IndexRecords = len(records)
	for _, item := range records {
		if strings.TrimSpace(item.Record.ConversationID) != "" {
			summary.IndexConversationRef++
		}
	}

	if err := tx.QueryRow(ctx, `SELECT count(*) FROM iam.cubebox_files WHERE tenant_uuid = $1::uuid;`, tenantID).Scan(&summary.DBFiles); err != nil {
		return cubeboxFileVerifySummary{}, err
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM iam.cubebox_file_links WHERE tenant_uuid = $1::uuid AND link_role = $2;`, tenantID, cubeboxLinkRole).Scan(&summary.DBLinks); err != nil {
		return cubeboxFileVerifySummary{}, err
	}

	if summary.DBFiles != int64(summary.IndexRecords) {
		summary.Issues = append(summary.Issues, fmt.Sprintf("tenant %s cubebox_files count mismatch: index=%d db=%d", tenantID, summary.IndexRecords, summary.DBFiles))
	}
	if summary.DBLinks != int64(summary.IndexConversationRef) {
		summary.Issues = append(summary.Issues, fmt.Sprintf("tenant %s cubebox_file_links count mismatch: index=%d db=%d", tenantID, summary.IndexConversationRef, summary.DBLinks))
	}

	for _, item := range records {
		var (
			storageKey  string
			fileName    string
			mediaType   string
			sizeBytes   int64
			sha256Value string
			uploadedBy  string
			uploadedAt  time.Time
			linkCount   int64
		)
		err := tx.QueryRow(ctx, `
SELECT storage_key, file_name, media_type, size_bytes, sha256, uploaded_by, uploaded_at
FROM iam.cubebox_files
WHERE tenant_uuid = $1::uuid AND file_id = $2;
`, tenantID, item.Record.FileID).Scan(&storageKey, &fileName, &mediaType, &sizeBytes, &sha256Value, &uploadedBy, &uploadedAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				summary.Issues = append(summary.Issues, fmt.Sprintf("tenant %s missing cubebox_files row for %s", tenantID, item.Record.FileID))
				continue
			}
			return cubeboxFileVerifySummary{}, err
		}

		if storageKey != item.Record.StorageKey || fileName != item.Record.FileName || mediaType != item.Record.MediaType ||
			sizeBytes != item.Record.SizeBytes || sha256Value != item.Record.SHA256 || uploadedBy != item.Record.UploadedBy ||
			!uploadedAt.Equal(item.UploadedAt) {
			summary.Issues = append(summary.Issues, fmt.Sprintf("tenant %s file metadata mismatch for %s", tenantID, item.Record.FileID))
		}

		if strings.TrimSpace(item.Record.ConversationID) == "" {
			if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM iam.cubebox_file_links
WHERE tenant_uuid = $1::uuid AND file_id = $2;
`, tenantID, item.Record.FileID).Scan(&linkCount); err != nil {
				return cubeboxFileVerifySummary{}, err
			}
			if linkCount != 0 {
				summary.Issues = append(summary.Issues, fmt.Sprintf("tenant %s file %s should not have cubebox_file_links", tenantID, item.Record.FileID))
			}
			continue
		}

		if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM iam.cubebox_file_links
WHERE tenant_uuid = $1::uuid
  AND file_id = $2
  AND conversation_id = $3
  AND turn_id IS NULL
  AND link_role = $4;
`, tenantID, item.Record.FileID, item.Record.ConversationID, cubeboxLinkRole).Scan(&linkCount); err != nil {
			return cubeboxFileVerifySummary{}, err
		}
		if linkCount != 1 {
			summary.Issues = append(summary.Issues, fmt.Sprintf("tenant %s file %s expected one conversation_attachment link", tenantID, item.Record.FileID))
		}
	}

	return summary, nil
}

func cubeboxEnsureTenantExists(ctx context.Context, tx pgx.Tx, tenantID string) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM iam.tenants WHERE id = $1::uuid);`, tenantID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("tenant %s not found in iam.tenants", tenantID)
	}
	return nil
}

func cubeboxEnsureConversationExists(ctx context.Context, tx pgx.Tx, tenantID string, conversationID string) error {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil
	}
	var exists bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM iam.cubebox_conversations
  WHERE tenant_uuid = $1::uuid AND conversation_id = $2
);
`, tenantID, conversationID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("tenant %s conversation %s missing in iam.cubebox_conversations", tenantID, conversationID)
	}
	return nil
}

func cubeboxComputeSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func parseRFC3339Flexible(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value)); err == nil {
		return parsed.UTC(), nil
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(value))
}

func writeJSONToStdout(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fatal(err)
	}
}

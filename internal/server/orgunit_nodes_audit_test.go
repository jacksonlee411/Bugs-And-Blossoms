package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type orgUnitStoreWithoutAudit struct {
	OrgUnitStore
}

type orgUnitStoreWithAudit struct {
	OrgUnitStore
	listAuditFn func(ctx context.Context, tenantID string, orgID int, limit int) ([]OrgUnitNodeAuditEvent, error)
}

func (s orgUnitStoreWithAudit) ListNodeAuditEvents(ctx context.Context, tenantID string, orgID int, limit int) ([]OrgUnitNodeAuditEvent, error) {
	if s.listAuditFn != nil {
		return s.listAuditFn(ctx, tenantID, orgID, limit)
	}
	return []OrgUnitNodeAuditEvent{}, nil
}

type orgUnitReadStoreWithAudit struct {
	*orgUnitReadStoreStub
	listAuditFn func(ctx context.Context, tenantID string, orgID int, limit int) ([]OrgUnitNodeAuditEvent, error)
}

func (s *orgUnitReadStoreWithAudit) ListNodeAuditEvents(ctx context.Context, tenantID string, orgID int, limit int) ([]OrgUnitNodeAuditEvent, error) {
	if s.listAuditFn != nil {
		return s.listAuditFn(ctx, tenantID, orgID, limit)
	}
	return []OrgUnitNodeAuditEvent{}, nil
}

type auditRows struct {
	records [][]any
	idx     int
	scanErr error
	err     error
}

func (r *auditRows) Close()                        {}
func (r *auditRows) Err() error                    { return r.err }
func (r *auditRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *auditRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *auditRows) Next() bool {
	if r.idx >= len(r.records) {
		return false
	}
	r.idx++
	return true
}
func (r *auditRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	rec := r.records[r.idx-1]
	for i := range dest {
		if i >= len(rec) || rec[i] == nil {
			continue
		}
		switch d := dest[i].(type) {
		case *int64:
			*d = rec[i].(int64)
		case *string:
			*d = rec[i].(string)
		case *int:
			*d = rec[i].(int)
		case *bool:
			*d = rec[i].(bool)
		case *time.Time:
			*d = rec[i].(time.Time)
		case *[]byte:
			*d = append([]byte(nil), rec[i].([]byte)...)
		default:
			return fmt.Errorf("unsupported scan type %T", d)
		}
	}
	return nil
}
func (r *auditRows) Values() ([]any, error) { return nil, nil }
func (r *auditRows) RawValues() [][]byte    { return nil }
func (r *auditRows) Conn() *pgx.Conn        { return nil }

func TestOrgUnitInitiatorUUID(t *testing.T) {
	t.Run("principal uuid", func(t *testing.T) {
		id := uuid.NewString()
		ctx := withPrincipal(context.Background(), Principal{ID: id})
		if got := orgUnitInitiatorUUID(ctx, " tenant-id "); got != id {
			t.Fatalf("got=%q", got)
		}
	})
	t.Run("invalid principal id falls back to tenant", func(t *testing.T) {
		ctx := withPrincipal(context.Background(), Principal{ID: "not-uuid"})
		if got := orgUnitInitiatorUUID(ctx, " tenant-id "); got != "tenant-id" {
			t.Fatalf("got=%q", got)
		}
	})
	t.Run("missing principal falls back to tenant", func(t *testing.T) {
		if got := orgUnitInitiatorUUID(context.Background(), " tenant-id "); got != "tenant-id" {
			t.Fatalf("got=%q", got)
		}
	})
}

func TestOrgNodeAuditURLHelpers(t *testing.T) {
	t.Run("limit", func(t *testing.T) {
		if got := orgNodeAuditLimitFromURL(httptest.NewRequest(http.MethodGet, "/org/nodes/details", nil)); got != orgNodeAuditPageSize {
			t.Fatalf("got=%d", got)
		}
		if got := orgNodeAuditLimitFromURL(httptest.NewRequest(http.MethodGet, "/org/nodes/details?limit=x", nil)); got != orgNodeAuditPageSize {
			t.Fatalf("got=%d", got)
		}
		if got := orgNodeAuditLimitFromURL(httptest.NewRequest(http.MethodGet, "/org/nodes/details?limit=0", nil)); got != orgNodeAuditPageSize {
			t.Fatalf("got=%d", got)
		}
		if got := orgNodeAuditLimitFromURL(httptest.NewRequest(http.MethodGet, "/org/nodes/details?limit=-2", nil)); got != orgNodeAuditPageSize {
			t.Fatalf("got=%d", got)
		}
		if got := orgNodeAuditLimitFromURL(httptest.NewRequest(http.MethodGet, "/org/nodes/details?limit=5", nil)); got != 5 {
			t.Fatalf("got=%d", got)
		}
	})

	t.Run("active tab", func(t *testing.T) {
		if got := orgNodeActiveTabFromURL(httptest.NewRequest(http.MethodGet, "/org/nodes/details?tab=change", nil)); got != "change" {
			t.Fatalf("got=%q", got)
		}
		if got := orgNodeActiveTabFromURL(httptest.NewRequest(http.MethodGet, "/org/nodes/details?tab=%20CHANGE%20", nil)); got != "change" {
			t.Fatalf("got=%q", got)
		}
		if got := orgNodeActiveTabFromURL(httptest.NewRequest(http.MethodGet, "/org/nodes/details?tab=basic", nil)); got != "basic" {
			t.Fatalf("got=%q", got)
		}
	})
}

func TestListNodeAuditEventsHelper(t *testing.T) {
	ctx := context.Background()
	base := newOrgUnitMemoryStore()
	wrapped := orgUnitStoreWithoutAudit{OrgUnitStore: base}
	events, err := listNodeAuditEvents(ctx, wrapped, "t1", 10000001, 1)
	if err != nil {
		t.Fatalf("unexpected err=%v", err)
	}
	if len(events) != 0 {
		t.Fatalf("events=%#v", events)
	}

	expected := []OrgUnitNodeAuditEvent{{EventID: 1, EventUUID: "evt-1"}}
	withAudit := orgUnitStoreWithAudit{
		OrgUnitStore: base,
		listAuditFn: func(_ context.Context, tenantID string, orgID int, limit int) ([]OrgUnitNodeAuditEvent, error) {
			if tenantID != "t1" || orgID != 10000001 || limit != 3 {
				t.Fatalf("tenant=%q orgID=%d limit=%d", tenantID, orgID, limit)
			}
			return expected, nil
		},
	}
	events, err = listNodeAuditEvents(ctx, withAudit, "t1", 10000001, 3)
	if err != nil {
		t.Fatalf("unexpected err=%v", err)
	}
	if len(events) != 1 || events[0].EventUUID != "evt-1" {
		t.Fatalf("events=%#v", events)
	}

	withAuditErr := orgUnitStoreWithAudit{
		OrgUnitStore: base,
		listAuditFn: func(context.Context, string, int, int) ([]OrgUnitNodeAuditEvent, error) {
			return nil, errors.New("boom")
		},
	}
	if _, err := listNodeAuditEvents(ctx, withAuditErr, "t1", 1, 1); err == nil {
		t.Fatal("expected error")
	}
}

func TestOrgUnitPGStore_ListNodeAuditEvents(t *testing.T) {
	ctx := context.Background()
	t.Run("begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		if _, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 1); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("exec error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		if _, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 1); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("query error", func(t *testing.T) {
		tx := &stubTx{queryErr: errors.New("query")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 1); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan error", func(t *testing.T) {
		tx := &stubTx{rows: &auditRows{records: [][]any{{int64(1)}}, scanErr: errors.New("scan")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 1); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("rows error", func(t *testing.T) {
		tx := &stubTx{rows: &auditRows{records: [][]any{}, err: errors.New("rows")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 1); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("commit error", func(t *testing.T) {
		txTime := time.Date(2026, 1, 6, 10, 0, 0, 0, time.UTC)
		tx := &stubTx{
			rows: &auditRows{records: [][]any{{
				int64(10), "evt-10", 10000001, "RENAME", time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC), txTime,
				"Alice", "E100", "req-1", "", []byte(`{"name":"B"}`), []byte(`{"name":"A"}`), []byte(`{"name":"B"}`), "", false, "", time.Time{}, "",
			}}},
			commitErr: errors.New("commit"),
		}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 0); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("success", func(t *testing.T) {
		txTime := time.Date(2026, 1, 6, 10, 30, 0, 0, time.UTC)
		tx := &stubTx{rows: &auditRows{records: [][]any{
			{
				int64(10), "evt-10", 10000001, "CORRECT_EVENT", time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC), txTime,
				"Alice", "E100", "req-1", "reason", []byte(`{"target_event_uuid":"evt-9"}`), []byte(`{"name":"A"}`), []byte(`{"name":"B"}`), "", false, "", time.Time{}, "",
			},
			{
				int64(9), "evt-9", 10000001, "RENAME", time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), txTime,
				"Bob", "E101", "req-0", "", []byte(`{"name":"A"}`), []byte{}, []byte{}, "", true, "resc-evt-1", time.Date(2026, 1, 6, 12, 0, 0, 0, time.UTC), "resc-req-1",
			},
		}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		events, err := store.ListNodeAuditEvents(ctx, "t1", 10000001, 0)
		if err != nil {
			t.Fatalf("unexpected err=%v", err)
		}
		if len(events) != 2 {
			t.Fatalf("events=%#v", events)
		}
		if events[0].EffectiveDate != "2026-01-06" || string(events[0].BeforeSnapshot) == "" || string(events[0].AfterSnapshot) == "" {
			t.Fatalf("events[0]=%#v", events[0])
		}
		if len(events[1].BeforeSnapshot) != 0 || len(events[1].AfterSnapshot) != 0 {
			t.Fatalf("events[1]=%#v", events[1])
		}
		if !events[1].IsRescinded || events[1].RescindedByEventUUID != "resc-evt-1" || events[1].RescindedByRequestCode != "resc-req-1" {
			t.Fatalf("events[1]=%#v", events[1])
		}
	})
}

func TestOrgUnitMemoryStore_ListNodeAuditEvents(t *testing.T) {
	ctx := context.Background()
	store := newOrgUnitMemoryStore()
	node, err := store.CreateNodeCurrent(ctx, "t1", "2026-01-06", "A001", "Root", "", false)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := strconv.Atoi(node.ID)

	events, err := store.ListNodeAuditEvents(ctx, "t1", id, 1)
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	if events[0].EventType != "RENAME" || events[0].InitiatorName == "" {
		t.Fatalf("event=%#v", events[0])
	}

	events, err = store.ListNodeAuditEvents(ctx, "t1", id, 0)
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%#v err=%v", events, err)
	}

	if _, err := store.ListNodeAuditEvents(ctx, "t1", id+1, 1); !errors.Is(err, errOrgUnitNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestOrgNodeAuditFormattingHelpers(t *testing.T) {
	if got := formatOrgNodeDate(time.Time{}); got != "-" {
		t.Fatalf("got=%q", got)
	}
	if got := formatOrgNodeDate(time.Date(2026, 1, 6, 10, 0, 0, 0, time.FixedZone("UTC+2", 2*3600))); got != "2026-01-06" {
		t.Fatalf("got=%q", got)
	}

	if got := formatOrgNodePathIDs(nil); got != "-" {
		t.Fatalf("got=%q", got)
	}
	if got := formatOrgNodePathIDs([]int{10000001, 10000002}); got != "10000001.10000002" {
		t.Fatalf("got=%q", got)
	}

	if got := formatOrgNodeAuditTime(time.Time{}); got != "-" {
		t.Fatalf("got=%q", got)
	}
	auditTime := time.Date(2026, 1, 6, 2, 5, 0, 0, time.UTC)
	if got := formatOrgNodeAuditTime(auditTime); got != "2026-01-06 10:05" {
		t.Fatalf("got=%q", got)
	}

	if got := formatOrgNodeAuditActor(" ", " "); got != "-(-)" {
		t.Fatalf("got=%q", got)
	}
	if got := formatOrgNodeAuditActor("Alice", "E100"); got != "Alice(E100)" {
		t.Fatalf("got=%q", got)
	}

	labels := map[string]string{
		"CREATE":            "新建组织",
		"MOVE":              "调整上级",
		"RENAME":            "组织更名",
		"DISABLE":           "设为无效",
		"ENABLE":            "设为有效",
		"SET_BUSINESS_UNIT": "业务单元变更",
		"CORRECT_EVENT":     "修正记录",
		"CORRECT_STATUS":    "同日状态修正",
		"RESCIND_EVENT":     "撤销记录",
		"RESCIND_ORG":       "撤销组织",
		"UNKNOWN":           "-",
	}
	for typ, expected := range labels {
		if got := orgNodeEventTypeLabel(typ); got != expected {
			t.Fatalf("typ=%s got=%q", typ, got)
		}
	}
}

func TestOrgNodeAuditMapScalarAndDiff(t *testing.T) {
	if got := orgNodeAuditMap(nil); len(got) != 0 {
		t.Fatalf("got=%#v", got)
	}
	if got := orgNodeAuditMap(json.RawMessage("bad")); len(got) != 0 {
		t.Fatalf("got=%#v", got)
	}
	if got := orgNodeAuditMap(json.RawMessage("null")); len(got) != 0 {
		t.Fatalf("got=%#v", got)
	}
	valid := orgNodeAuditMap(json.RawMessage(`{"name":"A","disabled":false}`))
	if valid["name"] != "A" {
		t.Fatalf("valid=%#v", valid)
	}

	cases := map[string]struct {
		in   any
		want string
	}{
		"nil":        {in: nil, want: "-"},
		"blank":      {in: "  ", want: "-"},
		"string":     {in: "A", want: "A"},
		"bool true":  {in: true, want: "true"},
		"bool false": {in: false, want: "false"},
		"number":     {in: 12, want: "12"},
	}
	for name, tc := range cases {
		if got := orgNodeAuditScalarString(tc.in); got != tc.want {
			t.Fatalf("%s got=%q want=%q", name, got, tc.want)
		}
	}

	rows := orgNodeAuditDiffRows(
		map[string]any{"name": "A", "same": "x", "removed": "1"},
		map[string]any{"name": "B", "same": "x", "added": true},
	)
	if len(rows) != 3 {
		t.Fatalf("rows=%#v", rows)
	}
	joined := fmt.Sprintf("%v", rows)
	for _, token := range []string{"added", "removed", "name"} {
		if !strings.Contains(joined, token) {
			t.Fatalf("rows=%#v", rows)
		}
	}

	if !orgNodeAuditIsRescindEvent("RESCIND_EVENT") || orgNodeAuditIsRescindEvent("RENAME") {
		t.Fatal("unexpected rescind event classifier result")
	}

	if orgNodeAuditHasRequiredSnapshotFields(nil) {
		t.Fatal("expected nil snapshot to be incomplete")
	}
	if orgNodeAuditHasRequiredSnapshotFields(map[string]any{"name": "A"}) {
		t.Fatal("expected partial snapshot to be incomplete")
	}
	complete := map[string]any{
		"org_id":           1,
		"name":             "A",
		"status":           "active",
		"parent_id":        1,
		"node_path":        "1",
		"validity":         "[2026-01-01,)",
		"full_name_path":   "A",
		"is_business_unit": false,
	}
	if !orgNodeAuditHasRequiredSnapshotFields(complete) {
		t.Fatal("expected complete snapshot to pass")
	}
	rows2 := orgNodeAuditSnapshotRows(map[string]any{"b": "2", "a": "1"})
	if len(rows2) != 2 || rows2[0][0] != "a" || rows2[1][0] != "b" {
		t.Fatalf("rows2=%#v", rows2)
	}
}

func TestRenderOrgNodeAuditDetailEntry(t *testing.T) {
	event := OrgUnitNodeAuditEvent{
		EventID:             10,
		EventUUID:           "evt-10",
		EventType:           "CORRECT_EVENT",
		EffectiveDate:       "2026-01-06",
		TxTime:              time.Date(2026, 1, 6, 10, 5, 0, 0, time.UTC),
		InitiatorName:       "Alice",
		InitiatorEmployeeID: "E100",
		RequestCode:         "req-1",
		Reason:              "修复",
		Payload:             json.RawMessage(`{"target_event_uuid":"evt-9","target_effective_date":"2026-01-05","name":"B"}`),
		BeforeSnapshot:      json.RawMessage(`{"name":"A"}`),
		AfterSnapshot:       json.RawMessage(`{"name":"B"}`),
	}
	out := renderOrgNodeAuditDetailEntry(event)
	for _, token := range []string{"CORRECT_EVENT", "修正记录", "跳转到目标事件", "evt-9", "字段变更", "原始数据"} {
		if !strings.Contains(out, token) {
			t.Fatalf("missing %q in %q", token, out)
		}
	}

	outRescinded := renderOrgNodeAuditDetailEntry(OrgUnitNodeAuditEvent{
		EventID:                15,
		EventUUID:              "evt-15",
		EventType:              "RENAME",
		EffectiveDate:          "2026-01-07",
		TxTime:                 time.Date(2026, 1, 7, 1, 0, 0, 0, time.UTC),
		InitiatorName:          "Ann",
		InitiatorEmployeeID:    "E102",
		RescindedByEventUUID:   "evt-r-1",
		RescindedByTxTime:      time.Date(2026, 1, 8, 2, 0, 0, 0, time.UTC),
		RescindedByRequestCode: "req-r-1",
		IsRescinded:            true,
		Payload:                json.RawMessage(`{"new_name":"X"}`),
		BeforeSnapshot:         json.RawMessage(`{"name":"A"}`),
		AfterSnapshot:          json.RawMessage(`{"name":"X"}`),
	})
	for _, token := range []string{"已撤销", "evt-r-1", "撤销请求编号：req-r-1"} {
		if !strings.Contains(outRescinded, token) {
			t.Fatalf("missing %q in %q", token, outRescinded)
		}
	}

	outRescindEvent := renderOrgNodeAuditDetailEntry(OrgUnitNodeAuditEvent{
		EventID:        16,
		EventUUID:      "evt-16",
		EventType:      "RESCIND_EVENT",
		EffectiveDate:  "2026-01-07",
		TxTime:         time.Date(2026, 1, 8, 2, 0, 0, 0, time.UTC),
		RescindOutcome: "PRESENT",
		Payload:        json.RawMessage(`{"target_event_uuid":"evt-9"}`),
		BeforeSnapshot: json.RawMessage(`{"org_id":1,"name":"A","status":"active","parent_id":1,"node_path":"1","validity":"[2026-01-01,)","full_name_path":"A","is_business_unit":false}`),
		AfterSnapshot:  json.RawMessage(`{"org_id":1,"name":"B","status":"active","parent_id":1,"node_path":"1","validity":"[2026-01-01,)","full_name_path":"B","is_business_unit":false}`),
	})
	for _, token := range []string{"撤销前完整快照", "撤销后快照", "org_id", "full_name_path"} {
		if !strings.Contains(outRescindEvent, token) {
			t.Fatalf("missing %q in %q", token, outRescindEvent)
		}
	}

	outRescindBeforeMissing := renderOrgNodeAuditDetailEntry(OrgUnitNodeAuditEvent{
		EventID:        17,
		EventUUID:      "evt-17",
		EventType:      "RESCIND_EVENT",
		EffectiveDate:  "2026-01-08",
		RescindOutcome: "ABSENT",
		Payload:        json.RawMessage(`{"target_event_uuid":"evt-8"}`),
	})
	for _, token := range []string{"撤销前快照缺失", "撤销后已不存在（ABSENT）"} {
		if !strings.Contains(outRescindBeforeMissing, token) {
			t.Fatalf("missing %q in %q", token, outRescindBeforeMissing)
		}
	}

	outRescindAfterMissing := renderOrgNodeAuditDetailEntry(OrgUnitNodeAuditEvent{
		EventID:        18,
		EventUUID:      "evt-18",
		EventType:      "RESCIND_EVENT",
		EffectiveDate:  "2026-01-08",
		RescindOutcome: "PRESENT",
		Payload:        json.RawMessage(`{"target_event_uuid":"evt-8"}`),
		BeforeSnapshot: json.RawMessage(`{"name":"A"}`),
	})
	for _, token := range []string{"撤销前快照字段不完整（历史数据）", "撤销后快照缺失"} {
		if !strings.Contains(outRescindAfterMissing, token) {
			t.Fatalf("missing %q in %q", token, outRescindAfterMissing)
		}
	}

	outRescindAfterPartial := renderOrgNodeAuditDetailEntry(OrgUnitNodeAuditEvent{
		EventID:        19,
		EventUUID:      "evt-19",
		EventType:      "RESCIND_EVENT",
		EffectiveDate:  "2026-01-08",
		RescindOutcome: "PRESENT",
		Payload:        json.RawMessage(`{"target_event_uuid":"evt-8"}`),
		BeforeSnapshot: json.RawMessage(`{"org_id":1,"name":"A","status":"active","parent_id":1,"node_path":"1","validity":"[2026-01-01,)","full_name_path":"A","is_business_unit":false}`),
		AfterSnapshot:  json.RawMessage(`{"name":"B"}`),
	})
	if !strings.Contains(outRescindAfterPartial, "撤销后快照字段不完整（历史数据）") {
		t.Fatalf("unexpected output: %q", outRescindAfterPartial)
	}

	originalMarshal := orgNodeAuditMarshalIndent
	t.Cleanup(func() {
		orgNodeAuditMarshalIndent = originalMarshal
	})
	orgNodeAuditMarshalIndent = func(v any, prefix, indent string) ([]byte, error) {
		return nil, errors.New("marshal failed")
	}
	outMarshalError := renderOrgNodeAuditDetailEntry(OrgUnitNodeAuditEvent{EventID: 20, EventUUID: "evt-20", EventType: "RENAME", EffectiveDate: "2026-01-08"})
	orgNodeAuditMarshalIndent = originalMarshal
	if !strings.Contains(outMarshalError, "<pre>{}</pre>") {
		t.Fatalf("unexpected output: %q", outMarshalError)
	}

	out2 := renderOrgNodeAuditDetailEntry(OrgUnitNodeAuditEvent{EventID: 11,
		EventUUID:           "evt-11",
		EventType:           "UNKNOWN",
		EffectiveDate:       "2026-01-06",
		InitiatorName:       "",
		InitiatorEmployeeID: "",
		Payload:             json.RawMessage(`{"name":"A"}`),
	})
	if !strings.Contains(out2, "字段变更") || !strings.Contains(out2, "请求编号：-") || !strings.Contains(out2, "原因：-") {
		t.Fatalf("unexpected output: %q", out2)
	}

	out3 := renderOrgNodeAuditDetailEntry(OrgUnitNodeAuditEvent{
		EventID:        12,
		EventUUID:      "evt-12",
		EventType:      "RENAME",
		EffectiveDate:  "2026-01-06",
		TxTime:         time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC),
		Payload:        json.RawMessage(`{"name":"A"}`),
		BeforeSnapshot: json.RawMessage(`{"name":"A"}`),
		AfterSnapshot:  json.RawMessage(`{"name":"A"}`),
	})
	if !strings.Contains(out3, "无字段差异") {
		t.Fatalf("unexpected output: %q", out3)
	}

	outNoFallback := renderOrgNodeAuditDetailEntry(OrgUnitNodeAuditEvent{
		EventID:       99,
		EventUUID:     "evt-99",
		EventType:     "RENAME",
		EffectiveDate: "2026-01-06",
		TxTime:        time.Date(2026, 2, 9, 11, 11, 0, 0, time.UTC),
		Payload:       json.RawMessage(`{"new_name":"B"}`),
		// after_snapshot 缺失时不应回退使用 payload 生成 diff。
	})
	if strings.Contains(outNoFallback, "<td>new_name</td>") {
		t.Fatalf("expected no payload fallback diff row, got: %q", outNoFallback)
	}
	if !strings.Contains(outNoFallback, "快照缺失") {
		t.Fatalf("expected missing snapshot warning, got: %q", outNoFallback)
	}

	out4 := renderOrgNodeAuditDetailEntry(OrgUnitNodeAuditEvent{
		EventID:       13,
		EventUUID:     "evt-13",
		EventType:     "RENAME",
		EffectiveDate: "2026-01-06",
		TxTime:        time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC),
	})
	if !strings.Contains(out4, "原始数据") {
		t.Fatalf("unexpected output: %q", out4)
	}

	out5 := renderOrgNodeAuditDetailEntry(OrgUnitNodeAuditEvent{
		EventID:       14,
		EventUUID:     "evt-14",
		EventType:     "RENAME",
		EffectiveDate: "2026-01-06",
		TxTime:        time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC),
		Payload:       json.RawMessage("{"),
	})
	if !strings.Contains(out5, "&#34;payload&#34;: {}") {
		t.Fatalf("unexpected output: %q", out5)
	}
}

func TestRenderOrgNodeDetailsWithAudit_ChangeTabAndLoadMore(t *testing.T) {
	out := renderOrgNodeDetailsWithAudit(
		OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root", Status: "active"},
		"2026-01-06",
		"2026-01-06",
		true,
		[]OrgUnitNodeVersion{{EventID: 1, EffectiveDate: "2026-01-06", EventType: ""}},
		[]OrgUnitNodeAuditEvent{{
			EventID:              1,
			EventUUID:            "evt-1",
			EventType:            "RENAME",
			EffectiveDate:        "2026-01-06",
			TxTime:               time.Date(2026, 1, 6, 10, 0, 0, 0, time.UTC),
			InitiatorName:        "Alice",
			InitiatorEmployeeID:  "E100",
			IsRescinded:          true,
			RescindedByEventUUID: "evt-r-1",
			Payload:              json.RawMessage(`{"name":"B"}`),
			BeforeSnapshot:       json.RawMessage(`{"name":"A"}`),
			AfterSnapshot:        json.RawMessage(`{"name":"B"}`),
		}},
		true,
		1,
		false,
		"",
		"change",
	)
	for _, token := range []string{"data-active-tab=\"change\"", "org-node-change-load-more", "tab=change", "无更新权限", "org-node-record-actions-head", "org-node-record-item", "org-node-change-item-status is-rescinded"} {
		if !strings.Contains(out, token) {
			t.Fatalf("missing %q in %q", token, out)
		}
	}
	if strings.Contains(out, "org-node-basic-toolbar") {
		t.Fatalf("unexpected old toolbar in %q", out)
	}
}

func TestRenderOrgNodeDetailsWithAudit_RecordTimelineDesc(t *testing.T) {
	out := renderOrgNodeDetailsWithAudit(
		OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root", Status: "active"},
		"2026-01-10",
		"2026-01-10",
		true,
		[]OrgUnitNodeVersion{
			{EventID: 1, EffectiveDate: "2026-01-01", EventType: "CREATE"},
			{EventID: 2, EffectiveDate: "2026-01-10", EventType: "LOW"},
			{EventID: 3, EffectiveDate: "2026-01-10", EventType: "HIGH"},
			{EventID: 4, EffectiveDate: "", EventType: "EMPTY"},
		},
		nil,
		false,
		orgNodeAuditPageSize,
		true,
		"",
		"basic",
	)

	idxSameDateHigh := strings.Index(out, `org-node-record-item-type">HIGH</span>`)
	idxSameDateLow := strings.Index(out, `org-node-record-item-type">LOW</span>`)
	idxOldest := strings.Index(out, `data-target-date="2026-01-01"`)
	idxEmpty := strings.Index(out, `data-target-date=""`)
	if idxSameDateHigh == -1 || idxSameDateLow == -1 || idxOldest == -1 || idxEmpty == -1 {
		t.Fatalf("missing record item timeline in output: %q", out)
	}
	if !(idxSameDateHigh < idxSameDateLow && idxSameDateLow < idxOldest && idxOldest < idxEmpty) {
		t.Fatalf("expected record timeline in descending order, got: %q", out)
	}
	if !strings.Contains(out, `class="org-node-record-item is-active" data-target-date="2026-01-10"`) {
		t.Fatalf("expected current effective_date to stay active, got: %q", out)
	}
}

func TestHandleOrgNodeDetails_AuditBranches(t *testing.T) {
	base := &orgUnitReadStoreStub{
		orgUnitMemoryStore: newOrgUnitMemoryStore(),
		detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
			return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
		},
		listVersionsFn: func(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
			return []OrgUnitNodeVersion{{EventID: 1, EffectiveDate: "2026-01-06", EventType: "RENAME"}}, nil
		},
	}

	t.Run("audit error", func(t *testing.T) {
		store := &orgUnitReadStoreWithAudit{
			orgUnitReadStoreStub: base,
			listAuditFn: func(context.Context, string, int, int) ([]OrgUnitNodeAuditEvent, error) {
				return nil, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("audit not found ignored", func(t *testing.T) {
		store := &orgUnitReadStoreWithAudit{
			orgUnitReadStoreStub: base,
			listAuditFn: func(context.Context, string, int, int) ([]OrgUnitNodeAuditEvent, error) {
				return nil, errOrgUnitNotFound
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-06&tree_as_of=2026-01-06&org_id=10000001&tab=change&include_disabled=1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if body := rec.Body.String(); !strings.Contains(body, "include_disabled=1") {
			t.Fatalf("body=%q", body)
		}
	})

	t.Run("audit not found ignored", func(t *testing.T) {
		store := &orgUnitReadStoreWithAudit{
			orgUnitReadStoreStub: base,
			listAuditFn: func(context.Context, string, int, int) ([]OrgUnitNodeAuditEvent, error) {
				return nil, errOrgUnitNotFound
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06&org_id=10000001&tab=change", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "暂无变更日志") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("audit has more", func(t *testing.T) {
		store := &orgUnitReadStoreWithAudit{
			orgUnitReadStoreStub: base,
			listAuditFn: func(_ context.Context, _ string, _ int, limit int) ([]OrgUnitNodeAuditEvent, error) {
				if limit != 2 {
					t.Fatalf("limit=%d", limit)
				}
				return []OrgUnitNodeAuditEvent{
					{EventID: 2, EventUUID: "evt-2", EventType: "CORRECT_EVENT", EffectiveDate: "2026-01-06", TxTime: time.Date(2026, 1, 6, 10, 0, 0, 0, time.UTC), InitiatorName: "A", InitiatorEmployeeID: "E1", Payload: json.RawMessage(`{"target_event_uuid":"evt-1","target_effective_date":"2026-01-05"}`)},
					{EventID: 1, EventUUID: "evt-1", EventType: "RENAME", EffectiveDate: "2026-01-05", TxTime: time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC), InitiatorName: "B", InitiatorEmployeeID: "E2", Payload: json.RawMessage(`{"name":"B"}`)},
				}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/details?effective_date=2026-01-06&tree_as_of=2026-01-06&org_id=10000001&limit=1&tab=change", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "viewer"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetails(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "org-node-change-load-more") || !strings.Contains(body, "tab=change") || !strings.Contains(body, "跳转到目标事件") {
			t.Fatalf("unexpected body: %q", body)
		}
	})
}

func TestHandleOrgNodeDetailsPage_AuditBranches(t *testing.T) {
	base := &orgUnitReadStoreStub{
		orgUnitMemoryStore: newOrgUnitMemoryStore(),
		detailsFn: func(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
			return OrgUnitNodeDetails{OrgID: 10000001, OrgCode: "A001", Name: "Root"}, nil
		},
		listVersionsFn: func(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
			return []OrgUnitNodeVersion{{EventID: 1, EffectiveDate: "2026-01-06", EventType: "RENAME"}}, nil
		},
	}

	t.Run("audit error", func(t *testing.T) {
		store := &orgUnitReadStoreWithAudit{
			orgUnitReadStoreStub: base,
			listAuditFn: func(context.Context, string, int, int) ([]OrgUnitNodeAuditEvent, error) {
				return nil, errors.New("boom")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-06&org_id=10000001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("audit with change tab", func(t *testing.T) {
		store := &orgUnitReadStoreWithAudit{
			orgUnitReadStoreStub: base,
			listAuditFn: func(context.Context, string, int, int) ([]OrgUnitNodeAuditEvent, error) {
				return []OrgUnitNodeAuditEvent{{
					EventID:             1,
					EventUUID:           "evt-1",
					EventType:           "RENAME",
					EffectiveDate:       "2026-01-06",
					TxTime:              time.Date(2026, 1, 6, 10, 0, 0, 0, time.UTC),
					InitiatorName:       "Alice",
					InitiatorEmployeeID: "E100",
					Payload:             json.RawMessage(`{"name":"B"}`),
					BeforeSnapshot:      json.RawMessage(`{"name":"A"}`),
					AfterSnapshot:       json.RawMessage(`{"name":"B"}`),
				}}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-06&tree_as_of=2026-01-06&org_id=10000001&tab=change", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if body := rec.Body.String(); !strings.Contains(body, "OrgUnit / Details") || !strings.Contains(body, "data-active-tab=\"change\"") {
			t.Fatalf("body=%q", body)
		}
	})

	t.Run("audit has more", func(t *testing.T) {
		store := &orgUnitReadStoreWithAudit{
			orgUnitReadStoreStub: base,
			listAuditFn: func(_ context.Context, _ string, _ int, limit int) ([]OrgUnitNodeAuditEvent, error) {
				if limit != 2 {
					t.Fatalf("limit=%d", limit)
				}
				return []OrgUnitNodeAuditEvent{
					{EventID: 2, EventUUID: "evt-2", EventType: "CORRECT_EVENT", EffectiveDate: "2026-01-06", TxTime: time.Date(2026, 1, 6, 10, 0, 0, 0, time.UTC), InitiatorName: "A", InitiatorEmployeeID: "E1", Payload: json.RawMessage(`{"target_event_uuid":"evt-1","target_effective_date":"2026-01-05"}`)},
					{EventID: 1, EventUUID: "evt-1", EventType: "RENAME", EffectiveDate: "2026-01-05", TxTime: time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC), InitiatorName: "B", InitiatorEmployeeID: "E2", Payload: json.RawMessage(`{"name":"B"}`)},
				}, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/org/nodes/view?effective_date=2026-01-06&tree_as_of=2026-01-06&org_id=10000001&limit=1&tab=change", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "Tenant"}))
		req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "viewer"}))
		rec := httptest.NewRecorder()
		handleOrgNodeDetailsPage(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "org-node-change-load-more") || !strings.Contains(body, "tab=change") {
			t.Fatalf("unexpected body: %q", body)
		}
	})
}

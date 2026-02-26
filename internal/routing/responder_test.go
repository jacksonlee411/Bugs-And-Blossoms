package routing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteError_AcceptJSONCharset(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Accept", "application/json; charset=utf-8")
	rec := httptest.NewRecorder()

	WriteError(rec, req, RouteClassUI, http.StatusNotFound, "not_found", "not found")
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type=%q", rec.Header().Get("Content-Type"))
	}
}

func TestTraceIDFromRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		traceparent string
		want        string
	}{
		{name: "empty", traceparent: "", want: ""},
		{name: "malformed segments", traceparent: "00-abc-01", want: ""},
		{name: "invalid chars", traceparent: "00-0123456789abcdef0123456789abcdeg-0123456789abcdef-01", want: ""},
		{name: "all zero trace", traceparent: "00-00000000000000000000000000000000-0123456789abcdef-01", want: ""},
		{name: "valid", traceparent: "00-ABCDEFABCDEFABCDEFABCDEFABCDEFAB-0123456789abcdef-01", want: "abcdefabcdefabcdefabcdefabcdefab"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			if tc.traceparent != "" {
				req.Header.Set("traceparent", tc.traceparent)
			}
			if got := traceIDFromRequest(req); got != tc.want {
				t.Fatalf("traceIDFromRequest()=%q want %q", got, tc.want)
			}
		})
	}
}

func TestWriteError_TraceIDFromTraceparent(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("traceparent", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	WriteError(rec, req, RouteClassInternalAPI, http.StatusBadRequest, "bad", "bad")

	var body ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.TraceID != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("trace_id=%q", body.TraceID)
	}
}

func TestWriteError_RewritesGenericMessageFromCode(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "/org/api/org-units/write", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	WriteError(rec, req, RouteClassInternalAPI, http.StatusConflict, "ORG_ROOT_ALREADY_EXISTS", "orgunit_write_failed")

	var body ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Message == "orgunit_write_failed" {
		t.Fatalf("message should be normalized, got %q", body.Message)
	}
	if body.Message != "根组织已存在，请改为选择上级组织后新建。" {
		t.Fatalf("unexpected message: %q", body.Message)
	}
}

func TestWriteError_HumanizesUnknownGenericCode(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/dicts", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	WriteError(rec, req, RouteClassInternalAPI, http.StatusInternalServerError, "dict_release_failed", "dict_release_failed")

	var body ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Message != "Dict release failed." {
		t.Fatalf("unexpected message: %q", body.Message)
	}
}

func TestWriteError_KeepExplicitMessage(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/setids", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	const want = "default rule evaluation failed. please check the rule."
	WriteError(rec, req, RouteClassInternalAPI, http.StatusBadRequest, "DEFAULT_RULE_EVAL_FAILED", want)

	var body ErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Message != want {
		t.Fatalf("message=%q want %q", body.Message, want)
	}
}

func TestNormalizeErrorMessage_Branches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		code    string
		message string
		want    string
	}{
		{
			name:    "keep explicit message",
			code:    "ORG_CREATE_FAILED",
			message: "create failed for duplicated code",
			want:    "create failed for duplicated code",
		},
		{
			name:    "known code with generic message",
			code:    "ORG_TREE_NOT_INITIALIZED",
			message: "orgunit_write_failed",
			want:    "组织树尚未初始化，请先创建根组织。",
		},
		{
			name:    "empty code with generic message",
			code:    "",
			message: "operation failed",
			want:    "Request failed.",
		},
		{
			name:    "unknown code with generic message",
			code:    "dict_sync_error",
			message: "dict_sync_error",
			want:    "Dict sync error.",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeErrorMessage(tt.code, tt.message); got != tt.want {
				t.Fatalf("normalizeErrorMessage(%q, %q)=%q want %q", tt.code, tt.message, got, tt.want)
			}
		})
	}
}

func TestIsGenericErrorMessage_Patterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		code    string
		message string
		want    bool
	}{
		{name: "empty message", code: "E", message: "", want: true},
		{name: "same as code case insensitive", code: "ORG_CREATE_FAILED", message: "org_create_failed", want: true},
		{name: "snake failed", code: "x", message: "orgunit_write_failed", want: true},
		{name: "short sentence failed", code: "x", message: "create failed", want: true},
		{name: "internal error literal", code: "x", message: "internal_error", want: true},
		{name: "explicit message", code: "x", message: "cannot create org because parent is missing", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isGenericErrorMessage(tt.code, tt.message); got != tt.want {
				t.Fatalf("isGenericErrorMessage(%q, %q)=%v want %v", tt.code, tt.message, got, tt.want)
			}
		})
	}
}

func TestKnownErrorMessage_AllCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code string
		want string
	}{
		{code: "forbidden", want: "无权限执行该操作。"},
		{code: "unauthorized", want: "登录已失效，请重新登录。"},
		{code: "invalid_request", want: "请求参数无效，请检查后重试。"},
		{code: "tenant_not_found", want: "未找到租户，请检查访问域名。"},
		{code: "tenant_missing", want: "租户上下文缺失，请刷新后重试。"},
		{code: "tenant_resolve_error", want: "租户解析失败，请稍后重试。"},
		{code: "CAPABILITY_CONTEXT_MISMATCH", want: "上下文与服务端判定不一致，请检查业务单元与生效日期后重试。"},
		{code: "ORG_ROOT_ALREADY_EXISTS", want: "根组织已存在，请改为选择上级组织后新建。"},
		{code: "ORG_TREE_NOT_INITIALIZED", want: "组织树尚未初始化，请先创建根组织。"},
		{code: "ORG_ALREADY_EXISTS", want: "组织编码已存在，请使用其他编码。"},
		{code: "ORG_NOT_FOUND_AS_OF", want: "在当前查询时点未找到目标组织。"},
		{code: "ORG_CODE_NOT_FOUND", want: "组织编码不存在。"},
		{code: "ORG_CODE_INVALID", want: "组织编码格式无效。"},
		{code: "FIELD_OPTION_NOT_ALLOWED", want: "字段值不在允许范围内，请重新选择。"},
		{code: "FIELD_REQUIRED_VALUE_MISSING", want: "必填字段缺少有效值，请补全后重试。"},
		{code: "FIELD_POLICY_MISSING", want: "未找到匹配的字段策略，请刷新后重试。"},
		{code: "FIELD_POLICY_REDUNDANT_OVERRIDE", want: "该策略与基线配置一致，无需重复覆盖。"},
		{code: "FIELD_POLICY_CONFLICT", want: "字段策略存在冲突，请联系管理员。"},
		{code: "FIELD_POLICY_DISABLE_NOT_ALLOWED", want: "停用该策略会导致当前上下文无可用策略，请先补齐兜底策略后再重试。"},
		{code: "FIELD_POLICY_VERSION_REQUIRED", want: "缺少策略版本，请刷新页面后重试。"},
		{code: "FIELD_POLICY_VERSION_STALE", want: "策略版本已过期，请刷新页面后重试。"},
		{code: "unknown", want: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.code, func(t *testing.T) {
			t.Parallel()
			if got := knownErrorMessage(tt.code); got != tt.want {
				t.Fatalf("knownErrorMessage(%q)=%q want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestHumanizeErrorCode_Branches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code string
		want string
	}{
		{code: "", want: "Request failed."},
		{code: "___", want: "Request failed."},
		{code: "failed", want: "Request failed."},
		{code: "error", want: "Request error."},
		{code: "dict_release_failed", want: "Dict release failed."},
		{code: "tenant_resolve_error", want: "Tenant resolve error."},
		{code: "org_api_id_error", want: "Org API ID error."},
		{code: "foo-bar", want: "Foo bar."},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.code, func(t *testing.T) {
			t.Parallel()
			if got := humanizeErrorCode(tt.code); got != tt.want {
				t.Fatalf("humanizeErrorCode(%q)=%q want %q", tt.code, got, tt.want)
			}
		})
	}
}

func TestTitleCaseWordsAndCapitalizeWord(t *testing.T) {
	t.Parallel()

	if got := titleCaseWords(nil); got != "" {
		t.Fatalf("titleCaseWords(nil)=%q want empty", got)
	}
	if got := titleCaseWords([]string{"org", "api", "db", "uuid", "rls", "id", "code"}); got != "Org API DB UUID RLS ID code" {
		t.Fatalf("unexpected titleCaseWords result: %q", got)
	}
	if got := titleCaseWords([]string{"org", "", "id"}); got != "Org  ID" {
		t.Fatalf("unexpected empty-word handling: %q", got)
	}

	if got := capitalizeWord(""); got != "" {
		t.Fatalf("capitalizeWord(empty)=%q want empty", got)
	}
	if got := capitalizeWord("org"); got != "Org" {
		t.Fatalf("capitalizeWord(org)=%q want Org", got)
	}
}

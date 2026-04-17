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
		{code: "policy_missing", want: "未找到匹配的字段策略，请刷新后重试。"},
		{code: "policy_redundant_override", want: "该策略与基线配置一致，无需重复覆盖。"},
		{code: "policy_conflict_ambiguous", want: "字段策略存在冲突，请联系管理员。"},
		{code: "policy_disable_not_allowed", want: "停用该策略会导致当前上下文无可用策略，请先补齐兜底策略后再重试。"},
		{code: "policy_version_required", want: "缺少策略版本，请刷新页面后重试。"},
		{code: "policy_version_conflict", want: "策略版本已过期，请刷新页面后重试。"},
		{code: "ai_plan_schema_constrained_decode_failed", want: "计划结构化解析失败，请补全必填信息后重试。"},
		{code: "ai_plan_boundary_violation", want: "计划超出助手执行边界，请调整后重试。"},
		{code: "ai_plan_contract_version_mismatch", want: "计划契约版本不一致，请重新生成并确认后再提交。"},
		{code: "ai_version_tuple_stale", want: "确认基线已变化，请重新确认后再提交。"},
		{code: "ai_plan_determinism_violation", want: "计划确定性校验失败，请重新生成后重试。"},
		{code: "ai_model_provider_unavailable", want: "当前无可用模型服务，请检查模型健康状态后重试。"},
		{code: "ai_model_timeout", want: "模型请求超时，请稍后重试。"},
		{code: "ai_model_rate_limited", want: "模型服务限流，请稍后重试。"},
		{code: "ai_model_config_invalid", want: "模型配置不合法，请修正后重新应用。"},
		{code: "ai_runtime_config_invalid", want: "助手运行时模型配置不合法，请修正配置并重启服务。"},
		{code: "ai_runtime_config_missing", want: "助手运行时模型配置缺失，请完成配置并重启服务。"},
		{code: "ai_model_secret_missing", want: "模型密钥缺失，请检查 key_ref 配置后重试。"},
		{code: "assistant_conversation_cursor_invalid", want: "会话分页游标无效或已过期，请刷新列表后重试。"},
		{code: "assistant_conversation_list_failed", want: "加载助手会话列表失败，请稍后重试。"},
		{code: "assistant_runtime_unavailable", want: "助手运行主链暂不可用，请稍后重试。"},
		{code: "assistant_gate_unavailable", want: "助手确认或提交流程暂不可用，请稍后重试。"},
		{code: "assistant_ui_method_not_allowed", want: "当前请求方法不被允许，请刷新页面后重试。"},
		{code: "assistant_ui_path_invalid", want: "助手聊天路径无效，请从助手页面重新进入。"},
		{code: "assistant_session_invalid", want: "助手会话已失效，请重新登录。"},
		{code: "assistant_principal_invalid", want: "当前助手登录主体已失效，请重新登录。"},
		{code: "assistant_ui_bootstrap_unavailable", want: "正式助手入口启动信息暂不可用，请稍后重试。"},
		{code: "assistant_api_gone", want: "旧 Assistant API 已退役，请改用 CubeBox 正式接口。"},
		{code: "assistant_ui_retired", want: "旧助手入口已退役，请改用正式入口。"},
		{code: "assistant_ui_upstream_unavailable", want: "聊天服务暂不可用，请稍后重试。"},
		{code: "assistant_vendored_sid_missing", want: "登录会话缺失，请先从正式登录入口登录。"},
		{code: "assistant_vendored_session_invalid", want: "登录会话已失效，请重新登录。"},
		{code: "assistant_vendored_tenant_mismatch", want: "当前登录会话与租户不匹配，请重新登录。"},
		{code: "assistant_vendored_principal_invalid", want: "登录主体已失效，请重新登录。"},
		{code: "assistant_startup_endpoints_unavailable", want: "正式入口缺少可用 endpoint 配置，请检查 Assistant 运行时模型配置。"},
		{code: "assistant_startup_models_unavailable", want: "正式入口缺少可用模型清单，请检查 Assistant 运行时配置。"},
		{code: "cubebox_service_missing", want: "CubeBox 服务暂不可用，请稍后重试。"},
		{code: "cubebox_conversation_cursor_invalid", want: "CubeBox 会话分页游标无效或已过期，请刷新列表后重试。"},
		{code: "cubebox_conversation_list_failed", want: "加载 CubeBox 会话列表失败，请稍后重试。"},
		{code: "cubebox_conversation_load_failed", want: "加载 CubeBox 会话失败，请稍后重试。"},
		{code: "cubebox_conversation_create_failed", want: "创建 CubeBox 会话失败，请稍后重试。"},
		{code: "cubebox_conversation_delete_blocked_by_running_task", want: "该会话仍有运行中的任务，暂不能删除。"},
		{code: "cubebox_conversation_delete_not_implemented", want: "CubeBox 会话删除能力尚未正式实现。"},
		{code: "cubebox_turn_create_failed", want: "创建 CubeBox 对话轮次失败，请稍后重试。"},
		{code: "cubebox_turn_action_failed", want: "执行 CubeBox 轮次操作失败，请刷新后重试。"},
		{code: "cubebox_task_not_found", want: "未找到 CubeBox 任务，可能已完成清理或无权访问。"},
		{code: "cubebox_task_load_failed", want: "加载 CubeBox 任务失败，请稍后重试。"},
		{code: "cubebox_task_cancel_failed", want: "取消 CubeBox 任务失败，请稍后重试。"},
		{code: "cubebox_task_dispatch_failed", want: "CubeBox 任务调度失败，请稍后重试或联系管理员。"},
		{code: "cubebox_models_unavailable", want: "CubeBox 模型清单暂不可用，请稍后重试。"},
		{code: "cubebox_files_unavailable", want: "CubeBox 文件存储暂不可用，请稍后重试。"},
		{code: "cubebox_files_list_failed", want: "加载 CubeBox 文件列表失败，请稍后重试。"},
		{code: "cubebox_file_delete_failed", want: "CubeBox 文件删除失败，请稍后重试。"},
		{code: "cubebox_file_not_found", want: "未找到 CubeBox 文件。"},
		{code: "librechat_retired", want: "LibreChat 入口已退役，请改用 CubeBox 正式入口。"},
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

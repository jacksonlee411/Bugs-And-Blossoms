package routing

import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

type ErrorEnvelope struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	TraceID string            `json:"trace_id"`
	Meta    ErrorEnvelopeMeta `json:"meta"`
}

type ErrorEnvelopeMeta struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

var (
	genericSnakeFailedPattern = regexp.MustCompile(`^[a-z0-9_]+_failed$`)
	genericShortFailedPattern = regexp.MustCompile(`^[a-z]+(?: [a-z]+){0,2} failed$`)
)

func WriteError(w http.ResponseWriter, r *http.Request, rc RouteClass, status int, code string, message string) {
	message = normalizeErrorMessage(code, message)
	if isJSONOnly(rc) || wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(ErrorEnvelope{
			Code:    code,
			Message: message,
			TraceID: traceIDFromRequest(r),
			Meta: ErrorEnvelopeMeta{
				Path:   r.URL.Path,
				Method: r.Method,
			},
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte("<!doctype html><html><body>"))
	_, _ = w.Write([]byte(message))
	_, _ = w.Write([]byte("</body></html>"))
}

func wantsJSON(r *http.Request) bool {
	return r.Header.Get("Accept") == "application/json" || r.Header.Get("Accept") == "application/json; charset=utf-8"
}

func isJSONOnly(rc RouteClass) bool {
	return rc == RouteClassInternalAPI || rc == RouteClassPublicAPI || rc == RouteClassWebhook
}

func traceIDFromRequest(r *http.Request) string {
	traceparent := strings.TrimSpace(r.Header.Get("traceparent"))
	if traceparent == "" {
		return ""
	}
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		return ""
	}
	traceID := strings.ToLower(parts[1])
	if len(traceID) != 32 || traceID == "00000000000000000000000000000000" {
		return ""
	}
	for _, ch := range traceID {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return ""
		}
	}
	return traceID
}

func normalizeErrorMessage(code string, message string) string {
	code = strings.TrimSpace(code)
	message = strings.TrimSpace(message)
	if !isGenericErrorMessage(code, message) {
		return message
	}
	if known := knownErrorMessage(code); known != "" {
		return known
	}
	if code == "" {
		return "Request failed."
	}
	return humanizeErrorCode(code)
}

func isGenericErrorMessage(code string, message string) bool {
	if message == "" {
		return true
	}
	if code != "" && strings.EqualFold(message, code) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case genericSnakeFailedPattern.MatchString(lower):
		return true
	case genericShortFailedPattern.MatchString(lower):
		return true
	case lower == "internal_error":
		return true
	default:
		return false
	}
}

func knownErrorMessage(code string) string {
	switch strings.TrimSpace(code) {
	case "forbidden":
		return "无权限执行该操作。"
	case "unauthorized":
		return "登录已失效，请重新登录。"
	case "invalid_request":
		return "请求参数无效，请检查后重试。"
	case "tenant_not_found":
		return "未找到租户，请检查访问域名。"
	case "tenant_missing":
		return "租户上下文缺失，请刷新后重试。"
	case "tenant_resolve_error":
		return "租户解析失败，请稍后重试。"
	case "CAPABILITY_CONTEXT_MISMATCH":
		return "上下文与服务端判定不一致，请检查业务单元与生效日期后重试。"
	case "ORG_ROOT_ALREADY_EXISTS":
		return "根组织已存在，请改为选择上级组织后新建。"
	case "ORG_TREE_NOT_INITIALIZED":
		return "组织树尚未初始化，请先创建根组织。"
	case "ORG_ALREADY_EXISTS":
		return "组织编码已存在，请使用其他编码。"
	case "ORG_NOT_FOUND_AS_OF":
		return "在当前查询时点未找到目标组织。"
	case "ORG_CODE_NOT_FOUND":
		return "组织编码不存在。"
	case "ORG_CODE_INVALID":
		return "组织编码格式无效。"
	case "FIELD_OPTION_NOT_ALLOWED":
		return "字段值不在允许范围内，请重新选择。"
	case "FIELD_REQUIRED_VALUE_MISSING":
		return "必填字段缺少有效值，请补全后重试。"
	case "policy_missing":
		return "未找到匹配的字段策略，请刷新后重试。"
	case "policy_redundant_override":
		return "该策略与基线配置一致，无需重复覆盖。"
	case "policy_conflict_ambiguous":
		return "字段策略存在冲突，请联系管理员。"
	case "policy_disable_not_allowed":
		return "停用该策略会导致当前上下文无可用策略，请先补齐兜底策略后再重试。"
	case "policy_version_required":
		return "缺少策略版本，请刷新页面后重试。"
	case "policy_version_conflict":
		return "策略版本已过期，请刷新页面后重试。"
	case "ai_plan_schema_constrained_decode_failed":
		return "计划结构化解析失败，请补全必填信息后重试。"
	case "ai_plan_boundary_violation":
		return "计划超出助手执行边界，请调整后重试。"
	case "ai_plan_contract_version_mismatch":
		return "计划契约版本不一致，请重新生成并确认后再提交。"
	case "ai_version_tuple_stale":
		return "确认基线已变化，请重新确认后再提交。"
	case "ai_plan_determinism_violation":
		return "计划确定性校验失败，请重新生成后重试。"
	case "ai_model_provider_unavailable":
		return "当前无可用模型服务，请检查模型健康状态后重试。"
	case "ai_model_timeout":
		return "模型请求超时，请稍后重试。"
	case "ai_model_rate_limited":
		return "模型服务限流，请稍后重试。"
	case "ai_model_config_invalid":
		return "模型配置不合法，请修正后重新应用。"
	case "ai_runtime_config_invalid":
		return "助手运行时模型配置不合法，请修正配置并重启服务。"
	case "ai_runtime_config_missing":
		return "助手运行时模型配置缺失，请完成配置并重启服务。"
	case "ai_model_secret_missing":
		return "模型密钥缺失，请检查 key_ref 配置后重试。"
	case "ai_reply_model_target_mismatch":
		return "助手回复未命中预期的大模型链路，请稍后重试。"
	case "ai_reply_render_failed":
		return "助手回复生成失败，请稍后重试。"
	case "assistant_conversation_cursor_invalid":
		return "会话分页游标无效或已过期，请刷新列表后重试。"
	case "assistant_conversation_list_failed":
		return "加载助手会话列表失败，请稍后重试。"
	case "assistant_runtime_unavailable":
		return "助手运行主链暂不可用，请稍后重试。"
	case "assistant_gate_unavailable":
		return "助手确认或提交流程暂不可用，请稍后重试。"
	case "assistant_ui_method_not_allowed":
		return "当前请求方法不被允许，请刷新页面后重试。"
	case "assistant_ui_path_invalid":
		return "助手聊天路径无效，请从助手页面重新进入。"
	case "assistant_session_invalid":
		return "助手会话已失效，请重新登录。"
	case "assistant_principal_invalid":
		return "当前助手登录主体已失效，请重新登录。"
	case "assistant_ui_bootstrap_unavailable":
		return "正式助手入口启动信息暂不可用，请稍后重试。"
	case "assistant_api_gone":
		return "旧 Assistant API 已退役，请改用 CubeBox 正式接口。"
	case "assistant_ui_retired":
		return "旧助手入口已退役，请改用正式入口。"
	case "assistant_vendored_api_retired":
		return "旧助手兼容接口已退役，请重新打开正式助手入口。"
	case "assistant_ui_upstream_unavailable":
		return "聊天服务暂不可用，请稍后重试。"
	case "assistant_vendored_sid_missing":
		return "登录会话缺失，请先从正式登录入口登录。"
	case "assistant_vendored_session_invalid":
		return "登录会话已失效，请重新登录。"
	case "assistant_vendored_tenant_mismatch":
		return "当前登录会话与租户不匹配，请重新登录。"
	case "assistant_vendored_principal_invalid":
		return "登录主体已失效，请重新登录。"
	case "assistant_startup_endpoints_unavailable":
		return "正式入口缺少可用 endpoint 配置，请检查 Assistant 运行时模型配置。"
	case "assistant_startup_models_unavailable":
		return "正式入口缺少可用模型清单，请检查 Assistant 运行时配置。"
	case "cubebox_service_missing":
		return "CubeBox 服务暂不可用，请稍后重试。"
	case "cubebox_conversation_cursor_invalid":
		return "CubeBox 会话分页游标无效或已过期，请刷新列表后重试。"
	case "cubebox_conversation_list_failed":
		return "加载 CubeBox 会话列表失败，请稍后重试。"
	case "cubebox_conversation_load_failed":
		return "加载 CubeBox 会话失败，请稍后重试。"
	case "cubebox_conversation_create_failed":
		return "创建 CubeBox 会话失败，请稍后重试。"
	case "cubebox_conversation_delete_blocked_by_running_task":
		return "该会话仍有运行中的任务，暂不能删除。"
	case "cubebox_conversation_delete_not_implemented":
		return "CubeBox 会话删除能力尚未正式实现。"
	case "cubebox_turn_create_failed":
		return "创建 CubeBox 对话轮次失败，请稍后重试。"
	case "cubebox_turn_action_failed":
		return "执行 CubeBox 轮次操作失败，请刷新后重试。"
	case "cubebox_task_not_found":
		return "未找到 CubeBox 任务，可能已完成清理或无权访问。"
	case "cubebox_task_load_failed":
		return "加载 CubeBox 任务失败，请稍后重试。"
	case "cubebox_task_cancel_failed":
		return "取消 CubeBox 任务失败，请稍后重试。"
	case "cubebox_task_dispatch_failed":
		return "CubeBox 任务调度失败，请稍后重试或联系管理员。"
	case "cubebox_models_unavailable":
		return "CubeBox 模型清单暂不可用，请稍后重试。"
	case "cubebox_files_unavailable":
		return "CubeBox 文件存储暂不可用，请稍后重试。"
	case "cubebox_files_list_failed":
		return "加载 CubeBox 文件列表失败，请稍后重试。"
	case "cubebox_file_delete_failed":
		return "CubeBox 文件删除失败，请稍后重试。"
	case "cubebox_file_delete_blocked":
		return "该 CubeBox 文件仍被引用，暂不能删除。"
	case "cubebox_file_not_found":
		return "未找到 CubeBox 文件。"
	case "cubebox_file_upload_invalid":
		return "CubeBox 文件上传参数无效，请检查后重试。"
	case "librechat_retired":
		return "LibreChat 入口已退役，请改用 CubeBox 正式入口。"
	default:
		return ""
	}
}

func humanizeErrorCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "Request failed."
	}

	normalized := strings.ReplaceAll(code, "-", "_")
	parts := strings.Split(normalized, "_")
	words := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		words = append(words, strings.ToLower(part))
	}
	if len(words) == 0 {
		return "Request failed."
	}

	last := words[len(words)-1]
	if last == "failed" {
		words = words[:len(words)-1]
		if len(words) == 0 {
			return "Request failed."
		}
		return titleCaseWords(words) + " failed."
	}

	if last == "error" {
		words = words[:len(words)-1]
		if len(words) == 0 {
			return "Request error."
		}
		return titleCaseWords(words) + " error."
	}

	return titleCaseWords(words) + "."
}

func titleCaseWords(words []string) string {
	if len(words) == 0 {
		return ""
	}
	out := slices.Clone(words)
	for i, word := range out {
		if word == "" {
			continue
		}
		if i == 0 {
			out[i] = capitalizeWord(word)
			continue
		}
		switch word {
		case "id", "uuid", "api", "db", "rls":
			out[i] = strings.ToUpper(word)
		default:
			out[i] = word
		}
	}
	return strings.Join(out, " ")
}

func capitalizeWord(word string) string {
	if word == "" {
		return ""
	}
	runes := []rune(word)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

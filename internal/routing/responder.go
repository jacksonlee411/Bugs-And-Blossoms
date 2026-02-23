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
	case "FIELD_POLICY_MISSING":
		return "未找到匹配的字段策略，请刷新后重试。"
	case "FIELD_POLICY_CONFLICT":
		return "字段策略存在冲突，请联系管理员。"
	case "FIELD_POLICY_DISABLE_NOT_ALLOWED":
		return "停用该策略会导致当前上下文无可用策略，请先补齐兜底策略后再重试。"
	case "FIELD_POLICY_VERSION_REQUIRED":
		return "缺少策略版本，请刷新页面后重试。"
	case "FIELD_POLICY_VERSION_STALE":
		return "策略版本已过期，请刷新页面后重试。"
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

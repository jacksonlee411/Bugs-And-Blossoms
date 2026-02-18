package fieldmeta

import (
	"regexp"
	"strings"
)

// Contract (DEV-PLAN-106A): dict field keys are derived from dict registry codes.
//
// - namespace: d_
// - shape: d_<dict_code>
// - dict_code: ^[a-z][a-z0-9_]{0,63}$ (SSOT: dict registry)
// - additional constraint (Org): len("d_"+dict_code) <= 63 => len(dict_code) <= 61
var customDictFieldKeyRe = regexp.MustCompile(`^d_[a-z][a-z0-9_]{0,60}$`)

// IsCustomDictFieldKey reports whether fieldKey is a supported tenant-enabled DICT(text) field key.
//
// SSOT: docs/dev-plans/106a-org-ext-fields-dict-as-field-key-and-custom-label.md
func IsCustomDictFieldKey(fieldKey string) bool {
	return customDictFieldKeyRe.MatchString(strings.TrimSpace(fieldKey))
}

// DictCodeFromDictFieldKey extracts dict_code from fieldKey if it is a supported dict field key.
func DictCodeFromDictFieldKey(fieldKey string) (string, bool) {
	key := strings.TrimSpace(fieldKey)
	if !IsCustomDictFieldKey(key) {
		return "", false
	}
	return strings.TrimPrefix(key, "d_"), true
}

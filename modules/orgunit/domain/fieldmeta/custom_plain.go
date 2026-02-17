package fieldmeta

import (
	"regexp"
	"strings"
)

var customPlainFieldKeyRe = regexp.MustCompile(`^x_[a-z0-9_]{1,60}$`)

// IsCustomPlainFieldKey reports whether fieldKey is a supported tenant-defined PLAIN(text) field key.
//
// Contract:
// - namespace: x_
// - shape: x_[a-z0-9_]{1,60}
// SSOT: docs/dev-plans/106-org-ext-fields-enable-dict-registry-and-custom-plain-fields.md
func IsCustomPlainFieldKey(fieldKey string) bool {
	return customPlainFieldKeyRe.MatchString(strings.TrimSpace(fieldKey))
}

package types

import "encoding/json"

// TenantFieldConfig is the minimal metadata surface the write-path needs at runtime
// (e.g. to compute enabled ext fields and to derive DICT label snapshots).
// SSOT: docs/dev-plans/100e1-orgunit-mutation-policy-and-ext-corrections-prereq.md
type TenantFieldConfig struct {
	FieldKey         string
	ValueType        string
	DataSourceType   string
	DataSourceConfig json.RawMessage
}

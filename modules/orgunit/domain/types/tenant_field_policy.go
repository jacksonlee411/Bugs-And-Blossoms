package types

// TenantFieldPolicy models runtime write-policy constraints for a single field.
// A policy is resolved by (tenant, field_key, scope_type, scope_key, as_of).
type TenantFieldPolicy struct {
	FieldKey        string
	ScopeType       string
	ScopeKey        string
	Maintainable    bool
	DefaultMode     string
	DefaultRuleExpr *string
	EnabledOn       string
	DisabledOn      *string
}

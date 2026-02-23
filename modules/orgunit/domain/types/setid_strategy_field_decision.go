package types

// SetIDStrategyFieldDecision models a resolved runtime field decision from setid_strategy_registry.
type SetIDStrategyFieldDecision struct {
	CapabilityKey     string
	FieldKey          string
	Required          bool
	Visible           bool
	Maintainable      bool
	DefaultRuleRef    string
	DefaultValue      string
	AllowedValueCodes []string
}

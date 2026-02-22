package server

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"
)

func TestValidateFieldPolicyCELExpr(t *testing.T) {
	if err := validateFieldPolicyCELExpr(`next_org_code("ORG", 1)`); err != nil {
		t.Fatalf("expected valid expression, got %v", err)
	}

	if err := validateFieldPolicyCELExpr(`next_org_code('ORG', 1)`); err == nil || !strings.Contains(err.Error(), "double quotes") {
		t.Fatalf("expected double-quote error, got %v", err)
	}

	if err := validateFieldPolicyCELExpr(`next_org_code("ORG")`); err == nil {
		t.Fatal("expected compile error")
	}

	if err := validateFieldPolicyCELExpr(`"x"+`); err == nil {
		t.Fatal("expected compile error")
	}

	if err := validateFieldPolicyCELExpr(`1 + 1`); err == nil || err.Error() != "expression must return string" {
		t.Fatalf("expected output type error, got %v", err)
	}
}

func TestValidateFieldPolicyCELExpr_EnvError(t *testing.T) {
	origin := newOrgUnitFieldPolicyCELEnv
	t.Cleanup(func() {
		newOrgUnitFieldPolicyCELEnv = origin
	})

	newOrgUnitFieldPolicyCELEnv = func() (*cel.Env, error) {
		return nil, errors.New("env failed")
	}

	if err := validateFieldPolicyCELExpr(`"x"`); err == nil || err.Error() != "env failed" {
		t.Fatalf("expected env error, got %v", err)
	}
}

package superadmin

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

func loadAuthorizer() (*authz.Authorizer, error) {
	modelPath := os.Getenv("AUTHZ_MODEL_PATH")
	if modelPath == "" {
		p, err := defaultAuthzModelPath()
		if err != nil {
			return nil, err
		}
		modelPath = p
	}

	policyPath := os.Getenv("AUTHZ_POLICY_PATH")
	if policyPath == "" {
		p, err := defaultAuthzPolicyPath()
		if err != nil {
			return nil, err
		}
		policyPath = p
	}

	mode, err := authz.ModeFromEnv()
	if err != nil {
		return nil, err
	}

	return authz.NewAuthorizer(modelPath, policyPath, mode)
}

func defaultAuthzModelPath() (string, error) {
	path := "config/access/model.conf"
	for range 8 {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		path = filepath.Join("..", path)
	}
	return "", errors.New("superadmin: authz model not found")
}

func defaultAuthzPolicyPath() (string, error) {
	path := "config/access/policy.csv"
	for range 8 {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		path = filepath.Join("..", path)
	}
	return "", errors.New("superadmin: authz policy not found")
}

package server

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Tenant struct {
	ID     string `yaml:"id"`
	Domain string `yaml:"domain"`
	Name   string `yaml:"name"`
}

type tenantsFile struct {
	Version int      `yaml:"version"`
	Tenants []Tenant `yaml:"tenants"`
}

func loadTenants() (map[string]Tenant, error) {
	path := os.Getenv("TENANTS_PATH")
	if path == "" {
		p, err := defaultTenantsPath()
		if err != nil {
			return nil, err
		}
		path = p
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tf tenantsFile
	if err := yaml.Unmarshal(b, &tf); err != nil {
		return nil, err
	}
	if tf.Version != 1 {
		return nil, errors.New("tenants: unsupported version")
	}
	if len(tf.Tenants) == 0 {
		return nil, errors.New("tenants: empty")
	}

	m := make(map[string]Tenant, len(tf.Tenants))
	for _, t := range tf.Tenants {
		if t.Domain == "" || t.ID == "" {
			return nil, errors.New("tenants: invalid tenant")
		}
		m[t.Domain] = t
	}
	return m, nil
}

func defaultTenantsPath() (string, error) {
	path := "config/tenants.yaml"
	for range 8 {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		path = filepath.Join("..", path)
	}
	return "", errors.New("server: tenants config not found")
}

func hostWithoutPort(host string) string {
	if h, _, ok := strings.Cut(host, ":"); ok {
		return h
	}
	return host
}

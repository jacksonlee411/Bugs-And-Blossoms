package routing

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

type Allowlist struct {
	Version     int                   `yaml:"version"`
	Entrypoints map[string]Entrypoint `yaml:"entrypoints"`
}

type Entrypoint struct {
	Routes []Route `yaml:"routes"`
}

type Route struct {
	Path       string   `yaml:"path"`
	Methods    []string `yaml:"methods"`
	RouteClass string   `yaml:"route_class"`
}

func ParseAllowlistYAML(b []byte) (Allowlist, error) {
	var a Allowlist
	if err := yaml.Unmarshal(b, &a); err != nil {
		return Allowlist{}, err
	}
	if a.Version != 1 {
		return Allowlist{}, errors.New("allowlist: unsupported version")
	}
	if a.Entrypoints == nil {
		return Allowlist{}, errors.New("allowlist: missing entrypoints")
	}
	return a, nil
}

func LoadAllowlist(path string) (Allowlist, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Allowlist{}, err
	}
	return ParseAllowlistYAML(b)
}

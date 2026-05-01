package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/server"
)

func main() {
	root, err := findRepoRoot()
	if err != nil {
		fatal(err)
	}
	policyPath := filepath.Join(root, "config", "access", "policy.csv")
	allowlistPath := filepath.Join(root, "config", "routing", "allowlist.yaml")
	facts, err := server.CollectAuthzCoverageFactsWithAllowlist(policyPath, allowlistPath)
	if err != nil {
		fatal(err)
	}
	if errs := server.LintAuthzCoverage(facts); len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "[authz-lint] %v\n", err)
		}
		os.Exit(1)
	}
	if errs := scanLegacyAuthzLanguage(root); len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "[authz-lint] %v\n", err)
		}
		os.Exit(1)
	}
	fmt.Println("[authz-lint] coverage OK")
}

func scanLegacyAuthzLanguage(root string) []error {
	scanRoots := []string{
		filepath.Join(root, "apps", "web", "src"),
		filepath.Join(root, "config", "access"),
		filepath.Join(root, "internal"),
		filepath.Join(root, "modules"),
		filepath.Join(root, "pkg"),
		filepath.Join(root, "cmd"),
	}
	blocked := []string{
		"VITE_" + "PERMISSIONS",
		"permission" + "Key",
		"foundation" + ".read",
		"approval" + ".read",
		"orgunit" + ".read",
		"orgunit" + ".admin",
		"dict" + ".admin",
		"dict" + ".release.admin",
		"cubebox.conversations" + ".read",
		"cubebox.conversations" + ".use",
		"org." + "share_read",
		"iam." + "ping",
	}
	var errs []error
	for _, scanRoot := range scanRoots {
		if _, err := os.Stat(scanRoot); err != nil {
			continue
		}
		err := filepath.WalkDir(scanRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				base := filepath.Base(path)
				if strings.HasPrefix(base, ".") || base == "node_modules" || base == "assets" {
					return filepath.SkipDir
				}
				return nil
			}
			if !shouldScanLegacyAuthzFile(path) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(data)
			for _, token := range blocked {
				if strings.Contains(content, token) {
					rel, _ := filepath.Rel(root, path)
					errs = append(errs, fmt.Errorf("legacy authz language %q found in %s", token, rel))
				}
			}
			return nil
		})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func shouldScanLegacyAuthzFile(path string) bool {
	switch filepath.Ext(path) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".json", ".csv", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repo root not found from %s", wd)
		}
		dir = parent
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[authz-lint] %v\n", err)
	os.Exit(1)
}

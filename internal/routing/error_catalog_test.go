package routing

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type errorCatalogFile struct {
	Errors []struct {
		Code string `yaml:"code"`
	} `yaml:"errors"`
}

type packageKey struct {
	dir string
	pkg string
}

func TestErrorCatalog_CoversWriteErrorCodes(t *testing.T) {
	root := repoRoot(t)
	catalogCodes := loadCatalogCodes(t, root)
	discoveredCodes := discoverWriteErrorCodes(t, root)

	missing := make([]string, 0)
	for code := range discoveredCodes {
		if !catalogCodes[code] {
			missing = append(missing, code)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("error catalog missing user-visible codes: %v", missing)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repo root with go.mod not found from %s", wd)
		}
		dir = parent
	}
}

func loadCatalogCodes(t *testing.T, root string) map[string]bool {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, "config/errors/catalog.yaml"))
	if err != nil {
		t.Fatalf("read catalog: %v", err)
	}
	var catalog errorCatalogFile
	if err := yaml.Unmarshal(data, &catalog); err != nil {
		t.Fatalf("parse catalog: %v", err)
	}
	if len(catalog.Errors) == 0 {
		t.Fatal("catalog is empty")
	}
	out := make(map[string]bool, len(catalog.Errors))
	for _, item := range catalog.Errors {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			t.Fatal("catalog contains empty code")
		}
		out[code] = true
	}
	return out
}

func discoverWriteErrorCodes(t *testing.T, root string) map[string]bool {
	t.Helper()

	roots := []string{
		filepath.Join(root, "internal"),
		filepath.Join(root, "modules"),
	}
	files := make([]string, 0, 256)
	for _, scanRoot := range roots {
		err := filepath.WalkDir(scanRoot, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				base := filepath.Base(path)
				if strings.HasPrefix(base, ".") || base == "vendor" || base == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_templ.go") {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", scanRoot, err)
		}
	}
	sort.Strings(files)

	fset := token.NewFileSet()
	type fileInfo struct {
		key  packageKey
		file *ast.File
	}
	parsed := make([]fileInfo, 0, len(files))
	consts := map[packageKey]map[string]string{}

	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		key := packageKey{dir: filepath.Dir(path), pkg: f.Name.Name}
		parsed = append(parsed, fileInfo{key: key, file: f})
		if consts[key] == nil {
			consts[key] = map[string]string{}
		}
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					value, err := strconv.Unquote(lit.Value)
					if err != nil {
						continue
					}
					consts[key][name.Name] = strings.TrimSpace(value)
				}
			}
		}
	}

	out := map[string]bool{}
	for _, item := range parsed {
		pkgConsts := consts[item.key]
		ast.Inspect(item.file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			callName := writeErrorCallName(call.Fun)
			if callName == "" {
				return true
			}
			idx := 4
			if callName == "writeError" {
				idx = 3
			}
			if len(call.Args) <= idx {
				return true
			}
			if code := resolveCodeExpr(call.Args[idx], pkgConsts); code != "" {
				out[code] = true
			}
			return true
		})
	}
	return out
}

func writeErrorCallName(fn ast.Expr) string {
	switch x := fn.(type) {
	case *ast.Ident:
		if x.Name == "WriteError" || x.Name == "writeError" {
			return x.Name
		}
	case *ast.SelectorExpr:
		if x.Sel.Name == "WriteError" {
			return "WriteError"
		}
	}
	return ""
}

func resolveCodeExpr(expr ast.Expr, consts map[string]string) string {
	switch x := expr.(type) {
	case *ast.BasicLit:
		if x.Kind != token.STRING {
			return ""
		}
		value, err := strconv.Unquote(x.Value)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(value)
	case *ast.Ident:
		if consts == nil {
			return ""
		}
		return strings.TrimSpace(consts[x.Name])
	default:
		return ""
	}
}

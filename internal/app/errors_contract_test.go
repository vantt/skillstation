package app

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestErrorCodeRegistryMatchesContract keeps the error codes in one place:
// every code the application declares lives in errors.go, appears in the
// registry of docs/contracts/error-codes.md and in the enum of
// schemas/error-envelope.schema.json, and is either returned by the
// application or listed there as reserved.
func TestErrorCodeRegistryMatchesContract(t *testing.T) {
	registry, produced := scanErrorCodes(t)
	documented, reserved := documentedErrorCodes(t)
	schema := schemaErrorCodes(t)

	codes := map[string]bool{}
	for _, code := range registry {
		codes[code] = true
	}
	compareCodeSets(t, "docs/contracts/error-codes.md registry", codes, documented)
	compareCodeSets(t, "schemas/error-envelope.schema.json enum", codes, schema)
	for name, code := range registry {
		switch {
		case produced[name] && reserved[code]:
			t.Errorf("%s is returned by the application but listed under Reserved codes in error-codes.md", code)
		case !produced[name] && !reserved[code]:
			t.Errorf("nothing returns %s: list it under Reserved codes in error-codes.md or remove it", code)
		}
	}
	for code := range reserved {
		if !codes[code] {
			t.Errorf("reserved code %s is not in errors.go", code)
		}
	}
}

// scanErrorCodes parses the application's non-test sources. It returns the
// ErrorCode constants of errors.go by name and the names any code refers to
// outside their declaration. An ErrorCode constant declared elsewhere, or an
// error code written as a bare string, fails the test.
func scanErrorCodes(t *testing.T) (map[string]string, map[string]bool) {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	registry := map[string]string{}
	declared := map[*ast.Ident]bool{}
	var parsed []*ast.File
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, file)
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value := spec.(*ast.ValueSpec)
				if typ, ok := value.Type.(*ast.Ident); !ok || typ.Name != "ErrorCode" {
					continue
				}
				for index, ident := range value.Names {
					if name != "errors.go" {
						t.Errorf("%s declares error code %s; error codes belong in errors.go", name, ident.Name)
						continue
					}
					literal, ok := value.Values[index].(*ast.BasicLit)
					if !ok {
						t.Fatalf("%s is not a string literal", ident.Name)
					}
					code, err := strconv.Unquote(literal.Value)
					if err != nil {
						t.Fatal(err)
					}
					registry[ident.Name] = code
					declared[ident] = true
				}
			}
		}
	}
	produced := map[string]bool{}
	for _, file := range parsed {
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.Ident:
				if _, ok := registry[node.Name]; ok && !declared[node] {
					produced[node.Name] = true
				}
			case *ast.CallExpr:
				if fun, ok := node.Fun.(*ast.Ident); ok && fun.Name == "ErrorCode" && isStringLiteral(node.Args) {
					t.Errorf("%s converts a string to ErrorCode; use a constant from errors.go", fset.Position(node.Pos()))
				}
			case *ast.CompositeLit:
				if typ, ok := node.Type.(*ast.Ident); ok && typ.Name == "Error" {
					for _, element := range node.Elts {
						pair, ok := element.(*ast.KeyValueExpr)
						if key, isIdent := pair.Key.(*ast.Ident); ok && isIdent && key.Name == "Code" && isStringLiteral([]ast.Expr{pair.Value}) {
							t.Errorf("%s writes an error code as a string; use a constant from errors.go", fset.Position(pair.Pos()))
						}
					}
				}
			}
			return true
		})
	}
	return registry, produced
}

func isStringLiteral(args []ast.Expr) bool {
	if len(args) != 1 {
		return false
	}
	literal, ok := args[0].(*ast.BasicLit)
	return ok && literal.Kind == token.STRING
}

// documentedErrorCodes reads the registry table of error-codes.md and its
// Reserved codes list.
func documentedErrorCodes(t *testing.T) (map[string]bool, map[string]bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "contracts", "error-codes.md"))
	if err != nil {
		t.Fatal(err)
	}
	row := regexp.MustCompile("^\\| `([a-z_]+)` \\|")
	item := regexp.MustCompile("^- `([a-z_]+)`$")
	documented, reserved := map[string]bool{}, map[string]bool{}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "#") {
			section = strings.TrimSpace(strings.TrimLeft(line, "#"))
			continue
		}
		switch section {
		case "Error code registry":
			if match := row.FindStringSubmatch(line); match != nil {
				documented[match[1]] = true
			}
		case "Reserved codes":
			if match := item.FindStringSubmatch(line); match != nil {
				reserved[match[1]] = true
			}
		}
	}
	if len(documented) == 0 || len(reserved) == 0 {
		t.Fatalf("error-codes.md: found %d registry rows and %d reserved codes", len(documented), len(reserved))
	}
	return documented, reserved
}

func schemaErrorCodes(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "schemas", "error-envelope.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			Code struct {
				Enum []string `json:"enum"`
			} `json:"code"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, code := range schema.Properties.Code.Enum {
		codes[code] = true
	}
	return codes
}

func compareCodeSets(t *testing.T, where string, registry, other map[string]bool) {
	t.Helper()
	var missing, extra []string
	for code := range registry {
		if !other[code] {
			missing = append(missing, code)
		}
	}
	for code := range other {
		if !registry[code] {
			extra = append(extra, code)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("%s lacks %v from errors.go", where, missing)
	}
	if len(extra) > 0 {
		t.Errorf("%s has %v that errors.go does not declare", where, extra)
	}
}

package web

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestWebAPIDocumentListsTheRegisteredRoutes keeps the endpoint table of the
// web API contract equal to the routes the server registers: a route added,
// removed or renamed on one side only fails here.
func TestWebAPIDocumentListsTheRegisteredRoutes(t *testing.T) {
	t.Parallel()
	documented := documentedWebRoutes(t)
	registered := registeredWebRoutes(t)
	if !slices.Equal(documented, registered) {
		t.Fatalf("docs/contracts/web-api.md section 3 and the registered routes differ\nonly documented: %v\nonly registered: %v",
			missingFrom(documented, registered), missingFrom(registered, documented))
	}

	// The patterns read from the source are the ones the server serves.
	mux := http.NewServeMux()
	srv := &Server{}
	for _, reg := range routeRegistrars {
		reg(srv, mux)
	}
	for _, pattern := range documented {
		method, path, found := strings.Cut(pattern, " ")
		if !found {
			method, path = http.MethodGet, pattern
		}
		request := httptest.NewRequest(method, regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(path, "x"), nil)
		if _, served := mux.Handler(request); served != pattern {
			t.Errorf("documented route %q is served by pattern %q", pattern, served)
		}
	}
}

func documentedWebRoutes(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "contracts", "web-api.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "## 3. Endpoints")
	end := strings.Index(text, "## 4. ")
	if start < 0 || end < start {
		t.Fatal("web-api.md has no section 3 followed by section 4")
	}
	row := regexp.MustCompile("(?m)^\\| `([^`]+)` \\|")
	var routes []string
	for _, match := range row.FindAllStringSubmatch(text[start:end], -1) {
		routes = append(routes, match[1])
	}
	slices.Sort(routes)
	return routes
}

// registeredWebRoutes reads every mux.HandleFunc pattern of the package's
// non-test files. A pattern is a string literal, a string constant of the
// package, or a concatenation of both.
func registeredWebRoutes(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var parsed []*ast.File
	constants := map[string]string{}
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
				for index, ident := range value.Names {
					if index < len(value.Values) {
						if literal, ok := value.Values[index].(*ast.BasicLit); ok && literal.Kind == token.STRING {
							constants[ident.Name], _ = strconv.Unquote(literal.Value)
						}
					}
				}
			}
		}
	}
	var routes []string
	for _, file := range parsed {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "HandleFunc" {
				return true
			}
			pattern, ok := stringValue(call.Args[0], constants)
			if !ok {
				t.Errorf("%s: route pattern is not a string this test can read", fset.Position(call.Pos()))
				return true
			}
			routes = append(routes, pattern)
			return true
		})
	}
	slices.Sort(routes)
	return routes
}

func stringValue(expr ast.Expr, constants map[string]string) (string, bool) {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		if typed.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(typed.Value)
		return value, err == nil
	case *ast.Ident:
		value, ok := constants[typed.Name]
		return value, ok
	case *ast.BinaryExpr:
		if typed.Op != token.ADD {
			return "", false
		}
		left, ok := stringValue(typed.X, constants)
		if !ok {
			return "", false
		}
		right, ok := stringValue(typed.Y, constants)
		return left + right, ok
	}
	return "", false
}

func missingFrom(have, other []string) []string {
	var missing []string
	for _, item := range have {
		if !slices.Contains(other, item) {
			missing = append(missing, item)
		}
	}
	return missing
}

package router_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// TestGeneratedErrorHandlerOpenAPIContractDrift ensures every generated wrapper
// that calls ErrorHandler has a matching OpenAPI operation with:
//   - responses['400'] -> BadRequest / ErrorResponse
//   - x-error-codes['400'] containing VALIDATION_ERROR
//
// Approach: AST-scan api.gen.go for ServerInterfaceWrapper methods that invoke
// ErrorHandler; parse api/openapi.yaml for operation response/x-error-codes.
// Avoids brittle line numbers.
func TestGeneratedErrorHandlerOpenAPIContractDrift(t *testing.T) {
	root := repoRoot(t)
	ehOps := generatedErrorHandlerOperations(t, filepath.Join(root, "internal/transport/http/generated/api.gen.go"))
	if len(ehOps) == 0 {
		t.Fatal("no ErrorHandler-using operations found in generated wrappers")
	}
	if _, ok := ehOps["GetHealth"]; ok {
		t.Fatal("GetHealth unexpectedly uses ErrorHandler")
	}

	specPath := filepath.Join(root, "api/openapi.yaml")
	spec, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(spec), "\r\n", "\n")

	var missing400, missingCode, badRef []string
	for _, oid := range sortedKeys(ehOps) {
		block := operationBlock(t, text, oid)
		if !regexp.MustCompile(`(?m)^        '400':\s*$`).MatchString(block) {
			missing400 = append(missing400, oid)
			continue
		}
		// 400 must reference BadRequest (ErrorResponse component chain)
		if !strings.Contains(block, `"$ref": "#/components/responses/BadRequest"`) &&
			!strings.Contains(block, `'$ref': '#/components/responses/BadRequest'`) {
			lines := strings.Split(block, "\n")
			in400 := false
			var sub400 strings.Builder
			for _, line := range lines {
				if strings.HasPrefix(line, "        '400':") {
					in400 = true
					continue
				}
				if in400 {
					if strings.HasPrefix(line, "        '") || (len(line) > 0 && !strings.HasPrefix(line, "          ")) {
						break
					}
					sub400.WriteString(line)
					sub400.WriteByte('\n')
				}
			}
			content := sub400.String()
			if !strings.Contains(content, "BadRequest") && !strings.Contains(content, "ErrorResponse") {
				badRef = append(badRef, oid)
			}
		}
		if !hasValidationErrorUnder400(block) {
			missingCode = append(missingCode, oid)
		}
	}

	if len(missing400) > 0 {
		t.Fatalf("operations missing OpenAPI 400 response: %v", missing400)
	}
	if len(missingCode) > 0 {
		t.Fatalf("operations missing x-error-codes 400 VALIDATION_ERROR: %v", missingCode)
	}
	if len(badRef) > 0 {
		t.Fatalf("operations with 400 not using BadRequest/ErrorResponse: %v", badRef)
	}

	// Health must remain outside forced 400 parse surface.
	health := operationBlock(t, text, "GetHealth")
	if regexp.MustCompile(`(?m)^        '400':\s*$`).MatchString(health) {
		t.Fatal("GetHealth should not declare generated-parse 400")
	}
}

func generatedErrorHandlerOperations(t *testing.T, genPath string) map[string]struct{} {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, genPath, nil, 0)
	if err != nil {
		t.Fatalf("parse generated file: %v", err)
	}
	out := map[string]struct{}{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name == nil || fn.Body == nil {
			continue
		}
		if !isServerInterfaceWrapperRecv(fn.Recv) {
			continue
		}
		uses := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != "ErrorHandler" {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "siw" {
				uses = true
			}
			return true
		})
		if uses {
			out[fn.Name.Name] = struct{}{}
		}
	}
	return out
}

func isServerInterfaceWrapperRecv(recv *ast.FieldList) bool {
	if recv == nil || len(recv.List) != 1 {
		return false
	}
	switch t := recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name == "ServerInterfaceWrapper"
		}
	case *ast.Ident:
		return t.Name == "ServerInterfaceWrapper"
	}
	return false
}

func operationBlock(t *testing.T, openapi, operationID string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^      operationId: ` + regexp.QuoteMeta(operationID) + `\s*$`)
	loc := re.FindStringIndex(openapi)
	if loc == nil {
		t.Fatalf("operationId %s not found in OpenAPI", operationID)
	}
	prefix := openapi[:loc[0]]
	methodStarts := regexp.MustCompile(`(?m)^    (?:get|post|put|patch|delete):\s*$`).FindAllStringIndex(prefix, -1)
	if len(methodStarts) == 0 {
		t.Fatalf("method start not found for %s", operationID)
	}
	start := methodStarts[len(methodStarts)-1][0]
	rest := openapi[loc[0]:]
	end := len(openapi)
	for _, pat := range []string{
		`(?m)\n    (?:get|post|put|patch|delete):\s*$`,
		`(?m)\n  "/[^"]+":\s*$`,
		`(?m)\ncomponents:\s*$`,
	} {
		if m := regexp.MustCompile(pat).FindStringIndex(rest); m != nil {
			cand := loc[0] + m[0]
			if cand < end {
				end = cand
			}
		}
	}
	return openapi[start:end]
}

func hasValidationErrorUnder400(block string) bool {
	m := regexp.MustCompile(`(?ms)^      x-error-codes:\n((?:        .*\n)+)`).FindStringSubmatch(block)
	if m == nil {
		return false
	}
	cur := ""
	for _, line := range strings.Split(m[1], "\n") {
		if sm := regexp.MustCompile(`^        '(\d+)':\s*$`).FindStringSubmatch(line); sm != nil {
			cur = sm[1]
			continue
		}
		if cur == "400" && strings.TrimSpace(line) == "- VALIDATION_ERROR" {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

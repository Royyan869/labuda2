package http

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestContentDTOsHaveNoTopLevelTypeField(t *testing.T) {
	src, err := os.ReadFile("content_handler.go")
	if err != nil {
		t.Fatalf("read content_handler.go: %v", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "content_handler.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse content_handler.go: %v", err)
	}

	assertStructHasNoTopLevelType := func(name string) {
		t.Helper()

		st := findStructType(t, file, name)
		if st == nil {
			t.Fatalf("struct %s not found", name)
		}

		for _, field := range st.Fields.List {
			for _, ident := range field.Names {
				if ident.Name == "Type" {
					t.Fatalf("%s still has a top-level Type field", name)
				}
			}

			if field.Tag != nil && strings.Contains(field.Tag.Value, `json:"type"`) {
				t.Fatalf("%s still exposes a top-level json type field", name)
			}
		}
	}

	assertStructHasNoTopLevelType("CreateContentRequest")
	assertStructHasNoTopLevelType("ContentResponse")
	assertStructHasNoTopLevelType("UpdateContentRequest")
}

func findStructType(t *testing.T, file *ast.File, name string) *ast.StructType {
	t.Helper()

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != name {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				t.Fatalf("%s is not a struct type", name)
			}
			return st
		}
	}

	return nil
}

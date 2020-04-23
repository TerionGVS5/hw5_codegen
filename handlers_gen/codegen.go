package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strings"
)

func main() {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}
	out, _ := os.Create(os.Args[2])
	defer out.Close()

	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out) // empty line
	fmt.Fprintln(out, `import _ "encoding/json"`)
	fmt.Fprintln(out, `import _ "net/http"`)
	fmt.Fprintln(out, `import _ "reflect"`)
	fmt.Fprintln(out, `import _ "strconv"`)
	fmt.Fprintln(out) // empty line

	for _, decl := range node.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Doc != nil {
			for _, docString := range funcDecl.Doc.List {
				if !strings.HasPrefix(docString.Text, "// apigen:api ") {
					continue
				}
				for _, funcParam := range funcDecl.Type.Params.List {
					if funcParam.Names[0].Name != "in" {
						continue
					}
					for _, structField := range funcParam.Type.(*ast.Ident).Obj.Decl.(*ast.TypeSpec).Type.(*ast.StructType).Fields.List {
						fmt.Println(structField)
					}
				}
			}
		}
	}
}

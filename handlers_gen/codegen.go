package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strings"
)

type ApiGen struct {
	Url    string `json:"url"`
	Auth   bool   `json:"auth"`
	Method string `json:"method"`
}

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
	fmt.Fprintln(out, `import "encoding/json"`)
	fmt.Fprintln(out, `import _ "net/http"`)
	fmt.Fprintln(out, `import _ "reflect"`)
	fmt.Fprintln(out, `import _ "strconv"`)
	fmt.Fprintln(out) // empty line

	// fill common error responses
	fmt.Fprintln(out, `var unknownMethodResponse, _ = json.Marshal(map[string]string{
	"error": "unknown method",
	})`)
	fmt.Fprintln(out) // empty line
	addedBadMethodResponse := false
	addedUnauthorizedResponse := false

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
				var currApiGen ApiGen
				// add common error response if need
				json.Unmarshal([]byte(strings.Replace(docString.Text, "// apigen:api ", "", 1)), &currApiGen)
				if !addedBadMethodResponse && currApiGen.Method != "" {
					addedBadMethodResponse = true
					fmt.Fprintln(out, `var badMethodResponse, _ = json.Marshal(map[string]string{
						"error": "bad method",
					})`)
					fmt.Fprintln(out) // empty line
				}
				if !addedUnauthorizedResponse && currApiGen.Auth == true {
					addedUnauthorizedResponse = true
					fmt.Fprintln(out, `var unauthorizedResponse, _ = json.Marshal(map[string]string{
						"error": "unauthorized",
					})`)
					fmt.Fprintln(out) // empty line
				}
				// here fill handler
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

package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"regexp"
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

	// re here
	reParamName := regexp.MustCompile(`paramname=(?P<paramname>\w+)`)
	reParamDefault := regexp.MustCompile(`default=(?P<default>\w+)`)

	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out) // empty line
	fmt.Fprintln(out, `import "encoding/json"`)
	fmt.Fprintln(out, `import "net/http"`)
	fmt.Fprintln(out, `import _ "reflect"`)
	fmt.Fprintln(out, `import "strconv"`)
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
				apiName := funcDecl.Recv.List[0].Type.(*ast.StarExpr).X
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
				if !addedUnauthorizedResponse && currApiGen.Auth {
					addedUnauthorizedResponse = true
					fmt.Fprintln(out, `var unauthorizedResponse, _ = json.Marshal(map[string]string{
						"error": "unauthorized",
					})`)
					fmt.Fprintln(out) // empty line
				}
				// here fill handler
				// fill first line
				fmt.Fprintln(out, fmt.Sprintf(`func (srv *%[1]s) handler%[2]s(w http.ResponseWriter, r *http.Request) {`,
					apiName, funcDecl.Name))
				if currApiGen.Auth {
					fmt.Fprintln(out, `if r.Header.Get("X-Auth") != "100500" {
						w.WriteHeader(http.StatusForbidden)
						w.Write(unauthorizedResponse)
						return
					}`)
					fmt.Fprintln(out) // empty line
				}
				if currApiGen.Method != "" {
					fmt.Fprintln(out, fmt.Sprintf(`if r.Method != "%s" {
						w.WriteHeader(http.StatusNotAcceptable)
						w.Write(badMethodResponse)
						return
					}`, currApiGen.Method))
					fmt.Fprintln(out) // empty line
				}
				for _, funcParam := range funcDecl.Type.Params.List {
					if funcParam.Names[0].Name != "in" {
						continue
					}
					for _, structField := range funcParam.Type.(*ast.Ident).Obj.Decl.(*ast.TypeSpec).Type.(*ast.StructType).Fields.List {
						//fmt.Println(structField.Tag.Value)
						var fieldName string
						paramNames := reParamName.FindStringSubmatch(structField.Tag.Value)
						if len(paramNames) > 0 {
							fieldName = paramNames[1]
						} else {
							fieldName = strings.ToLower(structField.Names[0].Name)
						}
						if strings.Contains(structField.Tag.Value, "required") {
							fmt.Fprintln(out, fmt.Sprintf(`if r.FormValue("%[1]s") == "" {
								w.WriteHeader(http.StatusBadRequest)
								w.Write(%[1]sEmpty)
								return
							}`, fieldName))
							// todo внести данные в словарь для формирования респонсов
						}
						if structField.Type.(*ast.Ident).Name == "string" {
							fmt.Fprintln(out, fmt.Sprintf(`%[1]sParam := r.FormValue("%[1]s")`, fieldName))
						} else {
							fmt.Fprintln(out, fmt.Sprintf(`%[1]sParam, %[1]sParamErr := strconv.ParseInt(r.FormValue("%[1]s"), 10, 64)`, fieldName))
						}
						paramDefaults := reParamDefault.FindStringSubmatch(structField.Tag.Value)
						if len(paramDefaults) > 0 {
							if structField.Type.(*ast.Ident).Name == "string" {
								fmt.Fprintln(out, fmt.Sprintf(`if %[1]sParam == "" {
									%[1]sParam = "%[2]s"
								}`, fieldName, paramDefaults[1]))
							} else {
								fmt.Fprintln(out, fmt.Sprintf(`if %[1]sParam == "" {
									%[1]sParam = %[2]s
								}`, fieldName, paramDefaults[1]))
							}
						}
					}
				}
			}
		}
	}
}

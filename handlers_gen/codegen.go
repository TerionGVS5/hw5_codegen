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

type CaseHTTPInfo struct {
	Url     string
	Handler string
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
	reParamMin := regexp.MustCompile(`min=(?P<min>\w+)`)
	reParamMax := regexp.MustCompile(`max=(?P<max>\w+)`)
	reEnum := regexp.MustCompile(`enum=(?P<enum>[\w|]+)`)

	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out) // empty line
	fmt.Fprintln(out, `import "encoding/json"`)
	fmt.Fprintln(out, `import "net/http"`)
	fmt.Fprintln(out, `import "reflect"`)
	fmt.Fprintln(out, `import "strconv"`)
	fmt.Fprintln(out) // empty line

	// fill common error responses
	fmt.Fprintln(out, `var unknownMethodResponse, _ = json.Marshal(map[string]string{
	"error": "unknown method",
	})`)
	fmt.Fprintln(out) // empty line
	fmt.Fprintln(out, `func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
	}`)
	fmt.Fprintln(out, `type SR map[string]interface{}`)
	fmt.Fprintln(out) // empty line
	addedBadMethodResponse := false
	addedUnauthorizedResponse := false

	var responseNamesRelatedError = make(map[string]string)
	var serveHTTPObjects = make(map[string][]CaseHTTPInfo)

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
				serveHTTPObjects[fmt.Sprintf(`%s`, apiName)] = append(serveHTTPObjects[fmt.Sprintf(`%s`, apiName)], CaseHTTPInfo{
					Url:     currApiGen.Url,
					Handler: fmt.Sprintf(`handler%s`, funcDecl.Name),
				})
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
					var paramNamesRelatedRequestNames = make(map[string]string)
					for _, structField := range funcParam.Type.(*ast.Ident).Obj.Decl.(*ast.TypeSpec).Type.(*ast.StructType).Fields.List {
						var fieldName string
						paramNames := reParamName.FindStringSubmatch(structField.Tag.Value)
						if len(paramNames) > 0 {
							fieldName = paramNames[1]
						} else {
							fieldName = strings.ToLower(structField.Names[0].Name)
						}
						paramNamesRelatedRequestNames[structField.Names[0].Name] = fmt.Sprintf(`%sParam`, fieldName)
						if strings.Contains(structField.Tag.Value, "required") {
							fmt.Fprintln(out, fmt.Sprintf(`if r.FormValue("%[1]s") == "" {
								w.WriteHeader(http.StatusBadRequest)
								w.Write(%[1]sEmptyResponse%[2]s)
								return
							}`, fieldName, apiName))
							responseNamesRelatedError[fmt.Sprintf(`%[1]sEmptyResponse%[2]s`, fieldName, apiName)] = fmt.Sprintf(`"error": "%s must me not empty",`, fieldName)
						}
						if structField.Type.(*ast.Ident).Name == "string" {
							fmt.Fprintln(out, fmt.Sprintf(`%[1]sParam := r.FormValue("%[1]s")`, fieldName))
						} else {
							fmt.Fprintln(out, fmt.Sprintf(`%[1]sParam64, %[1]sParamErr := strconv.ParseInt(r.FormValue("%[1]s"), 10, 64)`, fieldName))
							fmt.Fprintln(out, fmt.Sprintf(`if %[1]sParamErr != nil {
								w.WriteHeader(http.StatusBadRequest)
								w.Write(int%[1]sResponse%[2]s)
								return
							}`, fieldName, apiName))
							fmt.Fprintln(out, fmt.Sprintf(`%[1]sParam := int(%[1]sParam64)`, fieldName))
							responseNamesRelatedError[fmt.Sprintf(`int%[1]sResponse%[2]s`, fieldName, apiName)] = fmt.Sprintf(`"error": "%s must be int",`, fieldName)
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
						paramMins := reParamMin.FindStringSubmatch(structField.Tag.Value)
						if len(paramMins) > 0 {
							if structField.Type.(*ast.Ident).Name == "string" {
								fmt.Fprintln(out, fmt.Sprintf(`if len([]rune(%[1]sParam)) < %[2]s {
									w.WriteHeader(http.StatusBadRequest)
									w.Write(min%[1]sResponse%[3]s)
									return
								}`, fieldName, paramMins[1], apiName))
								responseNamesRelatedError[fmt.Sprintf(`min%[1]sResponse%[2]s`, fieldName, apiName)] = fmt.Sprintf(`"error": "%[1]s len must be >= %[2]s",`, fieldName, paramMins[1])
							} else {
								fmt.Fprintln(out, fmt.Sprintf(`if %[1]sParam < %[2]s {
									w.WriteHeader(http.StatusBadRequest)
									w.Write(min%[1]sResponse%[3]s)
									return
								}`, fieldName, paramMins[1], apiName))
								responseNamesRelatedError[fmt.Sprintf(`min%[1]sResponse%[2]s`, fieldName, apiName)] = fmt.Sprintf(`"error": "%[1]s must be >= %[2]s",`, fieldName, paramMins[1])
							}
						}
						paramMaxs := reParamMax.FindStringSubmatch(structField.Tag.Value)
						if len(paramMaxs) > 0 {
							if structField.Type.(*ast.Ident).Name == "string" {
								fmt.Fprintln(out, fmt.Sprintf(`if len([]rune(%[1]sParam)) > %[2]s {
									w.WriteHeader(http.StatusBadRequest)
									w.Write(max%[1]sResponse%[3]s)
									return
								}`, fieldName, paramMaxs[1], apiName))
								responseNamesRelatedError[fmt.Sprintf(`max%[1]sResponse%[2]s`, fieldName, apiName)] = fmt.Sprintf(`"error": "%[1]s len must be <= %[2]s",`, fieldName, paramMaxs[1])
							} else {
								fmt.Fprintln(out, fmt.Sprintf(`if %[1]sParam > %[2]s {
									w.WriteHeader(http.StatusBadRequest)
									w.Write(max%[1]sResponse%[3]s)
									return
								}`, fieldName, paramMaxs[1], apiName))
								responseNamesRelatedError[fmt.Sprintf(`max%[1]sResponse%[2]s`, fieldName, apiName)] = fmt.Sprintf(`"error": "%[1]s must be <= %[2]s",`, fieldName, paramMaxs[1])
							}
						}
						paramEnums := reEnum.FindStringSubmatch(structField.Tag.Value)
						if len(paramEnums) > 0 {
							fmt.Fprintln(out, fmt.Sprintf(`if !contains(%#[1]v, %[2]sParam) {
									w.WriteHeader(http.StatusBadRequest)
									w.Write(%[2]sStatusResponse%[3]s)
									return
							}`, strings.Split(paramEnums[1], "|"), fieldName, apiName))
							semiformat := fmt.Sprintf("%v", strings.Split(paramEnums[1], "|"))
							tokens := strings.Split(semiformat, " ")
							responseNamesRelatedError[fmt.Sprintf(`%[1]sStatusResponse%[2]s`, fieldName, apiName)] = fmt.Sprintf(`"error": "%[1]s must be one of %[2]s",`, fieldName, strings.Join(tokens, ", "))
						}
					}
					fmt.Fprintln(out, `ctx := r.Context()`)
					fmt.Fprintln(out, fmt.Sprintf(`params := %s{`, funcParam.Type.(*ast.Ident).Name))
					for key, element := range paramNamesRelatedRequestNames {
						fmt.Fprintln(out, fmt.Sprintf(`%[1]s: %[2]s,`, key, element))
					}
					fmt.Fprintln(out, `}`)
					fmt.Fprintln(out, fmt.Sprintf(`newObj, err := srv.%s(ctx, params)
					if err != nil {
						if reflect.TypeOf(err).String() != "main.ApiError" {
							w.WriteHeader(http.StatusInternalServerError)
							errJson, _ := json.Marshal(SR{
								"error": err.Error(),
							})
							w.Write(errJson)
						} else {
							errAPI := err.(ApiError)
							w.WriteHeader(errAPI.HTTPStatus)
							errJson, _ := json.Marshal(SR{
								"error": errAPI.Err.Error(),
							})
							w.Write(errJson)
						}
					} else {
						newObjJson, _ := json.Marshal(SR{
							"error":    "",
							"response": newObj,
						})
						w.WriteHeader(http.StatusOK)
						w.Write(newObjJson)
					}`, funcDecl.Name.Name))
					fmt.Fprintln(out, `}`)

				}
			}
		}
	}
	for keyResponseName, valueResponseBody := range responseNamesRelatedError {
		fmt.Fprintln(out, fmt.Sprintf(`var %[1]s, _ = json.Marshal(map[string]string{
			%[2]s
		})`, keyResponseName, valueResponseBody))
	}
	for keyApiName, valueCases := range serveHTTPObjects {
		fmt.Fprintln(out, fmt.Sprintf(`func (srv *%s) ServeHTTP(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {`, keyApiName))
		for _, oneCase := range valueCases {
			fmt.Fprintln(out, fmt.Sprintf(`case "%[1]s":
						srv.%[2]s(w, r)`, oneCase.Url, oneCase.Handler))
		}
		fmt.Fprintln(out, `default:
			w.WriteHeader(http.StatusNotFound)
			w.Write(unknownMethodResponse)
		}}`)
	}
}

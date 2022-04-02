# hw5_codegen

Code generation is very widely used in Go and you need to be able to use this tool.
 
In this task, you will need to write a code generator that looks for methods of a structure marked with a special label and generates the following code for them:
* http wrappers for these methods
* authorization check
* method checks (GET/POST)
* parameter validation
* filling the structure with method parameters
* handling unknown errors
 
Those. you write a program (in the file `handlers_gen/codegen.go`) and then run it, passing as parameters the path to the file for which you want to generate the code, and the path to the file in which to write the result. The run will look something like this: `go build handlers_gen/* && ./codegen api.go api_handlers.go`. Those. it will run as `codegenerator_binary what_parse.go where_parse.go`
 
No need to hardcode. All data - field names, available values, boundary values ​​- everything is taken from the structrute itself, `struct tags apivalidator` and the code that we are parsing.
 
If you manually enter the name of the structure, which should get into the resulting code after generation, then you are doing it wrong, even if your tests pass. Your code generator should work universally for any fields and values ​​from those known to it. You need to write code in such a way that it works on code you don’t know, similar to api.go.
 
The only thing you can use is `type ApiError struct` when checking for an error. We think that this is some kind of well-known structure.
 
The code generator can process the following types of structure fields:
* `int`
* `string`
 
The following `apvalidator` placeholder validator labels are available to us:
* `required` - the field must not be empty (should not have a default value)
* `paramname` - if specified, then take from the parameter with this name, otherwise `lowercase` from the name
* `enum` - "one of"
* `default` - if specified and an empty value comes (default value) - set what is written in `default`
* `min` - >= X for `int` type, for strings `len(str)` >=
* `max` - <= X for `int` type
 
For the error format, see the tests. Error order:
* method presence (in `ServeHTTP`)
* method (POST)
* authorization
* parameters in the order in the structure
 
Authorization is checked simply for the fact that the value `100500` has come in the header
 
The generated code will have something like this chain
 
`ServeHTTP` - accepts all methods from the multiplexer, if found - calls `handler$methodName`, if not - says `404`
`handler$methodName` - a wrapper over a method of the `$methodName` structure - performs all checks, outputs errors or a result in `JSON` format
`$methodName` is directly a structure method for which we generate code and which we parse. Prefixed with `apigen:api` followed by `json` with the method name, type, and authorization requirement. You don't need to generate it, it's already there.
 
``` go
type SomeStructName struct{}
 
func (h *SomeStructName ) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {
    case "...":
        h.wrapperDoSomeJob(w, r)
    default:
        // 404
    }
}
 
func (h *SomeStructName ) wrapperDoSomeJob() {
    // filling in the params structure
    // parameter validation
    res, err := h.DoSomeJob(ctx, params)
    // other processing
}
```
 
By the structure of the code generator - you need to find all the methods, for each method generate validation of incoming parameters and other checks in `handler$methodName`, for a bunch of structure methods generate a binding in `ServeHTTP`
 
You don't have to worry much about code generator errors - the parameters that are passed to it will be considered guaranteed correct.
 
What needs to be parsed in ast:
* `node.Decls` -> `ast.FuncDecl` is a method. it needs to check that it has a label and start generating a wrapper for it
* `node.Decls` -> `ast.GenDecl` -> `spec.(*ast.TypeSpec)` + `currType.Type.(*ast.StructType)` is a structure. it is needed to generate validation for the method that we found in the previous paragraph
* https://golang.org/pkg/go/ast/#FuncDecl - here you can see which structure the method belongs to

Adviсe:
* You can use both templates to generate the whole method at once, or collect code from small pieces.
* The easiest way to implement in 2 passes - for the first to collect everything that needs to be generated, for the second - the actual code generation
* It will be necessary to convert a lot from interfaces, see what is always there `fmt.Printf("type: %T data: %+v\n", val, val)`

Directory structure:
* example/ - an example with code generation from the 3rd lecture of the 1st part of the course. You can take this code as a basis.
* handlers_gen/codegen.go - this is where you write your code
* api.go - you need to feed this file to the code generator. no need to edit it
* main.go - everything is clear here. no need to edit
* main_test.go - this file should be run for testing after code generation. no need to edit

The tests will run like this:
``` shell
# being in this folder
# .exe extension only for lucky windows owners
# collect 

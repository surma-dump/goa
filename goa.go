package main

import (
	"github.com/voxelbrain/goptions"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"regexp"
	"strings"
	"text/template"
)

var (
	exportMatcher    = regexp.MustCompile("^goa-export ([[:word:]]+)$")
	protobufTemplate *template.Template
)

func init() {
	protobufTemplate = template.Must(template.New("protobufTemplate").Funcs(template_funcs).Parse(PROTOBUF_TEMPLATE))
}

type ExportedFunc struct {
	GoName       string
	ExportedName string
	Type         *ast.FuncType
}

func main() {
	options := struct {
		Keep          bool `goptions: -k, --keep-temporary, description='Don't delete temporary files'"`
		goptions.Help `goptions:"-h, --help, description='Show this help'"`
		goptions.Verbs
		Init struct {
		} `goptions:"init"`
		Build struct {
			GoFile *os.File `goptions:"-g, --go-file, description='Go file to generate stubs for', rdonly"`
		} `goptions:"build"`
	}{}
	goptions.ParseAndFail(&options)
	if len(options.Verbs) == 0 {
		goptions.PrintHelp()
		return
	}

	protobuffile, err := os.Create("def.proto")
	if err != nil {
		log.Fatalf("Could not create output file 'def.proto': %s", err)
	}

	switch options.Verbs {
	case "build":
		fset := token.NewFileSet()
		tree, err := parser.ParseFile(fset, "", options.Build.GoFile, parser.ParseComments)
		if err != nil {
			log.Fatalf("Could not parse file: %s", err)
		}
		exported_funcs := []*ExportedFunc{}
		for _, decl := range tree.Decls {
			if f, ok := decl.(*ast.FuncDecl); ok {
				// If there's no comment at all, there's
				// definitely no "export" comment ;)
				if f.Doc == nil || f.Doc.List == nil {
					continue
				}
				for _, comment := range f.Doc.List {
					stripped_comment := strings.Trim(comment.Text, "/\t ")
					if strings.HasPrefix(stripped_comment, "goa-export") {
						if f.Recv != nil {
							log.Fatalf("Methods cannot be exported")
						}
						exported_funcs = append(exported_funcs, &ExportedFunc{
							GoName:       f.Name.Name,
							ExportedName: strings.TrimSpace(stripped_comment[len("goa-export"):]),
							Type:         f.Type,
						})
					}
				}
			}
		}

		err = protobufTemplate.Execute(protobuffile, exported_funcs)
		if err != nil {
			log.Fatalf("Protobuf rendering failed: %s", err)
		}
		protobuffile.Close()

	default:
		panic("Not implemented")
	}
}

const PROTOBUF_TEMPLATE = `
{{define "rule"}} {{if isArray .Type}} repeated {{else}} required {{end}} {{end}}
{{define "type"}} {{if isArray .Type}} {{with toArray .Type}}{{.Elt}}{{end}} {{else}} {{.Type}} {{end}} {{end}}
{{define "parameters"}}
	{{range $fidx, $field := .}}
		{{range $nidx, $name := .Names}}
			{{template "rule" $field}} {{template "type" $field}} {{$name.Name}}
		{{else}}
			{{template "rule" $field}} {{template "type" $field}} r_{{$fidx}}_0
		{{end}}
	{{end}}
{{end}}
{{range .}}

message {{.ExportedName}}_call {
	required int id
	{{with .Type.Params.List}}{{template "parameters" .}}{{end}}
}

message {{.ExportedName}}_result {
	required int id
	{{with .Type.Results.List}}{{template "parameters" .}}{{end}}
}
{{end}}
`

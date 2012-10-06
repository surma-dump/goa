package main

import (
	"fmt"
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

type TemplateData struct {
	Id   int
	Rule string
	Type string
	Name string
}
type ExportedFunc struct {
	GoName       string
	ExportedName string
	Type         *ast.FuncType
	Params       []TemplateData
	Results      []TemplateData
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

		exported_funcs := exportedFunctions(tree.Decls)
		for _, ef := range exported_funcs {
			ef.Params = templateData(ef.Type.Params)
			ef.Results = templateData(ef.Type.Results)
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

func exportedFunctions(decls []ast.Decl) []*ExportedFunc {
	ef := []*ExportedFunc{}
	for _, decl := range decls {
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
					ef = append(ef, &ExportedFunc{
						GoName:       f.Name.Name,
						ExportedName: strings.TrimSpace(stripped_comment[len("goa-export"):]),
						Type:         f.Type,
					})
				}
			}
		}
	}
	return ef
}

func templateData(f *ast.FieldList) []TemplateData {
	tld := []TemplateData{}
	id := 2
	for _, field := range f.List {
		entry := TemplateData{
			Type: "<unsupported>",
			Rule: "required",
			Name: "<anonymous>",
		}
		switch t := field.Type.(type) {
		case *ast.Ident:
			entry.Type = t.Name
		case *ast.ArrayType:
			entry.Type = t.Elt.(*ast.Ident).Name
			entry.Rule = "repeated"
		}
		if entry.Type == "int" {
			entry.Type = "int64"
		}
		if field.Names == nil {
			// Anonymous fields (e.g. return field list)
			entry.Id = id
			entry.Name = fmt.Sprintf("f_%d", entry.Id)
			tld = append(tld, entry)
			id++
		} else {
			for _, name := range field.Names {
				entry := entry //copy
				entry.Id = id
				entry.Name = name.Name
				tld = append(tld, entry)
				id++
			}
		}
	}
	return tld
}

const PROTOBUF_TEMPLATE = `
{{define "function"}}
	required int64 callid = 1;
	{{range .}}
	{{.Rule}} {{.Type}} {{.Name}} = {{.Id}};
	{{end}}
{{end}}

{{range .}}
message {{.ExportedName}}_call {
	{{template "function" .Params}}
}
message {{.ExportedName}}_result {
	{{template "function" .Results}}
}
{{end}}
`

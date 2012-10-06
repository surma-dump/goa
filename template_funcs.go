package main

import (
	"go/ast"
)

var (
	template_funcs = map[string]interface{}{
		"isArray": isArray,
		"toArray": toArray,
	}
)

func isArray(e ast.Expr) bool {
	_, ok := e.(*ast.ArrayType)
	return ok
}

func toArray(e ast.Expr) *ast.ArrayType {
	return e.(*ast.ArrayType)
}

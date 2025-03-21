package processing

import (
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
)

func FindNodeByID(stack *nodestack.NodeStack, id ast.Identifier) ast.Node {
	nodes := append([]ast.Node{}, stack.From)
	nodes = append(nodes, stack.Stack...)

	for _, node := range nodes {
		switch curr := node.(type) {
		case *ast.Function:
			for _, param := range curr.Parameters {
				if param.Name == id {
					return param.DefaultArg
				}
			}
		case *ast.Local:
			for _, bind := range curr.Binds {
				if bind.Variable == id {
					return bind.Body
				}
			}
		case *ast.DesugaredObject:
			for _, bind := range curr.Locals {
				if bind.Variable == id {
					return bind.Body
				}
			}
		}
	}
	return nil
}

func FindBindByIDViaStack(stack *nodestack.NodeStack, id ast.Identifier) *ast.LocalBind {
	nodes := append([]ast.Node{}, stack.From)
	nodes = append(nodes, stack.Stack...)
	for _, node := range nodes {
		switch curr := node.(type) {
		case *ast.Local:
			for _, bind := range curr.Binds {
				if bind.Variable == id {
					return &bind
				}
			}
		case *ast.DesugaredObject:
			for _, bind := range curr.Locals {
				if bind.Variable == id {
					return &bind
				}
			}
		}
	}
	return nil
}

package processing

import (
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	"github.com/sirupsen/logrus"
)

func FindNodeByID(stack *nodestack.NodeStack, id ast.Identifier) ast.Node {
	nodes := append([]ast.Node{}, stack.From)
	nodes = append(nodes, stack.Stack...)
	var defaultVal ast.Node

	for i := range nodes {
		node := nodes[len(nodes)-1-i]
		logrus.Errorf("Searching in node %T", node)
		switch curr := node.(type) {
		case *ast.Function:
			for _, param := range curr.Parameters {
				if param.Name == id && defaultVal == nil {
					defaultVal = param.DefaultArg
				}
			}
		case *ast.Local:
			for _, bind := range curr.Binds {
				logrus.Errorf("Got bind with id %v. Searching for %v, Body %T", bind.Variable, id, bind.Body)
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
	// Only return the default val if we can't find the actual var
	// TODO: this breaks overwriting variables
	// local a = 5;
	// function(a=3) a
	// function() -> should return 3
	return defaultVal
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

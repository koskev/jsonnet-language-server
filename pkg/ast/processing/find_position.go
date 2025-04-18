package processing

import (
	"errors"

	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/toolutils"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
)

// TODO: unsterstand why this function breaks other code
func FindNodeByPositionForReference(node ast.Node, location ast.Location) (*nodestack.NodeStack, error) {
	if node == nil {
		return nil, errors.New("node is nil")
	}

	stack := nodestack.NewNodeStack(node)
	// keeps the history of the navigation path to the requested Node.
	// used to backwards search Nodes from the found node to the root.
	searchStack := &nodestack.NodeStack{From: stack.From}
	var curr ast.Node
	for !stack.IsEmpty() {
		curr = stack.Pop()
		// This is needed because SuperIndex only spans "key: super" and not the ".foo" after. This only occurs
		// when super only has 1 additional index. "super.foo.bar" will not have this issue
		if curr, isType := curr.(*ast.SuperIndex); isType {
			var indexLength int
			if index, ok := curr.Index.(*ast.LiteralString); ok {
				indexLength = len(index.Value)
			}
			curr.Loc().End.Column = curr.Loc().End.Column + indexLength + 1
		}
		inRange := InRange(location, *curr.Loc())
		if inRange {
			searchStack.Push(curr)
		}
		// else if curr.Loc().End.IsSet() {
		// TODO: Why was this needed?
		// TODO: reimplement this function
		// continue

		switch curr := curr.(type) {
		case *ast.DesugaredObject:
			for _, field := range curr.Fields {
				body := field.Body
				// Functions do not have a LocRange, so we use the one from the field's body
				if funcBody, isFunc := body.(*ast.Function); isFunc {
					funcBody.LocRange = field.LocRange
				}
				stack.Push(body)

				if nameNode, isLiteral := field.Name.(*ast.LiteralString); isLiteral {
					nameNode.LocRange = field.LocRange
					stack.Push(nameNode)
				}
			}
			for _, local := range curr.Locals {
				stack.Push(local.Body)
			}
			for _, assert := range curr.Asserts {
				stack.Push(assert)
			}
		default:
			for _, c := range toolutils.Children(curr) {
				stack.Push(c)
			}
		}
	}
	return searchStack.ReorderDesugaredObjects(), nil
}

func FindNodeByPosition(node ast.Node, location ast.Location) (*nodestack.NodeStack, error) {
	if node == nil {
		return nil, errors.New("node is nil")
	}

	stack := nodestack.NewNodeStack(node)
	// keeps the history of the navigation path to the requested Node.
	// used to backwards search Nodes from the found node to the root.
	searchStack := &nodestack.NodeStack{From: stack.From}
	var curr ast.Node
	for !stack.IsEmpty() {
		curr = stack.Pop()
		// This is needed because SuperIndex only spans "key: super" and not the ".foo" after. This only occurs
		// when super only has 1 additional index. "super.foo.bar" will not have this issue
		if curr, isType := curr.(*ast.SuperIndex); isType {
			var indexLength int
			if index, ok := curr.Index.(*ast.LiteralString); ok {
				indexLength = len(index.Value)
			}
			curr.Loc().End.Column = curr.Loc().End.Column + indexLength + 1
		}
		inRange := InRange(location, *curr.Loc())
		if inRange {
			searchStack.Push(curr)
		} else if curr.Loc().End.IsSet() {
			continue
		}

		switch curr := curr.(type) {
		case *ast.DesugaredObject:
			for _, field := range curr.Fields {
				body := field.Body
				// Functions do not have a LocRange, so we use the one from the field's body
				if funcBody, isFunc := body.(*ast.Function); isFunc {
					funcBody.LocRange = field.LocRange
					stack.Push(funcBody)
				} else {
					stack.Push(field.Name)
					stack.Push(body)
				}
			}
			for _, local := range curr.Locals {
				stack.Push(local.Body)
			}
			for _, assert := range curr.Asserts {
				stack.Push(assert)
			}
		default:
			for _, c := range toolutils.Children(curr) {
				stack.Push(c)
			}
		}
	}
	return searchStack.ReorderDesugaredObjects(), nil
}

func FindNodeInStack(node ast.Node, documentstack *nodestack.NodeStack) (*nodestack.NodeStack, error) {
	if node == nil {
		return nil, errors.New("node is nil")
	}

	stack := nodestack.NewNodeStack(node)
	searchStack := documentstack.Clone()
	var curr ast.Node
	for !searchStack.IsEmpty() {
		curr = searchStack.Pop()
		inRange := InRange(node.Loc().Begin, *curr.Loc()) && node.Loc().FileName == curr.Loc().FileName
		if inRange {
			stack.PushFront(curr)
		}
	}
	return stack, nil
}

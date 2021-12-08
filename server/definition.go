package server

import (
	"errors"
	"fmt"
	"sort"

	"github.com/google/go-jsonnet/ast"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

type NodeStack struct {
	stack []ast.Node
}

func (s *NodeStack) Push(n ast.Node) *NodeStack {
	s.stack = append(s.stack, n)
	return s
}

func (s *NodeStack) Pop() (*NodeStack, ast.Node) {
	l := len(s.stack)
	n := s.stack[l-1]
	s.stack = s.stack[:l-1]
	return s, n
}

func (s *NodeStack) IsEmpty() bool {
	return len(s.stack) == 0
}

func (s *NodeStack) reorderDesugaredObjects() *NodeStack {
	sort.SliceStable(s.stack, func(i, j int) bool {
		_, iIsDesugared := s.stack[i].(*ast.DesugaredObject)
		_, jIsDesugared := s.stack[j].(*ast.DesugaredObject)
		if !iIsDesugared && !jIsDesugared {
			return false
		}

		iLoc, jLoc := s.stack[i].Loc(), s.stack[j].Loc()
		if iLoc.Begin.Line < jLoc.Begin.Line && iLoc.End.Line > jLoc.End.Line {
			return true
		}

		return false
	})
	return s
}

func Definition(node ast.Node, params protocol.DefinitionParams) (protocol.DefinitionLink, error) {
	responseDefLink, _ := findDefinition(node, params.Position)
	return *responseDefLink, nil
}

func findDefinition(root ast.Node, position protocol.Position) (*protocol.DefinitionLink, error) {
	searchStack, _ := findNodeByPosition(root, position)
	var deepestNode ast.Node
	searchStack, deepestNode = searchStack.Pop()
	var responseDefLink protocol.DefinitionLink
	switch deepestNode := deepestNode.(type) {
	case *ast.Var:
		var matchingBind *ast.LocalBind
		matchingBind, _ = findBindByIdViaStack(searchStack, deepestNode.Id)
		foundLocRange := &matchingBind.LocRange
		if foundLocRange.Begin.Line == 0 {
			foundLocRange = matchingBind.Body.Loc()
		}
		responseDefLink = protocol.DefinitionLink{
			TargetURI: protocol.DocumentURI(foundLocRange.FileName),
			TargetRange: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(foundLocRange.Begin.Line - 1),
					Character: uint32(foundLocRange.Begin.Column - 1),
				},
				End: protocol.Position{
					Line:      uint32(foundLocRange.End.Line - 1),
					Character: uint32(foundLocRange.End.Column - 1),
				},
			},
			TargetSelectionRange: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(foundLocRange.Begin.Line - 1),
					Character: uint32(foundLocRange.Begin.Column - 1),
				},
				End: protocol.Position{
					Line:      uint32(foundLocRange.Begin.Line - 1),
					Character: uint32(foundLocRange.Begin.Column - 1 + len(matchingBind.Variable)),
				},
			},
		}
	case *ast.SuperIndex, *ast.Index:
		indexSearchStack := NodeStack{}
		indexSearchStack.Push(deepestNode)
		indexList := buildIndexList(&indexSearchStack)
		tempSearchStack := *searchStack
		matchingField, err := findObjectFieldFromIndexList(&tempSearchStack, indexList)
		if err != nil {
			return nil, err
		}
		foundLocRange := &matchingField.LocRange
		responseDefLink = protocol.DefinitionLink{
			TargetURI: protocol.DocumentURI(foundLocRange.FileName),
			TargetRange: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(foundLocRange.Begin.Line - 1),
					Character: uint32(foundLocRange.Begin.Column - 1),
				},
				End: protocol.Position{
					Line:      uint32(foundLocRange.End.Line - 1),
					Character: uint32(foundLocRange.End.Column - 1),
				},
			},
			TargetSelectionRange: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(foundLocRange.Begin.Line - 1),
					Character: uint32(foundLocRange.Begin.Column - 1),
				},
				End: protocol.Position{
					Line:      uint32(foundLocRange.Begin.Line - 1),
					Character: uint32(foundLocRange.Begin.Column - 1 + len(matchingField.Name.(*ast.LiteralString).Value)),
				},
			},
		}
	}
	return &responseDefLink, nil
}

func buildIndexList(stack *NodeStack) []string {
	var indexList []string
	for !stack.IsEmpty() {
		_, curr := stack.Pop()
		switch curr := curr.(type) {
		case *ast.SuperIndex:
			stack = stack.Push(curr.Index)
			indexList = append(indexList, "super")
		case *ast.Index:
			stack = stack.Push(curr.Index)
			stack = stack.Push(curr.Target)
		case *ast.LiteralString:
			indexList = append(indexList, curr.Value)
		case *ast.Self:
			indexList = append(indexList, "self")
		case *ast.Var:
			indexList = append(indexList, string(curr.Id))
		}
	}
	return indexList
}

func findObjectFieldFromIndexList(stack *NodeStack, indexList []string) (*ast.DesugaredObjectField, error) {
	var foundField *ast.DesugaredObjectField
	var foundDesugaredObject *ast.DesugaredObject
	// First element will be super, self, or var name
	start, indexList := indexList[0], indexList[1:]
	if start == "super" {
		// Find the LHS desugared object of a binary node
		foundDesugaredObject = findLhsDesugaredObject(stack)
	} else if start == "self" {
		// Get the most recent ast.DesugaredObject as that will be our self object
		foundDesugaredObject = findDesugaredObjectFromStack(stack)
	} else {
		// Get ast.DesugaredObject at variable definition by getting bind then setting ast.DesugaredObject
		bind, _ := findBindByIdViaStack(stack, ast.Identifier(start))
		foundDesugaredObject = bind.Body.(*ast.DesugaredObject)
	}
	for len(indexList) > 0 {
		index := indexList[0]
		indexList = indexList[1:]
		foundField = findObjectFieldInObject(foundDesugaredObject, index)
		if foundField == nil {
			return nil, fmt.Errorf("field was not found in ast.DesugaredObject")
		}
		switch fieldNode := foundField.Body.(type) {
		case *ast.Var:
			bind, _ := findBindByIdViaStack(stack, fieldNode.Id)
			foundDesugaredObject = bind.Body.(*ast.DesugaredObject)
		case *ast.DesugaredObject:
			stack = stack.Push(fieldNode)
			foundDesugaredObject = findDesugaredObjectFromStack(stack)
		case *ast.Index:
			tempStack := &NodeStack{}
			tempStack = tempStack.Push(fieldNode)
			additionalIndexList := buildIndexList(tempStack)
			additionalIndexList = append(additionalIndexList, indexList...)
			return findObjectFieldFromIndexList(stack, additionalIndexList)
		}
	}
	return foundField, nil
}

func findObjectFieldInObject(objectNode *ast.DesugaredObject, index string) *ast.DesugaredObjectField {
	for _, field := range objectNode.Fields {
		literalString := field.Name.(*ast.LiteralString)
		if index == literalString.Value {
			return &field
		}
	}
	return nil
}

func findDesugaredObjectFromStack(stack *NodeStack) *ast.DesugaredObject {
	for !stack.IsEmpty() {
		_, curr := stack.Pop()
		switch curr := curr.(type) {
		case *ast.DesugaredObject:
			return curr
		case *ast.Local:

		}
	}
	return nil
}

func findLhsDesugaredObject(stack *NodeStack) *ast.DesugaredObject {
	for !stack.IsEmpty() {
		_, curr := stack.Pop()
		switch curr := curr.(type) {
		case *ast.Binary:
			lhsNode := curr.Left
			switch lhsNode := lhsNode.(type) {
			case *ast.DesugaredObject:
				return lhsNode
			case *ast.Var:
				bind, _ := findBindByIdViaStack(stack, lhsNode.Id)
				return bind.Body.(*ast.DesugaredObject)
			}
		case *ast.Local:
			for _, bind := range curr.Binds {
				stack = stack.Push(bind.Body)
			}
			if curr.Body != nil {
				stack = stack.Push(curr.Body)
			}
		}
	}
	return nil
}

func findBindByIdViaStack(stack *NodeStack, id ast.Identifier) (*ast.LocalBind, error) {
	for !stack.IsEmpty() {
		_, curr := stack.Pop()
		switch curr := curr.(type) {
		case *ast.Local:
			for _, bind := range curr.Binds {
				if bind.Variable == id {
					return &bind, nil
				}
			}
		case *ast.DesugaredObject:
			for _, bind := range curr.Locals {
				if bind.Variable == id {
					return &bind, nil
				}
			}
		}
	}
	return nil, nil
}

func findNodeByPosition(node ast.Node, position protocol.Position) (*NodeStack, error) {
	if node == nil {
		return nil, errors.New("node is nil")
	}

	stack := &NodeStack{}
	stack.Push(node)
	// keeps the history of the navigation path to the requested Node.
	// used to backwards search Nodes from the found node to the root.
	searchStack := &NodeStack{}
	var curr ast.Node
	for !stack.IsEmpty() {
		stack, curr = stack.Pop()
		// This is needed because SuperIndex only spans "key: super" and not the ".foo" after. This only occurs
		// when super only has 1 additional index. "super.foo.bar" will not have this issue
		if curr, isType := curr.(*ast.SuperIndex); isType {
			curr.Loc().End.Column = curr.Loc().End.Column + len(curr.Index.(*ast.LiteralString).Value) + 1
		}
		inRange := inRange(position, *curr.Loc())
		if inRange {
			searchStack = searchStack.Push(curr)
		} else if curr.Loc().End.IsSet() {
			continue
		}
		switch curr := curr.(type) {
		case *ast.Local:
			for _, bind := range curr.Binds {
				stack = stack.Push(bind.Body)
			}
			if curr.Body != nil {
				stack = stack.Push(curr.Body)
			}
		case *ast.DesugaredObject:
			for _, field := range curr.Fields {
				stack = stack.Push(field.Body)
			}
			for _, local := range curr.Locals {
				stack = stack.Push(local.Body)
			}
		case *ast.Binary:
			stack = stack.Push(curr.Left)
			stack = stack.Push(curr.Right)
		case *ast.Array:
			for _, element := range curr.Elements {
				stack = stack.Push(element.Expr)
			}
		case *ast.Apply:
			for _, posArg := range curr.Arguments.Positional {
				stack = stack.Push(posArg.Expr)
			}
			for _, namedArg := range curr.Arguments.Named {
				stack = stack.Push(namedArg.Arg)
			}
			stack = stack.Push(curr.Target)
		case *ast.Conditional:
			stack = stack.Push(curr.Cond)
			stack = stack.Push(curr.BranchTrue)
			stack = stack.Push(curr.BranchFalse)
		case *ast.Error:
			stack = stack.Push(curr.Expr)
		case *ast.Function:
			for _, param := range curr.Parameters {
				if param.DefaultArg != nil {
					stack = stack.Push(param.DefaultArg)
				}
			}
			stack = stack.Push(curr.Body)
		case *ast.Index:
			stack = stack.Push(curr.Target)
			stack = stack.Push(curr.Index)
		case *ast.InSuper:
			stack = stack.Push(curr.Index)
		case *ast.SuperIndex:
			stack = stack.Push(curr.Index)
		case *ast.Unary:
			stack = stack.Push(curr.Expr)
		}
	}
	return searchStack.reorderDesugaredObjects(), nil
}

func inRange(point protocol.Position, theRange ast.LocationRange) bool {
	if int(point.Line) == theRange.Begin.Line-1 && int(point.Character) < theRange.Begin.Column-1 {
		return false
	}

	if int(point.Line) == theRange.End.Line-1 && int(point.Character) >= theRange.End.Column-1 {
		return false
	}

	if int(point.Line) != theRange.Begin.Line-1 || int(point.Line) != theRange.End.Line-1 {
		return theRange.Begin.Line-1 <= int(point.Line) && int(point.Line) <= theRange.End.Line-1
	}

	return true
}

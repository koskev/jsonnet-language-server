package nodetree

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/toolutils"
)

type NodeTree struct {
	Parent   *NodeTree
	Children []*NodeTree

	Node ast.Node
}

func BuildTree(parent *NodeTree, node ast.Node) *NodeTree {
	tree := &NodeTree{
		Parent: parent,
		Node:   node,
	}
	children := toolutils.Children(node)
	for _, child := range children {
		tree.Children = append(tree.Children, BuildTree(tree, child))
	}

	return tree
}

// Why are Go generics to shitty!?!?
func GetTopNodesOfType[T ast.Node](tree *NodeTree) []T {
	var children []T
	if node, ok := tree.Node.(T); ok {
		children = append(children, node)
	} else {
		for _, child := range tree.Children {
			children = append(children, GetTopNodesOfType[T](child)...)
		}
	}
	return children
}

func (t *NodeTree) GetDeepestNodes() []ast.Node {
	var children []ast.Node
	if len(t.Children) == 0 {
		children = append(children, t.Node)
	} else {
		for _, child := range t.Children {
			children = append(children, child.GetDeepestNodes()...)
		}
	}
	return children
}

func (t *NodeTree) GetAllChildren() []ast.Node {
	var children []ast.Node

	children = append(children, t.Node)
	for _, child := range t.Children {
		children = append(children, child.GetAllChildren()...)
	}

	return children
}

func (t *NodeTree) String() string {
	var output strings.Builder

	output.WriteString(fmt.Sprintf("%v at %v", reflect.TypeOf(t.Node), t.Node.Loc()))
	switch node := t.Node.(type) {
	case *ast.Local:
		output.WriteString(fmt.Sprintf("Body Type %T", node.Body))
		for _, bind := range node.Binds {
			output.WriteString(fmt.Sprintf("BIND: %+v of type %T", bind.Variable, bind.Body))
		}
	case *ast.LiteralString:
		output.WriteString(fmt.Sprintf(" %s", node.Value))
	case *ast.Var:
		output.WriteString(fmt.Sprintf(" %s", node.Id))
	case *ast.Index:
		if nameNode, ok := node.Index.(*ast.LiteralString); ok {
			output.WriteString(fmt.Sprintf(" %s", nameNode.Value))
		}
		output.WriteString(fmt.Sprintf(" %T", node.Target))
	}
	output.WriteString("\n")
	for _, child := range t.Children {
		childString := child.String()
		for _, line := range strings.Split(childString, "\n") {
			output.WriteString(fmt.Sprintf("\t%s\n", line))
		}
	}
	return output.String()
}

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
	case *ast.LiteralString:
		output.WriteString(fmt.Sprintf(" %s", node.Value))
	case *ast.Var:
		output.WriteString(fmt.Sprintf(" %s", node.Id))
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

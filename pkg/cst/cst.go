package cst

import (
	"context"
	"fmt"

	jsonnet "github.com/koskev/tree-sitter-jsonnet/bindings/go"
	"github.com/sirupsen/logrus"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// FUCK YOU GO AND YOUR STUPID ENUMS!
type NodeType string

const (
	NodeSelf           = "self"
	NodeDollar         = "dollar"
	NodeDot            = "."
	NodeColon          = ":"
	NodeClosingBracket = ")"
	NodeSemicolon      = ";"
	NodeFieldAccess    = "fieldaccess"
	NodeFunctionCall   = "functioncall"
	NodeID             = "id"
	NodeLocalBind      = "local_bind"
	NodeLocal          = "local"
	NodeParenthesis    = "parenthesis"
	NodeBind           = "bind"
	NodeImport         = "import"
	NodeError          = "ERROR"
	NodeStringContent  = "string_content"
	NodeStringStart    = "string_start"
	NodeString         = "string"
)

func NewTree(ctx context.Context, content string) (*sitter.Node, error) {
	parser := sitter.NewParser()
	defer parser.Close()
	err := parser.SetLanguage(sitter.NewLanguage(jsonnet.Language()))
	if err != nil {
		return nil, err
	}
	tree := parser.Parse([]byte(content), nil)
	//node, err := parser.Parse(ctx, []byte(content), sitter.NewLanguage(jsonnet.GetLanguage()))
	if tree == nil {
		return nil, fmt.Errorf("parsing tree")
	}
	node := tree.RootNode()
	if node == nil {
		return nil, fmt.Errorf("getting root node")
	}
	return node, nil
}

func GetNodeAtPos(root *sitter.Node, point sitter.Point) *sitter.Node {
	startPos := point
	endPos := point
	if startPos.Column > 0 {
		// Include char before the cursor
		startPos.Column--
	}

	logrus.Tracef("Getting pos at %v and %v", startPos, endPos)

	return root.DescendantForPointRange(startPos, endPos)
}

func IsSymbolNode(node *sitter.Node) bool {
	switch node.GrammarName() {
	case NodeSemicolon, NodeDot, NodeClosingBracket, NodeColon:
		return true
	}
	return false
}

func GetNonSymbolNode(node *sitter.Node) *sitter.Node {
	for node != nil && IsSymbolNode(node) {
		node = node.Parent()
	}
	return node
}

func IsNodeAny(node *sitter.Node, nodeTypes []NodeType) bool {
	for _, nodeType := range nodeTypes {
		if IsNode(node, nodeType) {
			return true
		}
	}
	return false
}

func IsNode(node *sitter.Node, nodeType NodeType) bool {
	// Casting is needed since go is just stupid with "enums"....
	return node != nil && node.GrammarName() == string(nodeType)
}

// Gets the previous node in the tree
func GetPrevNode(node *sitter.Node) *sitter.Node {
	if sibling := node.PrevSibling(); sibling != nil {
		return GetLastChild(sibling)
	}
	// TODO: If node is document we are at the last line and should get the last child in the tree (bottom in neovim)
	return node.Parent()
}

func GetPrevNodeType(node *sitter.Node, nodeType NodeType) *sitter.Node {
	// TODO: is there really no while((retNode := GetPrevNode(node) != nil) in go?!?
	for retNode := GetPrevNode(node); retNode != nil; retNode = GetPrevNode(node) {
		if node != nil && retNode.GrammarName() == string(nodeType) {
			return retNode
		}
	}
	return nil
}

func GetLastChild(node *sitter.Node) *sitter.Node {
	cursor := node.Walk()
	for cursor.GotoLastChild() {
	}
	return cursor.Node()
}

func GetNodeString(node *sitter.Node) string {
	if node != nil {
		return fmt.Sprintf("%s (%d)", node.GrammarName(), node.GrammarId())
	}
	return "nil"
}

func GetFirstChildType(node *sitter.Node, nodeType NodeType) (*sitter.Node, error) {
	if node == nil {
		return nil, fmt.Errorf("node is nil")
	}
	cursor := node.Walk()
	children := node.Children(cursor)
	for _, child := range children {
		if child.GrammarName() == string(nodeType) {
			return &child, nil
		}
	}
	for _, child := range children {
		ret, err := GetFirstChildType(&child, nodeType)
		if err == nil {
			return ret, nil
		}
	}
	return nil, fmt.Errorf("unable to find child for type %s", nodeType)
}

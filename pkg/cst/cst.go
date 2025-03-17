package cst

import (
	"context"
	"fmt"

	jsonnet "github.com/koskev/tree-sitter-jsonnet/bindings/go"
	"github.com/sirupsen/logrus"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

const (
	NodeSelf           = 6
	NodeDollar         = 7
	NodeDot            = 16
	NodeColon          = 17
	NodeClosingBracket = 19
	NodeSemicolon      = 20
	NodeFieldAccess    = 71
	NodeFunctionCall   = 75
	NodeID             = 76
	NodeLocalBind      = 77
	NodeBind           = 104
	NodeError          = 65535
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
	switch node.GrammarId() {
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

func IsNodeAny(node *sitter.Node, nodeTypes []uint16) bool {
	for _, nodeType := range nodeTypes {
		if IsNode(node, nodeType) {
			return true
		}
	}
	return false
}

func IsNode(node *sitter.Node, nodeType uint16) bool {
	return node != nil && node.GrammarId() == nodeType
}

// Gets the previous node in the tree
func GetPrevNode(node *sitter.Node) *sitter.Node {
	if sibling := node.PrevSibling(); sibling != nil {
		return GetLastChild(sibling)
	}
	// TODO: If node is document we are at the last line and should get the last child in the tree (bottom in neovim)
	return node.Parent()
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

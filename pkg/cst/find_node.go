package cst

import (
	"context"
	"fmt"
	"slices"
	"strings"

	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	log "github.com/sirupsen/logrus"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type CompletionNodeInfo struct {
	Node        *tree_sitter.Node
	InjectIndex bool
	Global      bool
	Index       string
}

func getCurrentIndex(content string, pos protocol.Position) string {
	whitespace := []rune{' ', '\n', '\t'}
	endingTokens := []rune{':', ',', ';', '(', '='}

	currentPos := 0
	// FUCK YOU GO AND YOUR NON EXISTING FEATURES T_T
	currentLine := 0
	for i, char := range content {
		if currentLine == int(pos.Line) {
			currentPos = i + int(pos.Character) - 1
			break
		}
		if char == '\n' {
			currentLine += 1
		}
	}

	index := []rune{}

	for {
		currentRune := rune(content[currentPos])
		log.Errorf("Current rune '%v'", string(currentRune))
		if slices.Contains(endingTokens, currentRune) {
			break
		}
		if !slices.Contains(whitespace, currentRune) {
			index = append([]rune{currentRune}, index...)
		}
		if currentPos == 0 {
			break
		} else {
			currentPos -= 1
		}
	}
	return string(index)
}

func FindCompletionNode(ctx context.Context, content string, pos protocol.Position) (*CompletionNodeInfo, error) {
	var info CompletionNodeInfo

	// search if prev token is a :,;
	currentIndex := getCurrentIndex(content, pos)
	log.Errorf("################# current index %v", currentIndex)

	if !strings.Contains(currentIndex, ".") {
		info.Global = true
		info.Index = currentIndex
	}

	root, err := NewTree(ctx, content)
	if err != nil {
		return nil, err
	}
	found := GetNodeAtPos(root, position.ProtocolToCST(pos))
	log.Errorf("Found: %v", found.GrammarName())

	switch found.GrammarName() {
	case NodeDot:
		info.InjectIndex = true
		potentialNode := GetPrevNode(found)
		// myFunc(1).
		if potentialNode.GrammarName() == NodeClosingBracket {
			found = found.PrevSibling()
		} else {
			// myObj.
			found = potentialNode
		}
	case NodeLocal:
		// Probably global
		info.Global = true
	}

	// Inside an Object the node is an error if it ends in a dot
	if IsNode(found, NodeError) {
		fieldAccessNode := found.PrevSibling()
		if fieldAccessNode == nil {
			return nil, fmt.Errorf("access node is nil")
		}
		log.Errorf("Node id %v (%v)", fieldAccessNode.GrammarName(), fieldAccessNode.GrammarId())
		switch fieldAccessNode.GrammarName() {
		case NodeBind:
			fieldAccessNode = GetLastChild(fieldAccessNode)
		case NodeFunctionCall:

			acc, err := GetFirstChildType(fieldAccessNode, NodeFieldAccess)
			if err != nil {
				return nil, err
			}
			fieldAccessNode = GetLastChild(acc)
		case NodeParenthesis:
			// TODO: probably more cases here
			fieldAccessNode, err = GetFirstChildType(fieldAccessNode, NodeImport)
			if err != nil {
				return nil, err
			}
		}
		// fieldAccessNode, err := GetFirstChildType(sibling, NodeFieldAccess)
		if fieldAccessNode == nil {
			return nil, fmt.Errorf("no access node found")
		}
		found = GetNonSymbolNode(GetLastChild(fieldAccessNode))
	}
	info.Node = found
	log.Errorf("Found end: %+v", found.GrammarName())

	return &info, nil
}

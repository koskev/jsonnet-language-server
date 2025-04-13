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

type CompletionType = int

const (
	CompleteGlobal = iota
	CompleteLocal
	CompleteImport
)

type CompletionNodeInfo struct {
	Node           *tree_sitter.Node
	InjectIndex    bool
	Index          string
	CompletionType CompletionType
}

func positionToIndex(content string, pos protocol.Position) int {
	currentPos := 0
	currentLine := 0
	for i, char := range content {
		if currentLine == int(pos.Line) {
			currentPos = i + int(pos.Character)
			break
		}
		if char == '\n' {
			currentLine++
		}
	}
	return currentPos
}

func getCurrentIndex(content string, pos protocol.Position) string {
	whitespace := []rune{' ', '\n', '\t'}
	endingTokens := []rune{':', ',', ';', '(', '=', '[', '+'}

	// We want the position before the cursor
	currentPos := max(0, positionToIndex(content, pos)-1)

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
		}
		currentPos--
	}
	return string(index)
}

func FindCompletionNode(ctx context.Context, content string, pos protocol.Position) (*CompletionNodeInfo, error) {
	var info CompletionNodeInfo

	// search if prev token is a :,;
	currentIndex := getCurrentIndex(content, pos)
	log.Errorf("################# current index %v from %+v", currentIndex, pos)
	// Default to local completion
	info.CompletionType = CompleteLocal

	if !strings.Contains(currentIndex, ".") {
		info.CompletionType = CompleteGlobal
		info.Index = currentIndex
	}

	root, err := NewTree(ctx, content)
	if err != nil {
		return nil, err
	}
	found := GetNodeAtPos(root, position.ProtocolToCST(pos))
	log.Errorf("#Found: %v (%+v)", found.GrammarName(), found.Range())

	//nolint: gocritic
	switch found.GrammarName() {
	// In import
	case NodeStringStart:
		found = found.NextSibling()
		fallthrough
	case NodeStringContent:
		stringNode := found.Parent()
		if !IsNode(stringNode, NodeString) {
			break
		}
		importNode := stringNode.Parent()
		if IsNode(importNode, NodeImport) {
			info.CompletionType = CompleteImport
			startIndex := positionToIndex(content, position.CSTToProtocol(found.StartPosition()))
			endIndex := positionToIndex(content, position.CSTToProtocol(found.EndPosition()))
			info.Index = content[startIndex:endIndex]
			return &info, nil
		}

	case NodeDot:
		info.InjectIndex = true
		potentialNode := GetPrevNode(found)
		// If we are at the last node, the prev one is an error node. If that is the case we need to skip it
		if IsNode(potentialNode, NodeError) {
			potentialNode = GetPrevNode(potentialNode)
		}
		// myFunc(1).
		if potentialNode.GrammarName() == NodeClosingBracket || potentialNode.GrammarName() == NodeClosingSquareBracket {
			// a: myfunc(arg) the next id would be "arg". Therefore we take a look at the parent and see if it is a function call

			// Get the opening bracket
			for potentialNode != nil && potentialNode.GrammarName() != NodeOpeningBracket && potentialNode.GrammarName() != NodeOpeningSquareBracket {
				potentialNode = potentialNode.PrevSibling()
			}
			if potentialNode == nil {
				return nil, fmt.Errorf("finding the opening bracket")
			}
			// Get the node before the opening bracket
			found = GetPrevNode(potentialNode)
			//found = found.PrevSibling()
		} else {
			// myObj.
			found = potentialNode
		}
	}

	// Inside an Object the node is an error if it ends in a dot
	// if IsNode(found, NodeError) {
	//	fieldAccessNode := found.PrevSibling()
	//	if fieldAccessNode == nil {
	//		return nil, fmt.Errorf("access node is nil")
	//	}
	//	log.Errorf("Node id %v (%v)", fieldAccessNode.GrammarName(), fieldAccessNode.GrammarId())
	//	switch fieldAccessNode.GrammarName() {
	//	case NodeBind:
	//		fieldAccessNode = GetLastChild(fieldAccessNode)
	//	case NodeFunctionCall:

	//		acc, err := GetFirstChildType(fieldAccessNode, NodeFieldAccess)
	//		if err != nil {
	//			return nil, err
	//		}
	//		fieldAccessNode = GetLastChild(acc)
	//	case NodeParenthesis:
	//		// TODO: probably more cases here
	//		fieldAccessNode, err = GetFirstChildType(fieldAccessNode, NodeImport)
	//		if err != nil {
	//			return nil, err
	//		}
	//	}
	//	// fieldAccessNode, err := GetFirstChildType(sibling, NodeFieldAccess)
	//	if fieldAccessNode == nil {
	//		return nil, fmt.Errorf("no access node found")
	//	}
	//	found = GetNonSymbolNode(GetLastChild(fieldAccessNode))
	// }
	info.Node = found
	log.Errorf("Found end: %+v (%+v)", found.GrammarName(), found.StartPosition())

	return &info, nil
}

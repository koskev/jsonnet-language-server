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

	// If we are inside a function call this is the call node
	FunctionNode *tree_sitter.Node
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

var whitespace = []rune{' ', '\n', '\t'}

func getCurrentIndex(content string, pos protocol.Position) string {
	endingTokens := []rune{':', ',', ';', '(', '=', '[', '+'}

	// We want the position before the cursor
	currentPos := max(0, positionToIndex(content, pos)-1)

	index := []rune{}

	for {
		currentRune := rune(content[currentPos])
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

func getNextNonWhitespacePosition(content string, pos protocol.Position) protocol.Position {
	lines := strings.Split(content, "\n")

	for {
		if int(pos.Line) > len(lines)-1 {
			return pos
		}
		if int(pos.Character) > len(lines[pos.Line])-1 {
			return pos
		}
		currentRune := rune(lines[pos.Line][pos.Character-1])
		if !slices.Contains(whitespace, currentRune) {
			return pos
		}
		if pos.Line == 0 && pos.Character == 1 {
			return pos
		}
		if pos.Character == 1 {
			pos.Line--
			pos.Character = uint32(len(lines[pos.Line]) - 1)
		} else {
			pos.Character--
		}
	}
}

func FindCompletionNode(ctx context.Context, content string, pos protocol.Position) (*CompletionNodeInfo, error) {
	var info CompletionNodeInfo

	// search if prev token is a :,;
	currentIndex := getCurrentIndex(content, pos)
	// Replace the pos with the next non whitespace token. This way we don't need a special case for stuff like "," and ", ". E.g. for checking if we are inside a function call
	pos = getNextNonWhitespacePosition(content, pos)
	log.Tracef("current index %v from %+v", currentIndex, pos)
	// Default to local completion
	info.CompletionType = CompleteLocal

	root, err := NewTree(ctx, content)
	if err != nil {
		return nil, err
	}
	found := GetNodeAtPos(root, position.ProtocolToCST(pos))
	if found == nil {
		return nil, fmt.Errorf("unable to find tree node at %+v", pos)
	}
	log.Tracef("Found: %v (%+v)", found.GrammarName(), found.Range())

	if !strings.Contains(currentIndex, ".") {
		info.CompletionType = CompleteGlobal
		info.Index = currentIndex

		// Find if we have args parent. If true get the apply location and add it
		parent := found.Parent()
		if IsNode(parent, NodeArgs) {
			parent = parent.Parent()
		}
		if IsNode(parent, NodeFunctionCall) {
			if IsNode(parent.Child(0), NodeFieldAccess) {
				parent = parent.Child(0)
			}
			info.FunctionNode = parent
			idNode, err := GetLastChildType(parent, NodeID, false)
			if err == nil {
				info.FunctionNode = idNode
			}
		}
	}

	//nolint: gocritic
	switch found.GrammarName() {
	// In import
	case NodeStringStart:
		if found.NextSibling() != nil {
			found = found.NextSibling()
		}
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
			// a: (import "a.libsonnet")
			if IsNode(potentialNode.NextSibling(), NodeImport) {
				found = potentialNode.NextSibling()
			} else {
				// Get the node before the opening bracket
				found = GetPrevNode(potentialNode)
			}
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
	log.Tracef("Found end: %+v (%+v)", found.GrammarName(), found.StartPosition())

	return &info, nil
}

func GetParamPos(content string, pos tree_sitter.Point) (uint32, error) {
	tree, err := NewTree(context.Background(), content)
	if err != nil {
		return 0, fmt.Errorf("getting param position")
	}

	var count uint32
	node := tree.DescendantForPointRange(pos, pos)
	// If we hit the last bracket, we'll just use the previous node
	if IsNode(node, NodeClosingBracket) {
		node = GetPrevNode(node)
	}
	for node != nil && !IsNode(node, NodeOpeningBracket) {
		node = node.PrevNamedSibling()
		if node != nil && !IsSymbolNode(node) {
			count++
		}
	}
	return count, nil
}

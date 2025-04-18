package server

import (
	"cmp"
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	"github.com/grafana/jsonnet-language-server/pkg/nodetree"
	"github.com/grafana/jsonnet-language-server/pkg/utils"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	log "github.com/sirupsen/logrus"
)

// These are not too long, so we just hardcode them
var TokenToIntMap = map[protocol.SemanticTokenTypes]uint32{
	protocol.NamespaceType:     0,
	protocol.TypeType:          1,
	protocol.ClassType:         2,
	protocol.EnumType:          3,
	protocol.InterfaceType:     4,
	protocol.StructType:        5,
	protocol.TypeParameterType: 6,
	protocol.ParameterType:     7,
	protocol.VariableType:      8,
	protocol.PropertyType:      9,
	protocol.EnumMemberType:    10,
	protocol.EventType:         11,
	protocol.FunctionType:      12,
	protocol.MethodType:        13,
	protocol.MacroType:         14,
	protocol.KeywordType:       15,
	protocol.ModifierType:      16,
	protocol.CommentType:       17,
	protocol.StringType:        18,
	protocol.NumberType:        19,
	protocol.RegexpType:        20,
	protocol.OperatorType:      21,
	protocol.DecoratorType:     22,
}

var ModifierToIntMap = map[protocol.SemanticTokenModifiers]uint32{
	protocol.ModDeclaration:    0,
	protocol.ModDefinition:     1,
	protocol.ModReadonly:       2,
	protocol.ModStatic:         3,
	protocol.ModDeprecated:     4,
	protocol.ModAbstract:       5,
	protocol.ModAsync:          6,
	protocol.ModModification:   7,
	protocol.ModDocumentation:  8,
	protocol.ModDefaultLibrary: 9,
}

func (s *Server) GetSemanticTokenTypes() []string {
	tokenMap := make([]string, len(TokenToIntMap))

	for key, val := range TokenToIntMap {
		tokenMap[val] = string(key)
	}
	return tokenMap
}
func (s *Server) GetSemanticTokenModifiers() []string {
	modifierMap := make([]string, len(ModifierToIntMap))

	for key, val := range ModifierToIntMap {
		modifierMap[val] = string(key)
	}
	return modifierMap
}

type SemanticTokenMap struct {
	data []SemanticData
}

type SemanticData struct {
	line           uint32
	startChar      uint32
	length         uint32
	tokenID        uint32
	tokenModifiers uint32
}

func (m *SemanticTokenMap) asIntArray() []uint32 {
	intArray := []uint32{}
	// FUCK YOU GO!!!! for x in m.data.sorted() ?!?!?!!
	slices.SortFunc(m.data, func(a, b SemanticData) int {
		lineCmp := cmp.Compare(a.line, b.line)
		if lineCmp != 0 {
			return lineCmp
		}
		return cmp.Compare(a.startChar, b.startChar)
	})

	for i, token := range m.data {
		var prevToken SemanticData
		if i == 0 {
			prevToken = SemanticData{}
		} else {
			prevToken = m.data[i-1]
		}

		intArray = append(intArray, token.line-prevToken.line)
		if token.line != prevToken.line {
			intArray = append(intArray, token.startChar)
		} else {
			intArray = append(intArray, token.startChar-prevToken.startChar)
		}
		intArray = append(intArray, token.length)
		intArray = append(intArray, token.tokenID)
		intArray = append(intArray, token.tokenModifiers)
	}

	return intArray
}

func getModifierIDs(modifiers []protocol.SemanticTokenModifiers) (uint32, error) {
	var idMask uint32
	for _, modifier := range modifiers {
		id, ok := ModifierToIntMap[modifier]
		if !ok {
			return 0, fmt.Errorf("can't get id for %v", modifiers)
		}
		idMask |= 1 << id
	}

	return idMask, nil
}

func getID(token protocol.SemanticTokenTypes) (uint32, error) {
	id, ok := TokenToIntMap[token]
	if !ok {
		return 0, fmt.Errorf("can't get id for %s", token)
	}

	return id, nil
}

func (m *SemanticTokenMap) addNode(loc *ast.LocationRange, nodeType protocol.SemanticTokenTypes, modifiers []protocol.SemanticTokenModifiers, overrideLength int) error {
	id, err := getID(nodeType)
	if err != nil {
		return err
	}

	modifierIDs, err := getModifierIDs(modifiers)
	if err != nil {
		return err
	}

	if loc != nil && !loc.IsSet() {
		return fmt.Errorf("location is not set")
	}

	var length uint32
	// TODO: multiline
	if overrideLength > 0 {
		length = uint32(overrideLength)
	} else if loc.Begin.Line == loc.End.Line {
		length = uint32(loc.End.Column - loc.Begin.Column)
	}
	m.data = append(m.data, SemanticData{
		line:           uint32(loc.Begin.Line - 1),
		startChar:      uint32(loc.Begin.Column - 1),
		length:         length,
		tokenID:        id,
		tokenModifiers: modifierIDs,
	})

	return nil
}

func getKeywordLength(node ast.Node) uint32 {
	var length int
	switch node.(type) {
	case *ast.SuperIndex:
		length = len("super")
	case *ast.Dollar:
		length = len("$")
	case *ast.LiteralNull:
		length = len("null")
	default:
		typeString := reflect.TypeOf(node).String()
		typeString = strings.ReplaceAll(typeString, "*ast.", "")
		typeString = strings.ToLower(typeString)
		length = len(typeString)
	}

	return uint32(length)
}

func (s *Server) getTokenMap(root ast.Node) SemanticTokenMap {
	tree := nodetree.BuildTree(nil, root)
	tokenMap := SemanticTokenMap{}

	documentstack := nodestack.NodeStack{Stack: tree.GetAllChildren()}

	searchstack := documentstack.Clone()

	for !searchstack.IsEmpty() {
		node := searchstack.Pop()
		var nodeType protocol.SemanticTokenTypes
		var modifiers []protocol.SemanticTokenModifiers
		var overrideLength = -1
		nodeLocation := node.Loc()
		switch currentNode := node.(type) {
		case *ast.Self, *ast.SuperIndex, *ast.Dollar, *ast.LiteralNull:
			modifiers = append(modifiers, protocol.ModDefaultLibrary)
			nodeType = protocol.VariableType
			overrideLength = int(getKeywordLength(node))

		case *ast.Import:
			nodeType = protocol.KeywordType
			overrideLength = int(getKeywordLength(node))
		case *ast.Local:
			nodeType = protocol.KeywordType
			overrideLength = int(getKeywordLength(node))
			searchstack.Push(currentNode.Body)

		case *ast.Apply:
			// Add the args to the search stack to allow coloring function arguments
			for _, arg := range currentNode.Arguments.Named {
				searchstack.Push(arg.Arg)
			}

			for _, arg := range currentNode.Arguments.Positional {
				searchstack.Push(arg.Expr)
			}

			targetIndex, ok := currentNode.Target.(*ast.Index)
			if !ok {
				break
			}
			indexName, ok := targetIndex.Index.(*ast.LiteralString)
			if !ok {
				break
			}
			// Index spans the all before it as well. So we have to start at the end and subtract the len of the index
			nodeLocation.End = targetIndex.Loc().End
			// They are always on the same line
			nodeLocation.Begin.Line = targetIndex.Loc().End.Line
			nodeLocation.Begin.Column = max(0, nodeLocation.End.Column-len(indexName.Value))
			nodeType = protocol.FunctionType
		case *ast.Index:
			nodeType = protocol.VariableType
		case *ast.Var:
			nodeType = protocol.VariableType
			if currentNode.Id == utils.StdIdentifier {
				modifiers = append(modifiers, protocol.ModDefaultLibrary)
				break
			}

			if currentNode.Id == "$" {
				nodeType = protocol.KeywordType
				overrideLength = 1
				break
			}

			stackForNode, err := processing.FindNodeInStack(currentNode, &documentstack)
			if err != nil {
				log.Errorf("Could not find %s in documentstack: %v", currentNode.Id, err)
				continue
			}
			resolved := processing.FindNodeByIDWithOptions(stackForNode, currentNode.Id, true)
			if resolved != nil {
				switch resolved.(type) {
				case *ast.Import:
					nodeType = protocol.NamespaceType
				case *ast.Self:
					modifiers = append(modifiers, protocol.ModDefaultLibrary)
					nodeType = protocol.VariableType
				}
			} else {
				// TODO: Currently the function arguments are not on the stack. Therefore we can't find function arguments. For now we'll just assume all variables we can't find are function arguments
				nodeType = protocol.ParameterType
			}
			// TODO: if it is import -> namespace type
		case *ast.LiteralString:
			nodeType = protocol.StringType
		case *ast.LiteralNumber:
			nodeType = protocol.NumberType
		case *ast.LiteralBoolean:
			modifiers = append(modifiers, protocol.ModDefaultLibrary)
		case *ast.Conditional:
			searchstack.Push(currentNode.BranchFalse)
			searchstack.Push(currentNode.BranchTrue)
			searchstack.Push(currentNode.Cond)
		case *ast.DesugaredObject:
			for _, assert := range currentNode.Asserts {
				searchstack.Push(assert)
			}
		default:
			log.Tracef("[SEMANTIC] Unhandled node of type %T", currentNode)
		}

		if len(nodeType) > 0 && nodeLocation.IsSet() {
			err := tokenMap.addNode(nodeLocation, nodeType, modifiers, overrideLength)
			if err != nil {
				log.Errorf("Could not add %s: %v", nodeType, err)
			}
		}
	}
	return tokenMap
}

func (s *Server) SemanticTokensFull(_ context.Context, params *protocol.SemanticTokensParams) (*protocol.SemanticTokens, error) {
	tokens := protocol.SemanticTokens{
		Data: []uint32{},
	}
	if !s.configuration.EnableSemanticTokens {
		return &tokens, nil
	}

	doc, err := s.cache.Get(params.TextDocument.URI)
	if err != nil {
		log.Errorf("Could not get document: %v", err)
		return nil, utils.LogErrorf("Completion: %s: %w", errorRetrievingDocument, err)
	}

	if doc.AST == nil {
		log.Errorf("Completion: document %s was never successfully parsed, can't autocomplete", params.TextDocument.URI)
		return nil, nil
	}

	tokenMap := s.getTokenMap(doc.AST)

	tokens.Data = tokenMap.asIntArray()

	return &tokens, nil
}

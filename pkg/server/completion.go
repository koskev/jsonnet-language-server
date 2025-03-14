package server

import (
	"context"
	"reflect"
	"sort"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/formatter"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/grafana/jsonnet-language-server/pkg/utils"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	log "github.com/sirupsen/logrus"
)

func (s *Server) Completion(_ context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	doc, err := s.cache.Get(params.TextDocument.URI)
	if err != nil {
		return nil, utils.LogErrorf("Completion: %s: %w", errorRetrievingDocument, err)
	}

	line := getCompletionLine(doc.Item.Text, params.Position)

	// Short-circuit if it's a stdlib completion
	if items := s.completionStdLib(line); len(items) > 0 {
		return &protocol.CompletionList{IsIncomplete: false, Items: items}, nil
	}

	// Otherwise, parse the AST and search for completions
	if doc.AST == nil {
		log.Errorf("Completion: document was never successfully parsed, can't autocomplete")
		return nil, nil
	}

	searchStack, err := processing.FindNodeByPosition(doc.AST, position.ProtocolToAST(params.Position))
	if err != nil {
		log.Errorf("Completion: error computing node: %v", err)
		return nil, nil
	}

	vm := s.getVM(doc.Item.URI.SpanURI().Filename())

	// Inside parentheses, search for completions as if the content was on a separate line
	// e.g., this enables completion in function arguments
	if strings.LastIndex(line, "(") > strings.LastIndex(line, ")") {
		argsPart := line[strings.LastIndex(line, "(")+1:]
		// Only consider the last argument for completion
		arguments := strings.Split(argsPart, ",")
		lastArg := arguments[len(arguments)-1]
		lastArg = strings.TrimSpace(lastArg)
		lastArg = strings.TrimLeft(lastArg, "{[") // Ignore leading array or object tokens
		line = lastArg
	}

	items := s.completionFromStack(line, searchStack, vm, params.Position)
	return &protocol.CompletionList{IsIncomplete: false, Items: items}, nil
}

func getCompletionLine(fileContent string, position protocol.Position) string {
	line := strings.Split(fileContent, "\n")[position.Line]
	charIndex := int(position.Character)
	if charIndex > len(line) {
		charIndex = len(line)
	}
	line = line[:charIndex]
	return line
}

func getRightOf(input string, sep string) string {
	out := strings.Split(input, sep)

	return out[len(out)-1]
}

func (s *Server) completionFromStack(line string, stack *nodestack.NodeStack, vm *jsonnet.VM, position protocol.Position) []protocol.CompletionItem {
	lineWords := splitWords(line)
	lastWord := lineWords[len(lineWords)-1]
	lastWord = strings.TrimRight(lastWord, ",;") // Ignore trailing commas and semicolons, they can present when someone is modifying an existing line
	lastWord = strings.TrimSpace(lastWord)
	// Allow arg=<complete>
	//lastWord = getRightOf(lastWord, "=")
	//// Allow obj:<complete>
	//lastWord = getRightOf(lastWord, ":")

	indexes := strings.Split(lastWord, ".")
	log.Errorf("%s", indexes)

	if len(indexes) == 1 {
		items := []protocol.CompletionItem{}
		// firstIndex is a variable (local) completion
		for !stack.IsEmpty() {
			curr := stack.Pop()
			var binds ast.LocalBinds
			switch typedCurr := curr.(type) {
			case *ast.DesugaredObject:
				binds = typedCurr.Locals
			case *ast.Local:
				binds = typedCurr.Binds
			case *ast.Function:
				for _, param := range typedCurr.Parameters {
					items = append(items, createCompletionItem(string(param.Name), "", protocol.VariableCompletion, &ast.Var{}, position))
				}
			default:
				continue
			}
			for _, bind := range binds {
				label := string(bind.Variable)
				if strings.HasPrefix(label, indexes[0]) && label != "$" {
					items = append(items, createCompletionItem(label, "", protocol.VariableCompletion, bind.Body, position))
				}
			}
		}
		return items
	}

	processor := processing.NewProcessor(s.cache, vm)
	ranges, err := processor.FindRangesFromIndexList(stack, indexes, true)
	if err != nil {
		log.Errorf("Completion: error finding ranges: %v", err)
		return []protocol.CompletionItem{}
	}

	completionPrefix := strings.Join(indexes[:len(indexes)-1], ".")
	return s.createCompletionItemsFromRanges(ranges, completionPrefix, line, position)
}

func (s *Server) completionStdLib(line string) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}

	stdIndex := strings.LastIndex(line, "std.")
	if stdIndex != -1 {
		userInput := line[stdIndex+4:]
		funcStartWith := []protocol.CompletionItem{}
		funcContains := []protocol.CompletionItem{}
		for _, f := range s.stdlib {
			if f.Name == userInput {
				break
			}
			lowerFuncName := strings.ToLower(f.Name)
			findName := strings.ToLower(userInput)
			item := protocol.CompletionItem{
				Label:         f.Name,
				Kind:          protocol.FunctionCompletion,
				Detail:        f.Signature(),
				InsertText:    strings.ReplaceAll(f.Signature(), "std.", ""),
				Documentation: f.MarkdownDescription,
			}

			if len(findName) > 0 && strings.HasPrefix(lowerFuncName, findName) {
				funcStartWith = append(funcStartWith, item)
				continue
			}

			if strings.Contains(lowerFuncName, findName) {
				funcContains = append(funcContains, item)
			}
		}

		items = append(items, funcStartWith...)
		items = append(items, funcContains...)
	}

	return items
}

func (s *Server) createCompletionItemsFromRanges(ranges []processing.ObjectRange, completionPrefix, currentLine string, position protocol.Position) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}
	labels := make(map[string]bool)

	for _, field := range ranges {
		label := field.FieldName

		if field.Node == nil {
			continue
		}

		if labels[label] {
			continue
		}

		if !s.configuration.ShowDocstringInCompletion && strings.HasPrefix(label, "#") {
			continue
		}

		// Ignore the current field
		if strings.Contains(currentLine, label+":") && completionPrefix == "self" {
			continue
		}

		items = append(items, createCompletionItem(label, completionPrefix, protocol.FieldCompletion, field.Node, position))
		labels[label] = true
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Label < items[j].Label
	})

	return items
}

func formatLabel(str string) string {
	interStr := "interimPath" + str
	fmtStr, _ := formatter.Format("", interStr, formatter.DefaultOptions())
	ret, _ := strings.CutPrefix(fmtStr, "interimPath")
	ret, _ = strings.CutPrefix(ret, ".")
	ret = strings.TrimRight(ret, "\n")
	return ret
}

func createCompletionItem(label, prefix string, kind protocol.CompletionItemKind, body ast.Node, position protocol.Position) protocol.CompletionItem {
	paramsString := ""
	if asFunc, ok := body.(*ast.Function); ok {
		kind = protocol.FunctionCompletion
		params := []string{}
		for _, param := range asFunc.Parameters {
			params = append(params, string(param.Name))
		}
		paramsString = "(" + strings.Join(params, ", ") + ")"
	}

	insertText := formatLabel("['" + label + "']" + paramsString)

	concat := ""
	characterStartPosition := position.Character - 1
	if prefix == "" {
		characterStartPosition = position.Character
	}
	if prefix != "" && !strings.HasPrefix(insertText, "[") {
		concat = "."
		characterStartPosition = position.Character
	}
	detail := prefix + concat + insertText

	item := protocol.CompletionItem{
		Label:  label,
		Detail: detail,
		Kind:   kind,
		LabelDetails: protocol.CompletionItemLabelDetails{
			Description: typeToString(body),
		},
		InsertText: insertText,
	}

	if strings.HasPrefix(insertText, "[") {
		item.TextEdit = &protocol.TextEdit{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      position.Line,
					Character: characterStartPosition,
				},
				End: protocol.Position{
					Line:      position.Line,
					Character: position.Character,
				},
			},
			NewText: insertText,
		}
	}

	return item
}

func typeToString(t ast.Node) string {
	switch t.(type) {
	case *ast.Array:
		return "array"
	case *ast.LiteralBoolean:
		return "boolean"
	case *ast.Function:
		return "function"
	case *ast.LiteralNull:
		return "null"
	case *ast.LiteralNumber:
		return "number"
	case *ast.Object, *ast.DesugaredObject:
		return "object"
	case *ast.LiteralString:
		return "string"
	case *ast.Import, *ast.ImportStr:
		return "import"
	case *ast.Index:
		return "object field"
	case *ast.Var:
		return "variable"
	}
	return reflect.TypeOf(t).String()
}

// splitWords splits the input string into words, respecting spaces within parentheses.
func splitWords(input string) []string {
	var words []string
	var currentWord strings.Builder
	var insideParentheses bool

	for _, char := range input {
		switch char {
		case ' ':
			if insideParentheses {
				currentWord.WriteRune(char)
			} else if currentWord.Len() > 0 {
				words = append(words, currentWord.String())
				currentWord.Reset()
			}
		case '=':
			currentWord.Reset()
		case ':':
			currentWord.Reset()
		case '(':
			insideParentheses = true
			currentWord.WriteRune(char)
		case ')':
			currentWord.WriteRune(char)
			insideParentheses = false
		default:
			currentWord.WriteRune(char)
		}
	}

	if currentWord.Len() > 0 {
		words = append(words, currentWord.String())
	} else if strings.HasSuffix(input, " ") {
		words = append(words, "")
	}

	if len(words) == 0 {
		words = append(words, "")
	}

	return words
}

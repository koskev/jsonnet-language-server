package completion

import (
	"bytes"
	"reflect"
	"slices"
	"text/template"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/grafana/jsonnet-language-server/pkg/stdlib"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	log "github.com/sirupsen/logrus"
)

type StdSingleParamSnippet struct {
	customTemplate string
	stdFunctions   *map[string]stdlib.Function
}

func (s *StdSingleParamSnippet) applyTemplate(node ast.Node, callstack *nodestack.NodeStack, content string) []protocol.CompletionItem {
	var items []protocol.CompletionItem
	firstNode := callstack.Peek()
	lastNode := callstack.PeekFront()

	beginTextPos := LocationToIndex(firstNode.Loc().Begin, content)
	endTextPos := LocationToIndex(lastNode.Loc().End, content)
	callText := content[beginTextPos:endTextPos]

	pos := position.ASTToProtocol(lastNode.Loc().End)
	// Add a character due to the .
	pos.Character++

	if s.stdFunctions == nil {
		log.Errorf("Cannot create std snippet without stdlib info")
		return []protocol.CompletionItem{}
	}

	for _, stdFunction := range *s.stdFunctions {
		if len(stdFunction.Params) > 1 {
			continue
		}
		if len(stdFunction.TypeLimitations) > 0 && !slices.Contains(stdFunction.TypeLimitations, reflect.TypeOf(node)) {
			continue
		}

		if len(s.customTemplate) == 0 {
			s.customTemplate = "std.{{.FunctionName}}({{.CallText}})"
		}

		templateParser, err := template.New("").Parse(s.customTemplate)
		if err != nil {
			log.Errorf("Unable to create template: %v", err)
			return []protocol.CompletionItem{}
		}

		var buf bytes.Buffer
		err = templateParser.Execute(&buf, struct {
			CallText     string
			FunctionName string
		}{
			CallText:     callText,
			FunctionName: stdFunction.Name,
		})
		if err != nil {
			log.Errorf("Unable to execute template: %v", err)
			return []protocol.CompletionItem{}
		}

		items = append(items, protocol.CompletionItem{
			Label:            stdFunction.Name,
			Detail:           stdFunction.MarkdownDescription,
			InsertTextFormat: protocol.SnippetTextFormat,
			TextEdit: &protocol.TextEdit{
				NewText: buf.String(),
				Range: protocol.Range{
					Start: pos,
					End:   pos,
				},
			},
			// Remove the old text
			AdditionalTextEdits: []protocol.TextEdit{
				{
					NewText: "",
					Range: protocol.Range{
						Start: position.ASTToProtocol(firstNode.Loc().Begin),
						End:   pos,
					},
				},
			},
			Kind: protocol.SnippetCompletion,
		})
	}
	return items
}

func (c *Completion) CreateSnippets(searchstack *nodestack.NodeStack, node ast.Node, content string) []protocol.CompletionItem {
	callstack := BuildCallStack(searchstack)

	stdSnippets := StdSingleParamSnippet{
		stdFunctions: c.stdLib,
	}

	return stdSnippets.applyTemplate(node, callstack, content)
}

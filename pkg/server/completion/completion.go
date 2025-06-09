package completion

import (
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/server/config"
	"github.com/grafana/jsonnet-language-server/pkg/stdlib"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

type Completion struct {
	stdLib *map[string]stdlib.Function
	Config config.CompletionConfig
}

func NewCompletion(stdLib *map[string]stdlib.Function) *Completion {
	return &Completion{
		stdLib: stdLib,
	}
}

//nolint:unparam // Currently prefix is always called with ""
func (c *Completion) CreateCompletionItem(label, prefix string, kind protocol.CompletionItemKind, body ast.Node, position protocol.Position, tryEscape bool) protocol.CompletionItem {
	paramsString := ""
	if asFunc, ok := body.(*ast.Function); ok {
		kind = protocol.FunctionCompletion
		params := []string{}
		for _, param := range asFunc.Parameters {
			params = append(params, string(param.Name))
		}
		paramsString = "(" + strings.Join(params, ", ") + ")"
	}

	var insertText string
	if tryEscape {
		insertText = FormatLabel("['" + label + "']" + paramsString)
	} else {
		insertText = label
	}

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

	if c.Config.UseTypeInDetail {
		detail = TypeToString(body)
	}

	item := protocol.CompletionItem{
		Label:  label,
		Detail: detail,
		Kind:   kind,
		LabelDetails: &protocol.CompletionItemLabelDetails{
			Description: TypeToString(body),
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

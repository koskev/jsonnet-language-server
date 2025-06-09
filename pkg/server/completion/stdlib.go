package completion

import (
	"strings"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

func (c *Completion) CompletionStdLib(line string) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}

	stdIndex := strings.LastIndex(line, "std.")
	if stdIndex != -1 {
		userInput := line[stdIndex+4:]
		funcStartWith := []protocol.CompletionItem{}
		funcContains := []protocol.CompletionItem{}
		for _, f := range *c.stdLib {
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

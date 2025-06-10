package completion

import (
	"reflect"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

func (c *Completion) CompleteKeywords(stack *nodestack.NodeStack, pos protocol.Position) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}
	addSelf := false
	addSuper := false
	// TODO: determine when we can add a local
	addLocal := true

	for !stack.IsEmpty() {
		curr := stack.Pop()
		switch curr.(type) {
		case *ast.DesugaredObject:
			addSelf = true
			parentNode, _, err := stack.FindNext(reflect.TypeFor[*ast.Binary]())
			if err != nil {
				break
			}
			//nolint:forcetypeassert // go stuff
			parentBinary := parentNode.(*ast.Binary)
			addSuper = parentBinary.Right == curr
		default:
			break
		}
	}
	if addSelf {
		items = append(items, c.CreateCompletionItem("self", "", protocol.VariableCompletion, &ast.Self{}, pos, false))
	}
	if addSuper {
		items = append(items, c.CreateCompletionItem("super", "", protocol.VariableCompletion, &ast.SuperIndex{}, pos, false))
	}
	if addLocal {
		items = append(items, c.CreateCompletionItem("local", "", protocol.VariableCompletion, &ast.Local{}, pos, false))
	}

	return items
}

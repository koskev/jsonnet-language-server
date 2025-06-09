package server

import (
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

func (s *Server) createArraySnippets(searchstack *nodestack.NodeStack, cursorPos protocol.Position) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}
	if s.configuration.Completion.EnableArraySnippets {
		callstack := s.buildCallStack(searchstack)
		callstack.PrintStack()
		items = append(items, s.createCompletionItemSurround("length", "std.length(", ")", protocol.SnippetCompletion, callstack, cursorPos))
	}
	return items
}

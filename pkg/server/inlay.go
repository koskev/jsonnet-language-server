package server

import (
	"context"
	"fmt"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/nodetree"
	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

func (s *Server) InlayHint(_ context.Context, params *protocol.InlayHintParams) ([]protocol.InlayHint, error) {
	var inlayHints []protocol.InlayHint

	doc, err := s.cache.Get(params.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("getting document from cache: %w", err)
	}
	if doc.AST == nil {
		return nil, fmt.Errorf("document was never parsed")
	}

	// Generate complete AST for range
	// Iterate over all nodes
	// Apply inlay hint to all arguments
	// TODO: special case for stdlib
	tree := nodetree.BuildTree(nil, doc.AST)

	for _, node := range tree.GetAllChildren() {
		// nolint: gocritic
		switch currentNode := node.(type) {
		case *ast.Apply:
			// Get target func
			functionNode, err := s.getFunctionCallTarget(doc.AST, currentNode, params.TextDocument.URI)
			if err != nil {
				continue
			}
			var names []string
			for _, param := range functionNode.Parameters {
				names = append(names, string(param.Name))
			}

			for i, applyParam := range currentNode.Arguments.Positional {
				pos := position.ASTToProtocol(applyParam.Expr.Loc().Begin)
				inlayHints = append(inlayHints, protocol.InlayHint{
					Position:     &pos,
					PaddingRight: true,
					Label: []protocol.InlayHintLabelPart{
						{
							Value: names[i],
						},
					},
				})
			}
		}
	}

	return inlayHints, nil
}

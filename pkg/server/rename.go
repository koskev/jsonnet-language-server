package server

import (
	"context"
	"fmt"

	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

func (s *Server) Rename(_ context.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	positions, identifier, err := s.findAllReferences(params.TextDocument.URI, params.Position)
	if err != nil {
		return nil, err
	}
	sourcePos, err := s.findIdentifierLocations(params.TextDocument.URI.SpanURI().Filename(), identifier)
	if err != nil {
		return nil, fmt.Errorf("getting rename source pos")
	}
	for _, pos := range sourcePos {
		if pointInRange(position.ProtocolToAST(params.Position), pos.Begin, pos.End) {
			r := protocol.Location{
				Range: position.RangeASTToProtocol(pos),
				URI:   params.TextDocument.URI,
			}
			positions = append(positions, r)
		}
	}

	var response protocol.WorkspaceEdit
	response.Changes = map[string][]protocol.TextEdit{}

	for _, pos := range positions {
		edits := response.Changes[string(pos.URI)]
		edits = append(edits, protocol.TextEdit{
			Range:   pos.Range,
			NewText: params.NewName,
		})
		response.Changes[string(pos.URI)] = edits
	}

	return &response, nil
}

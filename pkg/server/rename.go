package server

import (
	"context"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

func (s *Server) Rename(_ context.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	positions, err := s.findAllReferences(params.TextDocument.URI, params.Position, true)
	if err != nil {
		return nil, err
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

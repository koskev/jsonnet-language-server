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
	edits := map[string][]protocol.TextEdit{}

	for _, pos := range positions {
		localEdits := edits[string(pos.URI)]
		localEdits = append(localEdits, protocol.TextEdit{
			Range:   pos.Range,
			NewText: params.NewName,
		})
		edits[string(pos.URI)] = localEdits
	}

	if s.clientCapabilities.Workspace.WorkspaceEdit.DocumentChanges {
		for fileName, edit := range edits {
			doc, err := s.cache.Get(protocol.DocumentURI(fileName))
			version := int32(0)
			if err == nil {
				version = doc.Item.Version
			}
			response.DocumentChanges = append(response.DocumentChanges, protocol.TextDocumentEdit{
				Edits: edit,
				TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{
						URI: protocol.DocumentURI(fileName),
					},
					Version: version,
				},
			})
		}
	} else {
		response.Changes = edits
	}

	return &response, nil
}

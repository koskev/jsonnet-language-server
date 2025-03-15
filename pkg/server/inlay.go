package server

import (
	"context"
	"fmt"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

func (s *Server) InlayHint(context.Context, *protocol.InlayHintParams) ([]protocol.InlayHint, error) {

	return nil, fmt.Errorf("Not implemented")

}

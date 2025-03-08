package position

import (
	"testing"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/stretchr/testify/assert"
)

func TestConversion(t *testing.T) {
	pos := protocol.Position{
		Line:      0,
		Character: 0,
	}
	assert.Equal(t, pos, ASTToProtocol(ProtocolToAST(pos)))

}

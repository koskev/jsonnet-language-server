package position

import (
	"github.com/google/go-jsonnet/ast"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

func ProtocolToAST(point protocol.Position) ast.Location {
	return ast.Location{
		Line:   int(point.Line) + 1,
		Column: int(point.Character) + 1,
	}
}

func ASTToProtocol(location ast.Location) protocol.Position {
	return protocol.Position{
		Line:      uint32(location.Line) - 1,
		Character: uint32(location.Column) - 1,
	}
}

func ProtocolToCST(point protocol.Position) sitter.Point {
	return sitter.Point{
		Row:    uint(point.Line),
		Column: uint(point.Character),
	}
}

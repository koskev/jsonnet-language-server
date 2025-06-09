package utils

import (
	"reflect"
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

func CompareSelf(selfNode ast.Node, other ast.Node) bool {
	selfType := reflect.TypeFor[*ast.Self]()
	return reflect.TypeOf(selfNode) == selfType && reflect.TypeOf(other) == selfType && selfNode.Context() == other.Context()
}

func GetCompletionLine(fileContent string, position protocol.Position) string {
	line := strings.Split(fileContent, "\n")[position.Line]
	charIndex := int(position.Character)
	charIndex = min(charIndex, len(line))
	line = line[:charIndex]
	return line
}

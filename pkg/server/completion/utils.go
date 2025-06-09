package completion

import (
	"reflect"
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/formatter"
)

func FormatLabel(str string) string {
	interStr := "interimPath" + str
	fmtStr, _ := formatter.Format("", interStr, formatter.DefaultOptions())
	ret, _ := strings.CutPrefix(fmtStr, "interimPath")
	ret, _ = strings.CutPrefix(ret, ".")
	ret = strings.TrimRight(ret, "\n")
	return ret
}

func LocationToIndex(pos ast.Location, text string) int {
	idx := 0
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i+1 >= pos.Line {
			idx += pos.Column - 1
			break
		}
		idx += len(line) + 1 // Append the \n again
	}
	return idx
}

func TypeToString(t ast.Node) string {
	switch t.(type) {
	case *ast.Array:
		return "array"
	case *ast.LiteralBoolean:
		return "boolean"
	case *ast.Function:
		return "function"
	case *ast.LiteralNull:
		return "null"
	case *ast.LiteralNumber:
		return "number"
	case *ast.Object, *ast.DesugaredObject:
		return "object"
	case *ast.LiteralString:
		return "string"
	case *ast.Import, *ast.ImportStr:
		return "import"
	case *ast.Index:
		return "object field"
	case *ast.Var:
		return "variable"
	case *ast.SuperIndex:
		return "super"
	}
	typeString := reflect.TypeOf(t).String()
	typeString = strings.ReplaceAll(typeString, "*ast.", "")
	typeString = strings.ToLower(typeString)

	return typeString
}

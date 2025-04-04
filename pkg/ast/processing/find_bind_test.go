package processing

import (
	"testing"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	"github.com/stretchr/testify/assert"
)

func TestFindBind(t *testing.T) {
	testBody := ast.LiteralString{Value: "testval"}
	var testCases = []struct {
		name         string
		nodes        []ast.Node
		ident        string
		expectedNode ast.Node
	}{
		{
			name: "nil",
		},
		{
			name:  "nil nodes",
			nodes: []ast.Node{nil, nil, nil},
		},
		{
			name:  "invalid nodes",
			nodes: []ast.Node{&ast.DesugaredObject{}, &ast.DesugaredObject{Locals: nil}, nil},
		},
		{
			name: "find simple object local",
			nodes: []ast.Node{
				&ast.DesugaredObject{
					Locals: ast.LocalBinds{
						{Variable: "test", Body: &testBody},
					},
				},
			},
			ident:        "test",
			expectedNode: &testBody,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stack := nodestack.NodeStack{}
			stack.Stack = tc.nodes
			foundNode := FindNodeByID(&stack, ast.Identifier(tc.ident))
			assert.Equal(t, tc.expectedNode, foundNode)
		})
	}
}

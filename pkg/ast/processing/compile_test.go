package processing

import (
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	"github.com/grafana/jsonnet-language-server/pkg/nodetree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCase = struct {
	name           string
	documentstack  *nodestack.NodeStack
	compileNode    ast.Node
	expectedResult string
}

func TestCompile(t *testing.T) {
	testCases := []testCase{}

	node := &ast.Local{
		Body: &ast.Var{
			Id: "myVar",
		},
		Binds: ast.LocalBinds{
			{
				Body: &ast.LiteralString{
					Value: "myValue",
				},
				Variable: "myVar",
			},
		},
	}
	tree := nodetree.BuildTree(nil, node)
	stack := &nodestack.NodeStack{Stack: tree.GetAllChildren()}
	testCases = append(testCases, testCase{
		name:           "single var",
		documentstack:  stack,
		compileNode:    node.Body,
		expectedResult: "\"myValue\"\n",
	})

	// TODO: Understand how nested vars are saved in the AST
	node = &ast.Local{
		Body: &ast.Local{
			Body: &ast.Var{
				Id: "myVar",
			},
			Binds: ast.LocalBinds{
				{
					Body: &ast.LiteralString{
						Value: "myValue",
					},
					Variable: "myVar",
				},
			},
		},
		Binds: ast.LocalBinds{
			{
				Body: &ast.LiteralString{
					Value: "myValue",
				},
				Variable: "outerVar",
			},
		},
	}
	tree = nodetree.BuildTree(nil, node)
	stack = &nodestack.NodeStack{Stack: tree.GetAllChildren()}
	testCases = append(testCases, testCase{
		name:           "multi var",
		documentstack:  stack,
		compileNode:    node,
		expectedResult: "\"myValue\"\n",
	})

	node = &ast.Local{
		Body: &ast.Local{
			Body: &ast.Var{
				Id: "outerVar",
			},
			Binds: ast.LocalBinds{
				{
					Body: &ast.LiteralString{
						Value: "myValue",
					},
					Variable: "myVar",
				},
			},
		},
		Binds: ast.LocalBinds{
			{
				Body: &ast.LiteralString{
					Value: "myValue",
				},
				Variable: "outerVar",
			},
		},
	}
	tree = nodetree.BuildTree(nil, node)
	stack = &nodestack.NodeStack{Stack: tree.GetAllChildren()}
	testCases = append(testCases, testCase{
		name:           "multi var",
		documentstack:  stack,
		compileNode:    node,
		expectedResult: "\"myValue\"\n",
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vm := jsonnet.MakeVM()
			node, err := CompileNodeFromStack(tc.compileNode, tc.documentstack, vm)
			require.NoError(t, err)

			result, err := vm.Evaluate(node)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

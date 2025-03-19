package processing

import (
	"reflect"
	"testing"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/cache"
	"github.com/grafana/jsonnet-language-server/pkg/nodetree"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompile(t *testing.T) {
	vm := jsonnet.MakeVM()

	processor := NewProcessor(cache.New(), vm)
	root, _, err := processor.vm.ImportAST("", "./testdata/varcompile.jsonnet")

	require.NoError(t, err)

	tree := nodetree.BuildTree(nil, root)
	logrus.Errorf("\n%s", tree)

	assert.Len(t, tree.Children, 2)
	tree = tree.Children[0]
	assert.Len(t, tree.Children, 2)
	tree = tree.Children[0]
	assert.Len(t, tree.Children, 2)
	tree = tree.Children[0]
	assert.Len(t, tree.Children, 3)

	funcNode := tree.Children[1].Node
	assert.Equal(t, reflect.TypeFor[*ast.Apply](), reflect.TypeOf(funcNode))
	_, err = processor.CompileNode(root, funcNode)
	require.NoError(t, err)
}

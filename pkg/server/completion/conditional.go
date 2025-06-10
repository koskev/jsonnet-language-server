package completion

import (
	"fmt"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
)

func (c *Completion) ResolveConditional(node *ast.Conditional, documentstack *nodestack.NodeStack) (ast.Node, error) {
	filename := documentstack.GetNextFilename()
	vm := c.GetVMCallback(filename)
	compiled, err := processing.CompileNodeFromStack(node.Cond, documentstack, vm)
	if err != nil {
		return nil, err
	}
	result, ok := compiled.(*ast.LiteralBoolean)
	if !ok {
		return nil, fmt.Errorf("node did not compile to literal boolean. Got %T", compiled)
	}
	if result.Value {
		return node.BranchTrue, nil
	}
	return node.BranchFalse, nil
}

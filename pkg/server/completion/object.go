package completion

import (
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	log "github.com/sirupsen/logrus"
)

func (c *Completion) EvaluateObjectFields(node *ast.DesugaredObject, documentstack *nodestack.NodeStack) *ast.DesugaredObject {
	// TODO: node.clone()
	vm := c.GetVMCallback(node.Loc().FileName)
	for i, field := range node.Fields {
		resolved := desugaredObjectKeyToString(field.Name, documentstack, vm)
		if resolved != nil {
			node.Fields[i].Name = resolved
		}
	}
	return node
}

func desugaredObjectKeyToString(node ast.Node, documentstack *nodestack.NodeStack, vm *jsonnet.VM) *ast.LiteralString {
	switch currentNode := node.(type) {
	case *ast.LiteralString:
		return currentNode
	case *ast.Conditional:
		compiled, err := processing.CompileNodeFromStack(currentNode.Cond, documentstack, vm)
		if err != nil {
			log.Errorf("Failed to compile node %v", err)
			return nil
		}
		result, ok := compiled.(*ast.LiteralBoolean)
		if !ok {
			log.Errorf("Result is not boolean but %T", compiled)
			return nil
		}
		if result.Value {
			return desugaredObjectKeyToString(currentNode.BranchTrue, documentstack, vm)
		}
		return desugaredObjectKeyToString(currentNode.BranchFalse, documentstack, vm)
	}

	return nil
}

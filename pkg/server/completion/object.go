package completion

import (
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	log "github.com/sirupsen/logrus"
)

func (c *Completion) EvaluateObjectFields(node *ast.DesugaredObject, documentstack *nodestack.NodeStack) *ast.DesugaredObject {
	// TODO: node.clone()
	for i, field := range node.Fields {
		resolved := c.desugaredObjectKeyToString(field.Name, documentstack)
		if resolved != nil {
			node.Fields[i].Name = resolved
		}
	}
	return node
}

func (c *Completion) desugaredObjectKeyToString(node ast.Node, documentstack *nodestack.NodeStack) *ast.LiteralString {
	// handle conditional
	switch currentNode := node.(type) {
	case *ast.LiteralString:
		return currentNode
	case *ast.Conditional:
		vm := c.GetVMCallback(node.Loc().FileName)
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
			return c.desugaredObjectKeyToString(currentNode.BranchTrue, documentstack)
		}
		return c.desugaredObjectKeyToString(currentNode.BranchFalse, documentstack)
	}

	return nil
}

package processing

import (
	"fmt"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	"github.com/grafana/jsonnet-language-server/pkg/nodetree"
	"github.com/sirupsen/logrus"
)

func resolveVar(node *ast.Var, documentstack *nodestack.NodeStack) (ast.Node, error) {
	foundNode := FindNodeByID(documentstack, node.Id)
	if foundNode == nil {
		return nil, fmt.Errorf("finding node in stack")
	}
	if varNode, ok := foundNode.(*ast.Var); ok {
		return resolveVar(varNode, documentstack)
	}
	return foundNode, nil
}

func CompileNodeFromStack(node ast.Node, documentstack *nodestack.NodeStack, vm *jsonnet.VM) (ast.Node, error) {
	tree := nodetree.BuildTree(nil, node)
	logrus.Errorf("TREE: %s", tree)

	compileNode := node

	stack := nodestack.NewNodeStack(node)
	for _, currentNode := range tree.GetAllChildren() {
		stack.Push(currentNode)
	}

	for !stack.IsEmpty() {
		currentNode := stack.Pop()
		switch currentNode := currentNode.(type) {
		case *ast.Var:
			// Recursively resolve the var. If we just add the var as the body, compile can't find the var
			varNode, err := resolveVar(currentNode, documentstack)
			if err != nil {
				logrus.Errorf("Failed to resolve var while compiling: %v", err)
				continue
			}
			stack.Push(varNode)
			compileNode = &ast.Local{
				Body: compileNode,
				Binds: ast.LocalBinds{
					{
						Variable: currentNode.Id,
						Body:     varNode,
					},
				},
			}
		default:
		}
	}

	logrus.Errorf("Evaluating node %T", compileNode)
	evalResult, err := vm.Evaluate(compileNode)
	if err != nil {
		return nil, fmt.Errorf("could not evaluate node: %w", err)
	}
	logrus.Errorf("######### Result: %s\n", evalResult)

	newNode, err := jsonnet.SnippetToAST("", evalResult)
	if err != nil {
		return nil, fmt.Errorf("could not compile snippet %s to ast: %w", evalResult, err)
	}
	return newNode, nil
}

func (p *Processor) CompileNode(root ast.Node, node ast.Node) (ast.Node, error) {
	// get node with stack
	stack, err := FindNodeByPosition(root, node.Loc().Begin)
	if err != nil {
		return nil, err
	}
	return CompileNodeFromStack(node, stack, p.vm)
}

func (p *Processor) CompileString(data string) (ast.Node, error) {
	return jsonnet.SnippetToAST("", data)
}

package processing

import (
	"fmt"
	"reflect"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	"github.com/sirupsen/logrus"
)

func CompileNodeFromStack(node ast.Node, documentstack *nodestack.NodeStack, vm *jsonnet.VM) (ast.Node, error) {
	stack := documentstack.Clone()
	for !stack.IsEmpty() {
		logrus.Errorf("Looking at %v", reflect.TypeOf(stack.Peek()))
		if localNode, ok := stack.Pop().(*ast.Local); ok {
			localNode.Body = node
			break
		}
	}

	logrus.Errorf("Evaluating node")
	evalResult, err := vm.Evaluate(documentstack.PeekFront())
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

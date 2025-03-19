package processing

import (
	"fmt"
	"reflect"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/nodetree"
	"github.com/sirupsen/logrus"
)

func (p *Processor) CompileNode(root ast.Node, node ast.Node) (ast.Node, error) {
	t := nodetree.BuildTree(nil, root)
	logrus.Errorf("PRE\n%s", t)

	switch currentNode := node.(type) {
	case *ast.Apply:
		// get node with stack
		stack, err := FindNodeByPosition(root, currentNode.Loc().Begin)
		if err != nil {
			return nil, err
		}
		for !stack.IsEmpty() {
			if localNode, ok := stack.Pop().(*ast.Local); ok {
				localNode.Body = currentNode
				break
			}
		}

	default:
		logrus.Errorf("Not handling %v", reflect.TypeOf(currentNode))
		return node, nil
	}

	t = nodetree.BuildTree(nil, root)
	logrus.Errorf("POST\n%s", t)
	evalResult, err := p.vm.Evaluate(root)
	if err != nil {
		return nil, fmt.Errorf("could not evaluate node: %w", err)
	}
	logrus.Errorf("Result: %s\n", evalResult)

	newNode, err := jsonnet.SnippetToAST("", evalResult)
	if err != nil {
		return nil, fmt.Errorf("could not compile snippet %s to ast: %w", evalResult, err)
	}
	return newNode, nil
}

func (p *Processor) CompileString(data string) (ast.Node, error) {
	return jsonnet.SnippetToAST("", data)
}

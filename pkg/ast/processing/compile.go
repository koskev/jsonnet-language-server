package processing

import (
	"fmt"
	"reflect"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/sirupsen/logrus"
)

func (p *Processor) CompileNode(root ast.Node, node ast.Node) (ast.Node, error) {
	//t := nodetree.BuildTree(nil, root)
	//logrus.Errorf("PRE\n%s", t)

	// get node with stack
	stack, err := FindNodeByPosition(root, node.Loc().Begin)
	if err != nil {
		return nil, err
	}
	for !stack.IsEmpty() {
		logrus.Errorf("Looking at %v", reflect.TypeOf(stack.Peek()))
		if localNode, ok := stack.Pop().(*ast.Local); ok {
			localNode.Body = node
			break
		}
	}

	//t = nodetree.BuildTree(nil, root)
	//logrus.Errorf("POST\n%s", t)
	logrus.Errorf("Evaluating node")
	evalResult, err := p.vm.Evaluate(root)
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

func (p *Processor) CompileString(data string) (ast.Node, error) {
	return jsonnet.SnippetToAST("", data)
}

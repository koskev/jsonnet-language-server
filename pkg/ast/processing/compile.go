package processing

import (
	"fmt"
	"reflect"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/nodetree"
	"github.com/sirupsen/logrus"
)

func (p *Processor) CompileNode(node ast.Node) (ast.Node, error) {
	t := nodetree.BuildTree(nil, node)
	logrus.Errorf("PRE\n%s", t)

	switch currentNode := node.(type) {
	case *ast.Var:
		varReference, err := p.FindVarReference(currentNode)
		if err != nil {
			return nil, err
		}
		return p.CompileNode(varReference)

	case *ast.Apply:
		target, err := p.CompileNode(currentNode.Target)
		if err != nil {
			return nil, err
		}
		currentNode.Target = target
	case *ast.Function:
		compiledBody, err := p.CompileNode(currentNode.Body)
		currentNode.Body = compiledBody
		return currentNode, err

	default:
		logrus.Errorf("Not handling %v", reflect.TypeOf(currentNode))
		return node, nil
	}

	t = nodetree.BuildTree(nil, node)
	logrus.Errorf("POST\n%s", t)
	evalResult, err := p.vm.Evaluate(node)
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

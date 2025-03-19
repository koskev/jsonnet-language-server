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
	t := nodetree.BuildTree(nil, node)
	logrus.Errorf("###%s", t)

	switch currentNode := node.(type) {
	case *ast.Apply:
		logrus.Errorf("###### APPLY %+v target %v", node, reflect.TypeOf(currentNode.Target))
		if target, ok := currentNode.Target.(*ast.Var); ok {
			varReference, err := p.FindVarReference(target)
			if err != nil {
				return nil, err
			}
			node = varReference
			//currentNode.Target = varReference
			logrus.Errorf("Replaced target with %+v %v", varReference, reflect.TypeOf(varReference))
		}
		if n, ok := node.(*ast.Function); ok {
			if v, ok := n.Body.(*ast.Var); ok {
				varReference, err := p.FindVarReference(v)
				if err != nil {
					return nil, err
				}
				node = varReference
				logrus.Errorf("####### %+v %v", varReference, reflect.TypeOf(varReference))
			}
		}
	}

	//for _, child := range t.GetAllChildren() {
	//	switch child := child.(type) {
	//	case *ast.Var:
	//		varReference, err := p.FindVarReference(child)
	//		if err != nil {
	//			return nil, err
	//		}
	//		logrus.Errorf("### Ref %v of type %v", varReference, reflect.TypeOf(varReference))
	//	}
	//	logrus.Errorf("Child %v of type %v", child, reflect.TypeOf(child))
	//}

	t = nodetree.BuildTree(nil, node)
	logrus.Errorf("%s", t)
	// TODO: find all dependencies
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

package completion

import (
	"reflect"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	log "github.com/sirupsen/logrus"
)

func BuildCallStack(documentstack *nodestack.NodeStack) *nodestack.NodeStack {
	node := documentstack.Pop()
	nodesToSearch := nodestack.NewNodeStack(node)
	callStack := &nodestack.NodeStack{}
	log.Tracef("Building call stack from %v", reflect.TypeOf(node))

	for !nodesToSearch.IsEmpty() {
		currentNode := nodesToSearch.Pop()
		log.Tracef("CALL BUILD %v", reflect.TypeOf(currentNode))
		switch currentNode := currentNode.(type) {
		case *ast.Index:
			log.Tracef("INDEX TARGET %v", reflect.TypeOf(currentNode.Target))
			callStack.Push(currentNode)
			nodesToSearch.Push(currentNode.Target)
			log.Tracef("New target %v %v", reflect.TypeOf(currentNode.Target), currentNode.Index)
		case *ast.Var:
			// TODO: somehow figure out function index stuff
			if _, ok := callStack.Peek().(*ast.Apply); !ok {
				callStack.Push(currentNode)
			}
			// Inside a function call the stack also contains the function. If we see a var we can abort as a var always marks the end of a "call"

			// Special case: if we have an array the next node in the documentstack is an index
			varNode, err := processing.ResolveVar(currentNode, documentstack)
			if err != nil {
				log.Errorf("could not resolve var while building stack: %v", err)
				continue
			}
			if indexNode, ok := documentstack.Peek().(*ast.Index); ok && indexNode != nil && reflect.TypeOf(varNode) == reflect.TypeFor[*ast.Array]() {
				callStack.PushFront(indexNode)
			}
		case *ast.Apply:
			// If callstack top is an index to the same node we'll delete it
			log.Tracef("TARGET %v", reflect.TypeOf(currentNode.Target))
			callStack.Push(currentNode)
			// If apply target is an index we need to add it to the search stack
			if _, ok := currentNode.Target.(*ast.Index); ok {
				nodesToSearch.Push(currentNode.Target)
			}
		default:
			callStack.Push(currentNode)
		}
	}

	for _, n := range callStack.Stack {
		log.Tracef("## Call: %v", reflect.TypeOf(n))
	}
	return callStack
}

func AddFunctionToStack(applyNode *ast.Apply, funcNode *ast.Function, searchstack *nodestack.NodeStack) *nodestack.NodeStack {
	searchstack = searchstack.Clone()
	// Get all positional arguments first. After that only named arguments remain
	for i, arg := range applyNode.Arguments.Positional {
		log.Tracef("Positional argument: %s", funcNode.Parameters[i].Name)
		searchstack.Push(&ast.Local{
			Binds: []ast.LocalBind{{
				Variable: funcNode.Parameters[i].Name,
				Body:     arg.Expr,
			}}})
	}
	for _, arg := range applyNode.Arguments.Named {
		log.Tracef("Named argument: %+v", arg)
		searchstack.Push(&ast.Local{
			Binds: []ast.LocalBind{{
				Variable: arg.Name,
				Body:     arg.Arg,
			}}})
	}
	searchstack.Push(funcNode.Body)
	return searchstack
}

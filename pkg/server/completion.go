package server

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/formatter"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	"github.com/grafana/jsonnet-language-server/pkg/cst"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/grafana/jsonnet-language-server/pkg/utils"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	log "github.com/sirupsen/logrus"
)

func (s *Server) Completion(_ context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	doc, err := s.cache.Get(params.TextDocument.URI)
	if err != nil {
		log.Errorf("Could not get document: %v", err)
		return nil, utils.LogErrorf("Completion: %s: %w", errorRetrievingDocument, err)
	}

	line := getCompletionLine(doc.Item.Text, params.Position)

	// Short-circuit if it's a stdlib completion
	if items := s.completionStdLib(line); len(items) > 0 {
		return &protocol.CompletionList{IsIncomplete: false, Items: items}, nil
	}

	// Otherwise, parse the AST and search for completions
	if doc.AST == nil {
		log.Errorf("Completion: document %s was never successfully parsed, can't autocomplete", params.TextDocument.URI)
		return nil, nil
	}

	info, err := cst.FindCompletionNode(context.Background(), doc.Item.Text, params.Position)

	if err != nil {
		log.Errorf("Unable to find completion node: %v", err)
		return nil, err
	}

	switch info.CompletionType {
	case cst.CompleteGlobal:
		searchStack, err := processing.FindNodeByPositionForReference(doc.AST, position.ProtocolToAST(params.Position))
		if err != nil {
			log.Errorf("Unable to find node position: %v", err)
			return nil, err
		}
		items := s.completeGlobal(info, searchStack, params.Position)
		return &protocol.CompletionList{IsIncomplete: false, Items: items}, nil

	case cst.CompleteImport:
		allFiles := map[string]bool{}
		indexParts := strings.Split(info.Index, "/")
		currentPath := strings.Join(indexParts[:len(indexParts)-1], "/")
		//		currentIndex := indexParts[len(indexParts)-1]
		importPaths := s.configuration.JPaths
		currentFileDir := filepath.Dir(doc.Item.URI.SpanURI().Filename())
		importPaths = append(importPaths, currentFileDir)
		for _, jpath := range importPaths {
			currentPath := filepath.Join(jpath, currentPath)
			files, err := utils.GetAllJsonnetFiles(currentPath)
			if err != nil {
				return nil, err
			}
			jpathAbsolute, err := filepath.Abs(jpath)
			if err != nil {
				return nil, err
			}
			for _, file := range files {
				relativePath := strings.TrimPrefix(file, jpathAbsolute+"/")
				allFiles[relativePath] = true
			}
		}
		var items []protocol.CompletionItem
		for relPath := range allFiles {
			items = append(items, protocol.CompletionItem{
				Label: relPath,
				Kind:  protocol.FileCompletion,
			})
		}
		return &protocol.CompletionList{IsIncomplete: false, Items: items}, nil
	}

	found := info.Node

	// TODO: get prev sibling -> get first fieldaccess -> get last child
	// Get prev node to get value to compile and complete from
	// prevNode := cst.GetPrevNode(found)
	searchStack, err := processing.FindNodeByPosition(doc.AST, position.CSTToAST(found.StartPosition()))
	if err != nil {
		return nil, fmt.Errorf("finding ast node %v", err)
	}

	log.Tracef("top item %v", reflect.TypeOf(searchStack.Peek()))

	items := s.createCompletionItems(searchStack, params.Position, info.InjectIndex)
	log.Tracef("Items: %+v", items)

	return &protocol.CompletionList{IsIncomplete: false, Items: items}, nil
}

func DesugaredObjectFieldsToString(node *ast.DesugaredObject) string {
	var builder strings.Builder

	if node == nil {
		return "nil"
	}
	for _, field := range node.Fields {
		if fieldName, ok := field.Name.(*ast.LiteralString); ok {
			builder.WriteString(fmt.Sprintf("Name: %v type: %+v\n", fieldName.Value, reflect.TypeOf(field.Body)))
			if child, ok := field.Body.(*ast.DesugaredObject); ok {
				builder.WriteString(DesugaredObjectFieldsToString(child))
			}
		}
	}
	return builder.String()
}

func getObjectFieldMap(object *ast.DesugaredObject) map[string]ast.DesugaredObjectField {
	fieldMap := map[string]ast.DesugaredObjectField{}
	for _, newField := range object.Fields {
		if nameNode, ok := newField.Name.(*ast.LiteralString); ok {
			fieldMap[nameNode.Value] = newField
		}
	}
	return fieldMap
}

// Merges all desugared Objects into one
// TODO: does not support + at the moment
func mergeDesugaredObjects(objects []*ast.DesugaredObject) *ast.DesugaredObject {
	if len(objects) == 0 {
		return nil
	}
	var newObject ast.DesugaredObject

	for len(objects) != 0 {
		var object *ast.DesugaredObject
		object, objects = objects[0], objects[1:]
		newObject.Asserts = append(newObject.Asserts, object.Asserts...)
		newObject.Fields = append(newObject.Fields, object.Fields...)
		newFields := getObjectFieldMap(&newObject)
		currentFields := getObjectFieldMap(object)
		maps.Copy(newFields, currentFields)
		// FUCK YOU GO AND YOUR STUPID FAKE ITERATORS! There is no way this isn't a feature. I have to miss something...
		// This is the long version of a simple "map" call...
		vals := make([]ast.DesugaredObjectField, 0, len(newFields))
		for _, v := range newFields {
			vals = append(vals, v)
		}
		newObject.Fields = vals

		newObject.Locals = append(newObject.Locals, object.Locals...)
	}
	return &newObject
}

func (s *Server) ResolveApplyArguments(stack *nodestack.NodeStack, documentstack *nodestack.NodeStack) *nodestack.NodeStack {
	var funcNode *ast.Function
	var applyNode *ast.Apply

	searchstack := stack.Clone()
	// Get function object first
stackLoop:
	for !searchstack.IsEmpty() {
		log.Tracef("Node type %v", reflect.TypeOf(searchstack.Peek()))
		switch currentNode := searchstack.Pop().(type) {
		default:
			log.Warnf("UNHANDLED IN RESOLVE %v", reflect.TypeOf(currentNode))
		case *ast.Import:
			_, imported, err := s.getAst(currentNode.File.Value, currentNode.LocRange.FileName)
			if err != nil {
				log.Errorf("could not import ast from file %s: %v", currentNode.LocRange.FileName, err)
				return nil
			}
			searchstack.Push(imported)

		case *ast.Index:
			log.Tracef("INDEX TARGET %v index: %v", reflect.TypeOf(currentNode.Target), currentNode.Index)
			searchstack.Push(currentNode.Target)
		case *ast.Function:
			funcNode = currentNode
			break stackLoop
		case *ast.Apply:
			indexList := stack.Clone().BuildIndexList()
			log.Tracef("indexList: %v", indexList)
			// TODO: Special case: var.Id starts with $ -> internal function and needs to be evaluated
			// TODO: this requires us to implement at least $std.$objectFlatMerge due to hidden objects
			// TODO: special case if var is std -> evaluate // TODO: hidden objects?
			//nolint:goconst
			if len(indexList) > 0 && (indexList[0] == utils.StdIdentifier || indexList[0] == "$std") {
				// Special case: Call std function
				vm, root, err := s.getAst(currentNode.Loc().FileName, "")
				if err != nil {
					log.Errorf("%v", err)
					continue
				}
				processor := processing.NewProcessor(s.cache, vm)
				compiled, err := processor.CompileNode(root, currentNode)
				if err != nil {
					log.Errorf("Failed to compile node %v", err)
					continue
				}
				searchstack.Push(compiled)
				continue
			}
			log.Tracef("FUNC TARGET %v", reflect.TypeOf(currentNode.Target))
			searchstack.Push(currentNode.Target)
			applyNode = currentNode
		case *ast.Var:

			log.Tracef("Var %v", currentNode.Id)
			bind := processing.FindBindByIDViaStack(documentstack, currentNode.Id)
			if bind != nil {
				searchstack.Push(bind.Body)
			} else {
				log.Errorf("Unable to find bind %v", currentNode.Id)
			}
		}
	}

	if applyNode == nil {
		log.Errorf("Unable to find apply in stack")
		return nil
	}

	if funcNode == nil {
		log.Errorf("Unable to find func in stack")
		return nil
	}

	return s.addFunctionToStack(applyNode, funcNode, searchstack)
}

func (s *Server) addFunctionToStack(applyNode *ast.Apply, funcNode *ast.Function, searchstack *nodestack.NodeStack) *nodestack.NodeStack {
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

func (s *Server) buildCallStack(documentstack *nodestack.NodeStack) *nodestack.NodeStack {
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
			swapNode := false
			if prevApply, ok := callStack.Peek().(*ast.Apply); ok {
				if prevApply.Target == currentNode {
					// If the apply is the current target, we need to swap the order. This way the apply binds get added before the index for the function
					log.Tracef("REMOVING PREV with target %v", reflect.TypeOf(currentNode.Target))
					swapNode = false
				}
			}
			if swapNode {
				tmp := callStack.Pop()
				callStack.Push(currentNode)
				callStack.Push(tmp)
			} else {
				callStack.Push(currentNode)
			}
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
			nodesToSearch.Push(currentNode.Target)
		default:
			callStack.Push(currentNode)
		}
	}

	for _, n := range callStack.Stack {
		log.Tracef("## Call: %v", reflect.TypeOf(n))
	}
	return callStack
}

func (s *Server) evaluateObjectFields(node *ast.DesugaredObject, documentstack *nodestack.NodeStack) *ast.DesugaredObject {
	// TODO: node.clone()
	for i, field := range node.Fields {
		resolved := s.desugaredObjectKeyToString(field.Name, documentstack)
		if resolved != nil {
			node.Fields[i].Name = resolved
		}
	}
	return node
}

func (s *Server) desugaredObjectKeyToString(node ast.Node, documentstack *nodestack.NodeStack) *ast.LiteralString {
	// handle conditional
	switch currentNode := node.(type) {
	case *ast.LiteralString:
		return currentNode
	case *ast.Conditional:
		vm := s.getVM(node.Loc().FileName)
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
			return s.desugaredObjectKeyToString(currentNode.BranchTrue, documentstack)
		}
		return s.desugaredObjectKeyToString(currentNode.BranchFalse, documentstack)
	}

	return nil
}

//nolint:unused
func (s *Server) getReturnObject(root ast.Node) *nodestack.NodeStack {
	objectStack := nodestack.NewNodeStack(root)
	searchstack := nodestack.NewNodeStack(root)

searchLoop:
	for !searchstack.IsEmpty() {
		switch currentNode := searchstack.Pop().(type) {
		case *ast.Local:
			log.Tracef("body type %v", reflect.TypeOf(currentNode.Body))
			searchstack.Push(currentNode.Body)
			objectStack.Push(currentNode.Body)

		default:
			log.Debugf("Breaking at %v", reflect.TypeOf(currentNode))
			break searchLoop
		}
	}
	return objectStack
}

func compareSelf(selfNode ast.Node, other ast.Node) bool {
	selfType := reflect.TypeFor[*ast.Self]()
	return reflect.TypeOf(selfNode) == selfType && reflect.TypeOf(other) == selfType && selfNode.Context() == other.Context()
}

// TODO: handle appply in this function
// Just add the apply node to the documentstack
// Then on function search for the apply node and add the variables to the stack
//
//nolint:gocyclo // I know TODO
func (s *Server) getDesugaredObject(callstack *nodestack.NodeStack, documentstack *nodestack.NodeStack) *ast.DesugaredObject {
	searchstack := nodestack.NewNodeStack(callstack.Peek())
	desugaredObjects := []*ast.DesugaredObject{}
	if len(documentstack.Stack) > 10_000 {
		log.Errorf("Stack too large")
		documentstack.PrintStack()
		return nil
	}

	log.Debugf("getDesugaredObject start: %+v", reflect.TypeOf(searchstack.Peek()))
	for !searchstack.IsEmpty() {
		documentstack.Push(searchstack.Peek())
		log.Debugf("Getting desugared object for %v", reflect.TypeOf(searchstack.Peek()))
		switch currentNode := searchstack.Pop().(type) {
		case *ast.Var:
			log.Debugf("#Var %v", currentNode.Id)
			log.Debugf("next search %v", reflect.TypeOf(callstack.Peek()))
			switch currentNode.Id {
			case "$":
				// XXX: Dollar is a node and not ast.Dollar. For whatever reason
				searchstack.Push(&ast.Dollar{NodeBase: currentNode.NodeBase})
			default:
				ref := processing.FindNodeByID(documentstack, currentNode.Id)
				if ref != nil {
					log.Debugf("Ref is %v", reflect.TypeOf(ref))
					searchstack.Push(ref)
				} else {
					log.Errorf("Unable to find reference to var %s", currentNode.Id)
				}
			}
		case *ast.DesugaredObject:
			obj := s.evaluateObjectFields(currentNode, documentstack)
			if obj != nil {
				desugaredObjects = append(desugaredObjects, currentNode)
			}
		case *ast.Self:
			// Create new stack to find the correct desugared object
			tempStack, err := processing.FindNodeInStack(currentNode, documentstack)
			if err != nil {
				log.Errorf("Unable to find self object from front: %v", err)
				continue
			}
			// Search for next DesugaredObject
			selfObject, pos, err := tempStack.FindNext(reflect.TypeFor[*ast.DesugaredObject]())
			if err != nil {
				log.Errorf("Unable to find self object from self: %v", err)
				continue
			}
			// Self refers to the next desugared object resolving all binaries: {a:1} + {b: self.a, c: self.d} + {d:2}
			// Super only refers to the "upper" binaries: {a:1} + {b: super.a, c: self.d} + {d:2}

			// Find top most binary and flatten it to also get the binary after the current one
			var binary *ast.Binary
			for {
				binNode, binaryPos, err := tempStack.FindNextFromIndex(reflect.TypeFor[*ast.Binary](), pos-1)
				if err != nil {
					break
				}
				if pos-1 != binaryPos {
					log.Debugf("Pos not matching %+v binpos %+v", pos, binaryPos)
					break
				}
				log.Debugf("Found new binary")
				//nolint:forcetypeassert // go shit
				binary = binNode.(*ast.Binary)
				pos = binaryPos
			}
			if binary == nil {
				searchstack.Push(selfObject)
				// No binary
				continue
			}
			binaryChildren := processing.FlattenBinary(binary)
			// Add all children except the self target
			for _, child := range binaryChildren {
				if !compareSelf(currentNode, child) {
					searchstack.Push(child)
				}
			}

		case *ast.SuperIndex:
			// Create new stack to find the correct desugared object
			tempStack, err := processing.FindNodeByPosition(documentstack.PeekFront(), currentNode.Loc().Begin)
			if err != nil {
				log.Errorf("Unable to find self object from front: %v", err)
				continue
			}
			// Search for next DesugaredObject
			selfObject, pos, err := tempStack.FindNext(reflect.TypeFor[*ast.DesugaredObject]())
			if err != nil {
				log.Errorf("Unable to find self object from super index: %v", err)
				continue
			}
			// Self refers to the next desugared object resolving all binaries: {a:1} + {b: self.a, c: self.d} + {d:2}
			// Super only refers to the "upper" binaries: {a:1} + {b: super.a, c: self.d} + {d:2}

			// This resolves all binaries "above"
			binNode, binaryPos, err := tempStack.FindNextFromIndex(reflect.TypeFor[*ast.Binary](), pos)
			searchstack.Push(selfObject)
			if err != nil {
				// No binary
				continue
			}
			// Self refers to a binary
			//nolint:forcetypeassert // go shit
			binary := binNode.(*ast.Binary)
			if pos-1 == binaryPos {
				// Direct parent
				searchstack.Push(binary.Right)
				searchstack.Push(binary.Left)
			}

		case *ast.Import:
			log.Debugf("Trying to import %s from %s", currentNode.File.Value, currentNode.LocRange.FileName)
			_, imported, err := s.getAst(currentNode.File.Value, currentNode.LocRange.FileName)
			if err == nil {
				// TODO: import has to have a clean stack (otherwise the locals might be weird?) but an import cannot always be a DesugaredObject due to function calls
				log.Errorf("Imported for %v", reflect.TypeOf(imported))
				// importedObject := s.getDesugaredObject(imported, nodestack.NewNodeStack(imported))
				// TODO: find the "return" of the imported node -> most top level non local object
				// TODO: maybe the existing function for top level object?

				// Remove the import from the stack to prevent infinite loops
				documentstack.Pop()
				searchstack.Push(imported)

				// returnStack := s.getReturnObject(imported)
				// log.Errorf("Prev %T prevprev %T", documentstack.Peek(), documentstack.Stack[len(documentstack.Stack)-2])
				//// FIXME: workaround for (import 'x')(arg) call
				// if len(documentstack.Stack) > 1 {
				//	if applyNode, ok := documentstack.Stack[len(documentstack.Stack)-2].(*ast.Apply); ok {
				//		returnStack.PushFront(applyNode)
				//	}
				// }

				////importedObject := s.buildDesugaredObject(returnStack)
				// importedObject := s.getDesugaredObject(returnStack.Clone(), returnStack)
				// if importedObject != nil {
				//	searchstack.Push(importedObject)
				// } else {
				//	log.Errorf("Imported is nil!")
				// }
			} else {
				log.Errorf("Failed to import %s: %v", currentNode.File.Value, err)
			}
		case *ast.Local:
			// We might push locals without a body e.g. for resolving function parameters
			if currentNode.Body != nil {
				log.Tracef("Local %v", reflect.TypeOf(currentNode.Body))
				documentstack.Push(currentNode.Body)
				obj := s.buildDesugaredObject(documentstack)
				if obj != nil {
					searchstack.Push(obj)
				}
			}
		// There might be indices in imports
		case *ast.Index:

			log.Tracef("Index with name %v", currentNode.Index)
			obj := s.buildDesugaredObject(documentstack)
			if obj != nil {
				searchstack.Push(obj)
			}

		case *ast.Binary:
			searchstack.Push(currentNode.Right)
			searchstack.Push(currentNode.Left)
		case *ast.Apply:
			indexList := nodestack.NewNodeStack(currentNode).BuildIndexList()
			log.Tracef("indexList: %v", indexList)
			// TODO: Special case: var.Id starts with $ -> internal function and needs to be evaluated
			// TODO: this requires us to implement at least $std.$objectFlatMerge due to hidden objects
			// TODO: special case if var is std -> evaluate // TODO: hidden objects?
			if len(indexList) > 0 && (indexList[0] == utils.StdIdentifier || indexList[0] == "$std") {
				// Special case: Call std function
				// Find the filename since not all nodes have it
				filename := currentNode.Loc().FileName
				for i := len(documentstack.Stack) - 1; i >= 0 && len(filename) == 0; i-- {
					filename = documentstack.Stack[i].Loc().FileName
				}
				vm, root, err := s.getAst(filename, "")
				if err != nil {
					log.Errorf("Getting vm %v %+v", err, currentNode.Loc())
					continue
				}
				processor := processing.NewProcessor(s.cache, vm)
				compiled, err := processor.CompileNode(root, currentNode)
				if err != nil {
					log.Errorf("Failed to compile node %v", err)
					continue
				}
				searchstack.Push(compiled)
				if desugar, ok := compiled.(*ast.DesugaredObject); ok {
					log.Tracef("Compiled desugar: %s", DesugaredObjectFieldsToString(desugar))
				}
				log.Debugf("Compiled! %+v", reflect.TypeOf(compiled))
				continue
			}
			searchstack.Push(currentNode.Target)
		case *ast.Function:
			// Find apply node on documentstack
			foundNode, _, err := documentstack.FindNext(reflect.TypeFor[*ast.Apply]())
			if err != nil {
				log.Errorf("Unable to find apply node in document stack. Size %d", len(documentstack.Stack))
				continue
			}
			//nolint:forcetypeassert // Due to the lack of features in go FindNext can't be a generic with a proper return type
			applyNode := foundNode.(*ast.Apply)
			searchstack.Push(currentNode.Body)
			log.Tracef("pushed %T", currentNode.Body)

			// Get all positional arguments first. After that only named arguments remain
			for i, arg := range applyNode.Arguments.Positional {
				if i >= len(currentNode.Parameters) {
					log.Errorf("arguments are longer than the parameters!")
					continue
				}
				log.Tracef("Positional argument: %s", currentNode.Parameters[i].Name)
				searchstack.Push(&ast.Local{
					Binds: []ast.LocalBind{{
						Variable: currentNode.Parameters[i].Name,
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

		case *ast.Dollar:
			myStack := documentstack.Clone()
			for !myStack.IsEmpty() {
				log.Tracef("DOLLAR %v", reflect.TypeOf(myStack.PeekFront()))
				switch dollarSearchNode := myStack.PopFront().(type) {
				// TODO: this only returns the outer most object. But we want the outer most object inside the current local
				case *ast.DesugaredObject:
					return dollarSearchNode
				case *ast.Binary:
					callstack.Push(dollarSearchNode)
					return s.getDesugaredObject(callstack, documentstack)
				}
			}
		case *ast.Conditional:
			resolved, err := s.resolveConditional(currentNode, documentstack)
			if err != nil {
				log.Errorf("Failed to resolve conditional: %v", err)
				if s.configuration.Workarounds.AssumeTrueConditionOnError {
					log.Warnf("Using default path as a workaround")
					resolved = currentNode.BranchTrue
				} else {
					continue
				}
			}
			searchstack.Push(resolved)

		case *ast.Array:
			// Pop array
			callstack.Pop()
			// Next is index
			nextNode := callstack.Pop()
			if nextNode == nil {
				log.Errorf("Array node has no next node")
				continue
			}
			indexNode, ok := nextNode.(*ast.Index)
			if !ok {
				log.Errorf("Array node has no index node")
				continue
			}

			log.Tracef("ARRAY %+v", currentNode)
			callstack.PrintStack()
			indexVal, ok := indexNode.Index.(*ast.LiteralNumber)
			if !ok {
				log.Errorf("Index is not a number")
				continue
			}
			indexNum, err := strconv.Atoi(indexVal.OriginalString)
			if err != nil {
				log.Errorf("Could not convert index to an int")
				continue
			}
			if indexNum >= len(currentNode.Elements) {
				log.Errorf("Index out of bounds")
				continue
			}
			searchstack.Push(currentNode.Elements[indexNum].Expr)

		default:
			log.Errorf("Unhandled type in getDesugaredObject: %v", reflect.TypeOf(currentNode))
		}
	}
	merged := mergeDesugaredObjects(desugaredObjects)
	if merged != nil {
		documentstack.Push(merged)
	}
	return merged
}

func (s *Server) resolveConditional(node *ast.Conditional, documentstack *nodestack.NodeStack) (ast.Node, error) {
	filename := documentstack.GetNextFilename()
	vm := s.getVM(filename)
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

// a.b.c(arg).d.e
// start at a
// get desurgared object for each step
// does only act on complete indices. The current typing index is handled one layer above
func (s *Server) buildDesugaredObject(documentstack *nodestack.NodeStack) *ast.DesugaredObject {
	callstack := s.buildCallStack(documentstack)

	log.Debugf("Callstack %+v", callstack)
	// First object is var or func -> resolve to desugared object (including their keys)
	baseObject := s.getDesugaredObject(callstack, documentstack)
	if baseObject == nil {
		return nil
	}

	// All others are indices, apply, or array -> find ind DesugaredObject and resolve next layer
stackLoop:
	for !callstack.IsEmpty() && baseObject != nil {
		callNode := callstack.Pop()
		log.Debugf("Looking at call %v", reflect.TypeOf(callNode))
		// nolint: gocritic // I might need more cases here
		switch callNode := callNode.(type) {
		// Resolve arguments and push them to the stack
		case *ast.Apply:
			log.Warnf("UNHANDLED APPLY in Callstack")
		// Search the current DesugaredObject and get the body for this index
		case *ast.Index:
			// TODO: get sub
			// TODO: keep local binds. Just push the parent object to the stack?
			// log.Errorf("Call node %+v", callNode)
			log.Debugf("Index %+v %v", callNode.Index, reflect.TypeOf(callNode.Target))
			// GIVE ME FUCKING ITERATORS!! this really seems just like C + gc... even js has proper iterators. FUCKING JS!
			// Just look at this for mess. This would be a simple filter with proper iterators

			// Check if the index has a name
			indexName, ok := callNode.Index.(*ast.LiteralString)
			if !ok {
				continue
			}
			log.Debugf("Finding %s in %s", indexName.Value, DesugaredObjectFieldsToString(baseObject))
			// Look at all fields of the desugared object
			for _, field := range baseObject.Fields {
				fieldName, ok := field.Name.(*ast.LiteralString)
				if !ok {
					log.Errorf("Field does not have a name!")
					continue
				}
				if indexName.Value == fieldName.Value {
					// member body might have function -> Bind stuff to stack and resolve object
					// TODO: Maybe modify apply and add function as target and call getDesugaredObject. I'd like to have all logic in one function
					funcNode, funcOk := field.Body.(*ast.Function)
					applyNode, applyOk := callstack.Peek().(*ast.Apply)
					var newDesugar *ast.DesugaredObject
					log.Tracef("Apply func %v %v", applyOk, funcOk)
					if applyOk && funcOk {
						// Pop the apply Node
						callstack.Pop()
						stack := s.addFunctionToStack(applyNode, funcNode, documentstack)
						if stack != nil {
							documentstack.Stack = append(documentstack.Stack, stack.Stack...)
						}
						newDesugar = s.getDesugaredObject(nodestack.NewNodeStack(stack.Peek()), documentstack)
					} else {
						log.Debugf("Found field %s", indexName.Value)
						newDesugar = s.getDesugaredObject(nodestack.NewNodeStack(field.Body), documentstack)
					}
					if newDesugar != nil {
						//if newDesugar, ok := field.Body.(*ast.DesugaredObject); ok {
						baseObject = newDesugar
						continue stackLoop
					}
					log.Errorf("Body is not a desugared object: %v", reflect.TypeOf(field.Body))
				}
			}
			// No match
			log.Errorf("No match for %v", indexName)
			return nil
		}
	}
	log.Debugf("finished object: %v", DesugaredObjectFieldsToString(baseObject))
	return baseObject
}

// Every node gets their own nodestack. E.g. to allow injecting local binds (for function args)
func (s *Server) createCompletionItems(searchstack *nodestack.NodeStack, pos protocol.Position, noEndIndex bool) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}
	searchstack = searchstack.Clone()
	indexName := ""
	if topIndex, ok := searchstack.Peek().(*ast.Index); ok && !noEndIndex {
		if indexNode, ok := topIndex.Index.(*ast.LiteralString); ok {
			indexName = indexNode.Value
			log.Debugf("Removing %s from stack", indexName)
			searchstack.Pop()
			// TODO: If different breaks TestCompletion/completion_in_function_arguments
			if searchstack.Peek() != topIndex.Target {
				searchstack.Push(topIndex.Target)
			}
		}
	}
	// tempstack := searchstack.Clone()
	// for !tempstack.IsEmpty() {
	//	log.Errorf("TREE FOR: %v", reflect.TypeOf(tempstack.Peek()))
	//	t := nodetree.BuildTree(nil, tempstack.Pop())
	//	log.Errorf("%s", t)
	// }

	log.Debugf("Searching completion for %v at %v", reflect.TypeOf(searchstack.Peek()), pos)
	object := s.buildDesugaredObject(searchstack)
	if object == nil {
		return items
	}

	// Sort by name
	sort.SliceStable(object.Fields, func(i, j int) bool {
		iName, iok := object.Fields[i].Name.(*ast.LiteralString)
		jName, jok := object.Fields[j].Name.(*ast.LiteralString)

		return iok && jok && iName.Value < jName.Value
	})
	for _, field := range object.Fields {
		if nameNode, ok := field.Name.(*ast.LiteralString); ok {
			if strings.HasPrefix(nameNode.Value, indexName) {
				items = append(items,
					s.createCompletionItem(nameNode.Value, "", protocol.VariableCompletion, field.Body, pos, true),
				)
			}
		}
	}

	return items
}

func getCompletionLine(fileContent string, position protocol.Position) string {
	line := strings.Split(fileContent, "\n")[position.Line]
	charIndex := int(position.Character)
	charIndex = min(charIndex, len(line))
	line = line[:charIndex]
	return line
}

func (s *Server) completeFunctionArguments(info *cst.CompletionNodeInfo, stack *nodestack.NodeStack, pos protocol.Position) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}
	if info.FunctionNode == nil {
		return nil
	}
	// Get the function arguments to complete
	// TODO: create function with stack parameter and pos to find the node?
	// TODO: Function to resolve apply from var
	foundNode, err := processing.FindNodeByPositionForReference(stack.PeekFront(), position.CSTToAST(info.FunctionNode.EndPosition()))
	if err != nil {
		log.Errorf("Could not get node for completing function arg names")
		return items
	}
	nextApplyNode, _, err := foundNode.FindNext(reflect.TypeFor[*ast.Apply]())
	if err != nil {
		log.Errorf("Could not find apply node in stack")
		return items
	}
	applyNode, ok := nextApplyNode.(*ast.Apply)
	if !ok {
		return items
	}
	var functionNode *ast.Function

	stdFunctionNode, err := s.getStdFunction(foundNode.Peek())

	if err == nil {
		functionNode = stdFunctionNode
	} else if indexNode, ok := foundNode.Peek().(*ast.Index); ok {
		// only resolve if we have an index. Otherwise we just have the function call
		indexName, ok := indexNode.Index.(*ast.LiteralString)
		if !ok {
			return items
		}
		foundNode.Pop()
		foundNode.Push(indexNode.Target)
		obj := s.buildDesugaredObject(foundNode)
		if obj == nil {
			return items
		}
		for _, field := range obj.Fields {
			if name, ok := field.Name.(*ast.LiteralString); ok {
				if name.Value != indexName.Value {
					continue
				}
				if funcNode, ok := field.Body.(*ast.Function); ok {
					functionNode = funcNode
				}
			}
		}
	} else {
		searchstack := nodestack.NewNodeStack(applyNode.Target)
		for !searchstack.IsEmpty() {
			currentNode := searchstack.Pop()
			switch currentNode := currentNode.(type) {
			case *ast.Var:
				resolved, err := processing.ResolveVar(currentNode, stack)
				if err != nil {
					return nil
				}
				searchstack.Push(resolved)

			case *ast.Function:
				functionNode = currentNode
			case *ast.Index:
				searchstack.Push(currentNode.Target)
			case *ast.Import:
				log.Debugf("Trying to import %s from %s", currentNode.File.Value, currentNode.LocRange.FileName)
				_, imported, err := s.getAst(currentNode.File.Value, currentNode.LocRange.FileName)
				if err == nil {
					searchstack.Push(imported)
				}

			default:
				log.Errorf("Unhandled %T", currentNode)
			}
		}
	}

	// FUCK YOU GO AND YOUR MISSING FEATURES! This is overly complicated because you lack basic features like iterators....
	setNamedArgs := []string{}

	for _, arg := range applyNode.Arguments.Named {
		setNamedArgs = append(setNamedArgs, string(arg.Name))
	}

	if functionNode == nil {
		log.Errorf("Could not find function node")
		return items
	}

	for i, param := range functionNode.Parameters {
		// Skip i unnamed parameters as they are already set
		// FIXME: if we complete "myFunc(a" a is considered a set argument and won't complete properly
		if i >= len(applyNode.Arguments.Positional) && !slices.Contains(setNamedArgs, string(param.Name)) {
			items = append(items, s.createCompletionItem(fmt.Sprintf("%s=", string(param.Name)), "", protocol.VariableCompletion, &ast.Var{}, pos, false))
		}
	}

	return items
}

func (s *Server) completeGlobal(info *cst.CompletionNodeInfo, stack *nodestack.NodeStack, pos protocol.Position) []protocol.CompletionItem {
	log.Tracef("##### Global path")
	items := []protocol.CompletionItem{}
	addSelf := false
	addSuper := false
	// TODO: determine when we can add a local
	addLocal := true

	items = append(items, s.completeFunctionArguments(info, stack, pos)...)

	for !stack.IsEmpty() {
		curr := stack.Pop()
		var binds ast.LocalBinds
		switch typedCurr := curr.(type) {
		case *ast.DesugaredObject:
			addSelf = true
			binds = typedCurr.Locals
			parentNode, _, err := stack.FindNext(reflect.TypeFor[*ast.Binary]())
			if err != nil {
				break
			}
			//nolint:forcetypeassert // go stuff
			parentBinary := parentNode.(*ast.Binary)
			addSuper = parentBinary.Right == curr

		case *ast.Local:
			binds = typedCurr.Binds
		case *ast.Function:
			for _, param := range typedCurr.Parameters {
				items = append(items, s.createCompletionItem(string(param.Name), "", protocol.VariableCompletion, &ast.Var{}, pos, true))
			}
		default:
			break
		}
		for _, bind := range binds {
			label := string(bind.Variable)
			items = append(items, s.createCompletionItem(label, "", protocol.VariableCompletion, bind.Body, pos, true))
		}
	}
	if addSelf {
		items = append(items, s.createCompletionItem("self", "", protocol.VariableCompletion, &ast.Self{}, pos, false))
	}
	if addSuper {
		items = append(items, s.createCompletionItem("super", "", protocol.VariableCompletion, &ast.SuperIndex{}, pos, false))
	}
	if addLocal {
		items = append(items, s.createCompletionItem("local", "", protocol.VariableCompletion, &ast.Local{}, pos, false))
	}

	filteredItems := []protocol.CompletionItem{}
	for _, item := range items {
		if strings.HasPrefix(item.Label, info.Index) && item.Label != "$" && (!strings.HasPrefix(item.Label, "#") || !s.configuration.ShowDocstringInCompletion) {
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems
}

func (s *Server) completionStdLib(line string) []protocol.CompletionItem {
	items := []protocol.CompletionItem{}

	stdIndex := strings.LastIndex(line, "std.")
	if stdIndex != -1 {
		userInput := line[stdIndex+4:]
		funcStartWith := []protocol.CompletionItem{}
		funcContains := []protocol.CompletionItem{}
		for _, f := range s.stdlib {
			if f.Name == userInput {
				break
			}
			lowerFuncName := strings.ToLower(f.Name)
			findName := strings.ToLower(userInput)
			item := protocol.CompletionItem{
				Label:         f.Name,
				Kind:          protocol.FunctionCompletion,
				Detail:        f.Signature(),
				InsertText:    strings.ReplaceAll(f.Signature(), "std.", ""),
				Documentation: f.MarkdownDescription,
			}

			if len(findName) > 0 && strings.HasPrefix(lowerFuncName, findName) {
				funcStartWith = append(funcStartWith, item)
				continue
			}

			if strings.Contains(lowerFuncName, findName) {
				funcContains = append(funcContains, item)
			}
		}

		items = append(items, funcStartWith...)
		items = append(items, funcContains...)
	}

	return items
}

func formatLabel(str string) string {
	interStr := "interimPath" + str
	fmtStr, _ := formatter.Format("", interStr, formatter.DefaultOptions())
	ret, _ := strings.CutPrefix(fmtStr, "interimPath")
	ret, _ = strings.CutPrefix(ret, ".")
	ret = strings.TrimRight(ret, "\n")
	return ret
}

//nolint:unparam // Currently prefix is always called with ""
func (s *Server) createCompletionItem(label, prefix string, kind protocol.CompletionItemKind, body ast.Node, position protocol.Position, tryEscape bool) protocol.CompletionItem {
	paramsString := ""
	if asFunc, ok := body.(*ast.Function); ok {
		kind = protocol.FunctionCompletion
		params := []string{}
		for _, param := range asFunc.Parameters {
			params = append(params, string(param.Name))
		}
		paramsString = "(" + strings.Join(params, ", ") + ")"
	}

	var insertText string
	if tryEscape {
		insertText = formatLabel("['" + label + "']" + paramsString)
	} else {
		insertText = label
	}

	concat := ""
	characterStartPosition := position.Character - 1
	if prefix == "" {
		characterStartPosition = position.Character
	}
	if prefix != "" && !strings.HasPrefix(insertText, "[") {
		concat = "."
		characterStartPosition = position.Character
	}
	detail := prefix + concat + insertText

	if s.configuration.UseTypeInDetail {
		detail = typeToString(body)
	}

	item := protocol.CompletionItem{
		Label:  label,
		Detail: detail,
		Kind:   kind,
		LabelDetails: &protocol.CompletionItemLabelDetails{
			Description: typeToString(body),
		},
		InsertText: insertText,
	}

	if strings.HasPrefix(insertText, "[") {
		item.TextEdit = &protocol.TextEdit{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      position.Line,
					Character: characterStartPosition,
				},
				End: protocol.Position{
					Line:      position.Line,
					Character: position.Character,
				},
			},
			NewText: insertText,
		}
	}

	return item
}

func typeToString(t ast.Node) string {
	switch t.(type) {
	case *ast.Array:
		return "array"
	case *ast.LiteralBoolean:
		return "boolean"
	case *ast.Function:
		return "function"
	case *ast.LiteralNull:
		return "null"
	case *ast.LiteralNumber:
		return "number"
	case *ast.Object, *ast.DesugaredObject:
		return "object"
	case *ast.LiteralString:
		return "string"
	case *ast.Import, *ast.ImportStr:
		return "import"
	case *ast.Index:
		return "object field"
	case *ast.Var:
		return "variable"
	case *ast.SuperIndex:
		return "super"
	}
	typeString := reflect.TypeOf(t).String()
	typeString = strings.ReplaceAll(typeString, "*ast.", "")
	typeString = strings.ToLower(typeString)

	return typeString
}

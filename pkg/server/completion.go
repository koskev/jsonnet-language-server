package server

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	"github.com/grafana/jsonnet-language-server/pkg/cst"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/grafana/jsonnet-language-server/pkg/server/completion"
	"github.com/grafana/jsonnet-language-server/pkg/stdlib"
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

	line := utils.GetCompletionLine(doc.Item.Text, params.Position)

	// Short-circuit if it's a stdlib completion
	if items := s.completionProvider.CompletionStdLib(line); len(items) > 0 {
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
	case cst.CompleteExtVar:
		log.Errorf("ext var %+v", s.configuration.ExtVars)
		log.Errorf("ext code %+v", s.configuration.ExtCode)
		var items []protocol.CompletionItem
		for key, value := range s.configuration.ExtVars {
			items = append(items, protocol.CompletionItem{
				Label:      key,
				Kind:       protocol.ValueCompletion,
				Detail:     value,
				InsertText: key,
			})
		}
		for key, value := range s.configuration.ExtCode {
			items = append(items, protocol.CompletionItem{
				Label:      key,
				Kind:       protocol.ValueCompletion,
				Detail:     value,
				InsertText: key,
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

	return completion.AddFunctionToStack(applyNode, funcNode, searchstack)
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
			obj := s.completionProvider.EvaluateObjectFields(currentNode, documentstack)
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
				if !utils.CompareSelf(currentNode, child) {
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
				obj := s.buildNode(documentstack)
				if obj != nil {
					searchstack.Push(obj)
				}
			}
		// There might be indices in imports
		case *ast.Index:

			log.Tracef("Index with name %v", currentNode.Index)
			obj := s.buildNode(documentstack)
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
					log.Tracef("Compiled desugar: %s", utils.DesugaredObjectFieldsToString(desugar))
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
			resolved, err := s.completionProvider.ResolveConditional(currentNode, documentstack)
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
	merged := utils.MergeDesugaredObjects(desugaredObjects)
	if merged != nil {
		documentstack.Push(merged)
	}
	return merged
}

// a.b.c(arg).d.e
// start at a
// get desurgared object for each step
// does only act on complete indices. The current typing index is handled one layer above
func (s *Server) buildNode(documentstack *nodestack.NodeStack) ast.Node {
	callstack := completion.BuildCallStack(documentstack)

	log.Debugf("Callstack %+v", callstack)
	// First object is var or func -> resolve to desugared object (including their keys)
	baseObject := s.getDesugaredObject(callstack, documentstack)
	if baseObject == nil {
		// No base object -> Just return the first object to complete
		return documentstack.Peek()
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
			log.Debugf("Finding %s in %s", indexName.Value, utils.DesugaredObjectFieldsToString(baseObject))
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
						stack := completion.AddFunctionToStack(applyNode, funcNode, documentstack)
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

					// If it is not an object, we just return the node. It might be an array where we want to add snippets
					return field.Body
				}
			}
			// No match
			log.Errorf("No match for %v", indexName)
			return nil
		}
	}
	log.Debugf("finished object: %v", utils.DesugaredObjectFieldsToString(baseObject))
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

	log.Errorf("Searching completion for %v at %v", reflect.TypeOf(searchstack.Peek()), pos)

	node := s.buildNode(searchstack.Clone())
	if node == nil {
		return items
	}

	if s.configuration.Completion.EnableSnippets {
		doc, err := s.cache.Get(protocol.URIFromPath(node.Loc().FileName))
		if err != nil {
			log.Errorf("Could not load file %s: %v", node.Loc().FileName, err)
		}
		items = append(items, s.completionProvider.CreateSnippets(searchstack, node, doc.Item.Text)...)
	}

	switch object := node.(type) {
	case *ast.DesugaredObject:

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
						s.completionProvider.CreateCompletionItem(nameNode.Value, "", protocol.VariableCompletion, field.Body, pos, true),
					)
				}
			}
		}
	default:
	}

	return items
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

	stdFunctionNode, err := stdlib.GetStdFunction(foundNode.Peek(), &s.stdlibMap)

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
		node := s.buildNode(foundNode)
		if node == nil {
			return items
		}
		obj, ok := node.(*ast.DesugaredObject)
		if !ok {
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
			items = append(items, s.completionProvider.CreateCompletionItem(fmt.Sprintf("%s=", string(param.Name)), "", protocol.VariableCompletion, &ast.Var{}, pos, false))
		}
	}

	return items
}

func (s *Server) completeGlobal(info *cst.CompletionNodeInfo, stack *nodestack.NodeStack, pos protocol.Position) []protocol.CompletionItem {
	log.Tracef("##### Global path")
	items := []protocol.CompletionItem{}

	items = append(items, s.completeFunctionArguments(info, stack, pos)...)

	searchStack := stack.Clone()

	for !searchStack.IsEmpty() {
		curr := searchStack.Pop()
		var binds ast.LocalBinds
		switch typedCurr := curr.(type) {
		case *ast.DesugaredObject:
			binds = typedCurr.Locals
		case *ast.Local:
			binds = typedCurr.Binds
		case *ast.Function:
			for _, param := range typedCurr.Parameters {
				items = append(items, s.completionProvider.CreateCompletionItem(string(param.Name), "", protocol.VariableCompletion, &ast.Var{}, pos, true))
			}
		default:
			break
		}
		for _, bind := range binds {
			label := string(bind.Variable)
			items = append(items, s.completionProvider.CreateCompletionItem(label, "", protocol.VariableCompletion, bind.Body, pos, true))
		}
	}

	items = append(items, s.completionProvider.CompleteKeywords(stack, pos)...)

	filteredItems := []protocol.CompletionItem{}
	for _, item := range items {
		if strings.HasPrefix(item.Label, info.Index) && item.Label != "$" && (!strings.HasPrefix(item.Label, "#") || !s.configuration.Completion.ShowDocstring) {
			filteredItems = append(filteredItems, item)
		}
	}

	return filteredItems
}

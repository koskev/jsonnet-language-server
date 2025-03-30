package server

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"sort"
	"strings"

	"github.com/google/go-jsonnet"
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

	if info.Global {
		searchStack, err := processing.FindNodeByPosition(doc.AST, position.ProtocolToAST(params.Position))
		if err != nil {
			log.Errorf("Unable to find node position: %v", err)
			return nil, err
		}
		items := s.completeGlobal([]string{info.Index}, searchStack, params.Position)
		return &protocol.CompletionList{IsIncomplete: false, Items: items}, nil
	}

	found := info.Node

	// TODO: get prev sibling -> get first fieldaccess -> get last child
	// Get prev node to get value to compile and complete from
	// prevNode := cst.GetPrevNode(found)
	searchStack, err := processing.FindNodeByPosition(doc.AST, position.CSTToAST(found.StartPosition()))
	if err != nil {
		log.Errorf("######## could not find ast node %v", err)
	}

	log.Errorf("top item %v", reflect.TypeOf(searchStack.Peek()))

	items := s.createCompletionItems(searchStack, params.Position, info.InjectIndex)
	log.Errorf("Items: %+v", items)
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
		log.Errorf("Node type %v", reflect.TypeOf(searchstack.Peek()))
		switch currentNode := searchstack.Pop().(type) {
		default:
			log.Errorf("UNHANDLED IN RESOLVE %v", reflect.TypeOf(currentNode))

		case *ast.Index:
			log.Errorf("INDEX TARGET %v", reflect.TypeOf(currentNode.Target))
			searchstack.Push(currentNode.Target)
		case *ast.Function:
			funcNode = currentNode
			break stackLoop
		case *ast.Apply:
			indexList := stack.Clone().BuildIndexList()
			log.Errorf("indexList: %v", indexList)
			// TODO: Special case: var.Id starts with $ -> internal function and needs to be evaluated
			// TODO: this requires us to implement at least $std.$objectFlatMerge due to hidden objects
			// TODO: special case if var is std -> evaluate // TODO: hidden objects?
			if len(indexList) > 0 && (indexList[0] == "std" || indexList[0] == "$std") {
				// Special case: Call std function
				evalResult, _ := s.getVM(currentNode.LocRange.FileName).Evaluate(currentNode)
				node, err := jsonnet.SnippetToAST("", evalResult)
				if err == nil {
					searchstack.Push(node)
					if desugar, ok := node.(*ast.DesugaredObject); ok {
						log.Errorf("Compiled desugar: %s", DesugaredObjectFieldsToString(desugar))
					}
					log.Errorf("Compiled! %+v", reflect.TypeOf(node))
					return searchstack
				}
				log.Errorf("Failed to compile node %v", err)
			}
			log.Errorf("FUNC TARGET %v", reflect.TypeOf(currentNode.Target))
			searchstack.Push(currentNode.Target)
			applyNode = currentNode
		case *ast.Var:

			log.Errorf("Var %v", currentNode.Id)
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
		log.Errorf("Positional argument: %s", funcNode.Parameters[i].Name)
		searchstack.Push(&ast.Local{
			Binds: []ast.LocalBind{{
				Variable: funcNode.Parameters[i].Name,
				Body:     arg.Expr,
			}}})
	}
	for _, arg := range applyNode.Arguments.Named {
		log.Errorf("Named argument: %+v", arg)
		searchstack.Push(&ast.Local{
			Binds: []ast.LocalBind{{
				Variable: arg.Name,
				Body:     arg.Arg,
			}}})
	}
	searchstack.Push(funcNode.Body)
	return searchstack
}

func getVarIndexApply(documentstack *nodestack.NodeStack) ast.Node {
	documentstack = documentstack.Clone()
	prevNode := documentstack.Pop()
	log.Errorf("varIndex %v", reflect.TypeOf(prevNode))
	_, ok := prevNode.(*ast.Var)
	if !ok {
		return prevNode
	}

stackLoop:
	for !documentstack.IsEmpty() {
		log.Errorf("GET NODE %v", reflect.TypeOf(documentstack.Peek()))
		switch currentNode := documentstack.Pop().(type) {
		case *ast.Index:
			prevNode = currentNode
		case *ast.Apply:
			prevNode = currentNode
		default:
			break stackLoop
		}
	}
	return prevNode
}

func (s *Server) buildCallStack(node ast.Node, documentstack *nodestack.NodeStack) *nodestack.NodeStack {
	nodesToSearch := nodestack.NewNodeStack(node)
	callStack := &nodestack.NodeStack{}
	log.Errorf("Building call stack from %v", reflect.TypeOf(node))

	for !nodesToSearch.IsEmpty() {
		currentNode := nodesToSearch.Pop()
		log.Errorf("CALL BUILD %v", reflect.TypeOf(currentNode))
		switch currentNode := currentNode.(type) {
		case *ast.Index:
			log.Errorf("INDEX TARGET %v", reflect.TypeOf(currentNode.Target))
			swapNode := false
			if prevApply, ok := callStack.Peek().(*ast.Apply); ok {
				if prevApply.Target == currentNode {
					// If the apply is the current target, we need to swap the order. This way the apply binds get added before the index for the function
					log.Errorf("REMOVING PREV with target %v", reflect.TypeOf(currentNode.Target))
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
			log.Errorf("New target %v %v", reflect.TypeOf(currentNode.Target), currentNode.Index)
		case *ast.Var:
			// TODO: somehow figure out function index stuff
			if _, ok := callStack.Peek().(*ast.Apply); !ok {
				callStack.Push(currentNode)
			}
			// Inside a function call the stack also contains the function. If we see a var we can abort as a var always marks the end of a "call"
		case *ast.Apply:
			// If callstack top is an index to the same node we'll delete it
			log.Errorf("TARGET %v", reflect.TypeOf(currentNode.Target))
			callStack.Push(currentNode)
			nodesToSearch.Push(currentNode.Target)
		default:
			callStack.Push(currentNode)
		}
	}

	for _, n := range callStack.Stack {
		log.Errorf("## Call: %v", reflect.TypeOf(n))
	}
	return callStack
}

func DesugaredObjectKeyToString(node ast.Node) *ast.LiteralString {
	// handle conditional
	switch currentNode := node.(type) {
	case *ast.LiteralString:
		return currentNode
	case *ast.Conditional:
		// TODO: evaluate conditional
		return DesugaredObjectKeyToString(currentNode.BranchTrue)
	}

	return nil
}

func (s *Server) getReturnObject(root ast.Node) *nodestack.NodeStack {
	objectStack := nodestack.NewNodeStack(root)
	searchstack := nodestack.NewNodeStack(root)

searchLoop:
	for !searchstack.IsEmpty() {
		switch currentNode := searchstack.Pop().(type) {
		case *ast.Local:
			log.Errorf("body type %v", reflect.TypeOf(currentNode.Body))
			searchstack.Push(currentNode.Body)
			objectStack.Push(currentNode.Body)

		default:
			log.Errorf("Breaking at %v", reflect.TypeOf(currentNode))
			break searchLoop
		}
	}
	return objectStack
}

func (s *Server) getDesugaredObject(callstack *nodestack.NodeStack, documentstack *nodestack.NodeStack) *ast.DesugaredObject {
	searchstack := nodestack.NewNodeStack(callstack.Peek())
	var desugaredObjects []*ast.DesugaredObject

	log.Errorf("getDesugaredObject start: %+v", reflect.TypeOf(searchstack.Peek()))
	// TODO: multiple objects -> merge or return multiple
	for !searchstack.IsEmpty() {
		documentstack.Push(searchstack.Peek())
		log.Errorf("Getting desugared object for %v", reflect.TypeOf(searchstack.Peek()))
		switch currentNode := searchstack.Pop().(type) {
		case *ast.Var:
			log.Errorf("#Var %v", currentNode.Id)
			log.Errorf("next search %v", reflect.TypeOf(callstack.Peek()))
			switch currentNode.Id {
			case "std", "$std":
				searchstack = s.ResolveApplyArguments(nodestack.NewNodeStack(callstack.PeekFront()), documentstack)
				// XXX: Dollar is a node and not ast.Dollar. For whatever reason
			case "$":
				searchstack.Push(&ast.Dollar{NodeBase: currentNode.NodeBase})
			default:
				// TODO: resolve default values of function arguments
				ref := processing.FindNodeByID(documentstack, currentNode.Id)
				if ref != nil {
					log.Errorf("Ref is %v", reflect.TypeOf(ref))
					searchstack.Push(ref)
				} else {
					log.Errorf("Unable to find reference to var %s", currentNode.Id)
				}
			}
		case *ast.DesugaredObject:
			// TODO: evaluate name/key
			desugaredObjects = append(desugaredObjects, currentNode)
		case *ast.Self:
			// Search for next DesugaredObject
			for !documentstack.IsEmpty() {
				log.Errorf("DOC SELF %v", reflect.TypeOf(documentstack.Peek()))
				if desugared, ok := documentstack.Pop().(*ast.DesugaredObject); ok {
					log.Errorf("Next in stack: %v", reflect.TypeOf(documentstack.Peek()))
					if bin, ok := documentstack.Pop().(*ast.Binary); ok {
						searchstack.Push(bin)
					} else {
						return desugared
						// searchstack.Push(desugared)
					}
				}
			}
		case *ast.Import:
			_, imported, err := s.getAst(currentNode.File.Value, currentNode.LocRange.FileName)
			if err == nil {
				log.Errorf("Imported for %v", reflect.TypeOf(imported))
				// importedObject := s.getDesugaredObject(imported, nodestack.NewNodeStack(imported))
				// TODO: find the "return" of the imported node -> most top level non local object
				// TODO: maybe the existing function for top level object?
				returnStack := s.getReturnObject(imported)
				importedObject := s.buildDesugaredObject(returnStack)
				if importedObject != nil {
					searchstack.Push(importedObject)
				} else {
					log.Errorf("Imported is nil!")
				}
			} else {
				log.Errorf("Failed to import %s: %v", currentNode.File.Value, err)
			}
		case *ast.Local:
			// TODO: why do we need this?
			if currentNode.Body != nil {
				log.Errorf("#### Local %v", reflect.TypeOf(currentNode.Body))
				documentstack.Push(currentNode.Body)
				obj := s.buildDesugaredObject(documentstack)
				searchstack.Push(obj)
			}
			// searchstack.Push(currentNode.Body)
		// There might be indices in imports
		case *ast.Index:

			log.Errorf("Index with name %v", currentNode.Index)
			obj := s.buildDesugaredObject(documentstack)
			searchstack.Push(obj)

		case *ast.Binary:
			searchstack.Push(currentNode.Right)
			searchstack.Push(currentNode.Left)
		case *ast.Apply:
			// TODO: this is somehow needed
			searchstack.Push(currentNode)
			applystack := s.ResolveApplyArguments(searchstack, documentstack)
			if applystack != nil {
				searchstack = applystack
				log.Errorf("New search stack %v", searchstack)
			}
		case *ast.Dollar:
			myStack := documentstack.Clone()
			for !myStack.IsEmpty() {
				log.Errorf("DOLLAR %v", reflect.TypeOf(myStack.PeekFront()))
				switch dollarSearchNode := myStack.PopFront().(type) {
				// TODO: this only returns the outer most object. But we want the outer most object inside the current local
				case *ast.DesugaredObject:
					return dollarSearchNode
				case *ast.Binary:
					callstack.Push(dollarSearchNode)
					return s.getDesugaredObject(callstack, documentstack)
				}
			}
		default:
			log.Errorf("Unhandled type in getDesugaredObject: %v", reflect.TypeOf(currentNode))
		}
	}
	return mergeDesugaredObjects(desugaredObjects)
}

// a.b.c(arg).d.e
// start at a
// get desurgared object for each step
// does only act on complete indices. The current typing index is handled one layer above
func (s *Server) buildDesugaredObject(documentstack *nodestack.NodeStack) *ast.DesugaredObject {
	node := getVarIndexApply(documentstack)
	callstack := s.buildCallStack(node, documentstack)

	log.Errorf("Callstack %+v", callstack)
	// First object is var or func -> resolve to desugared object (including their keys)
	baseObject := s.getDesugaredObject(callstack, documentstack)

	// All others are indices, apply, or array -> find ind DesugaredObject and resolve next layer
stackLoop:
	for !callstack.IsEmpty() && baseObject != nil {
		callNode := callstack.Pop()
		log.Errorf("Looking at call %v", reflect.TypeOf(callNode))
		// nolint: gocritic // I might need more cases here
		switch callNode := callNode.(type) {
		// Resolve arguments and push them to the stack
		case *ast.Apply:
			log.Errorf("UNHANDLED APPLY!")
		// Search the current DesugaredObject and get the body for this index
		case *ast.Index:
			// TODO: special case if apply
			// TODO: get sub
			// TODO: keep local binds. Just push the parent object to the stack?
			// log.Errorf("Call node %+v", callNode)
			log.Errorf("Index %+v %v", callNode.Index, reflect.TypeOf(callNode.Target))
			// GIVE ME FUCKING ITERATORS!! this really seems just like C + gc... even js has proper iterators. FUCKING JS!
			// Just look at this for mess. This would be a simple filter with proper iterators

			// Check if the index has a name
			indexName, ok := callNode.Index.(*ast.LiteralString)
			if !ok {
				continue
			}
			log.Errorf("Finding %s in %s", indexName.Value, DesugaredObjectFieldsToString(baseObject))
			// Look at all fields of the desugared object
			for _, field := range baseObject.Fields {
				fieldName, ok := field.Name.(*ast.LiteralString)
				if !ok {
					log.Errorf("Field does not have a name!")
					continue
				}
				if indexName.Value == fieldName.Value {
					// TODO: member body might have function -> Bind stuff to stack and resolve object
					// TODO: currently not working due to wrong injected target -> var but should be apply
					// TODO SO: Maybe modify apply and add function as target and call getDesugaredObject
					funcNode, funcOk := field.Body.(*ast.Function)
					applyNode, applyOk := callstack.Peek().(*ast.Apply)
					var newDesugar *ast.DesugaredObject
					log.Errorf("Apply func %v %v", applyOk, funcOk)
					if applyOk && funcOk {
						// Pop the apply Node
						callstack.Pop()
						stack := s.addFunctionToStack(applyNode, funcNode, documentstack)
						if stack != nil {
							documentstack.Stack = append(documentstack.Stack, stack.Stack...)
						}
						newDesugar = s.getDesugaredObject(nodestack.NewNodeStack(stack.Peek()), documentstack)
					} else {
						log.Errorf("Found field %s", indexName.Value)
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
	log.Errorf("finished object: %v", DesugaredObjectFieldsToString(baseObject))
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
			log.Errorf("Removing %s from stack", indexName)
			searchstack.Pop()
			// TODO: If different breaks TestCompletion/completion_in_function_arguments
			if searchstack.Peek() != topIndex.Target {
				searchstack.Push(topIndex.Target)
			}
		}
	}
	// tempstack := searchstack.Clone()
	//for !tempstack.IsEmpty() {
	//	log.Errorf("TREE FOR: %v", reflect.TypeOf(tempstack.Peek()))
	//	t := nodetree.BuildTree(nil, tempstack.Pop())
	//	log.Errorf("%s", t)
	//}

	log.Errorf("Searching completion for %v at %v", reflect.TypeOf(searchstack.Peek()), pos)
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
					createCompletionItem(nameNode.Value, "", protocol.VariableCompletion, field.Body, pos),
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

func (s *Server) completeGlobal(indexes []string, stack *nodestack.NodeStack, pos protocol.Position) []protocol.CompletionItem {
	log.Tracef("##### Global path")
	items := []protocol.CompletionItem{}
	// firstIndex is a variable (local) completion
	for !stack.IsEmpty() {
		curr := stack.Pop()
		var binds ast.LocalBinds
		switch typedCurr := curr.(type) {
		case *ast.DesugaredObject:
			binds = typedCurr.Locals
		case *ast.Local:
			binds = typedCurr.Binds
		case *ast.Function:
			for _, param := range typedCurr.Parameters {
				items = append(items, createCompletionItem(string(param.Name), "", protocol.VariableCompletion, &ast.Var{}, pos))
			}
		default:
			continue
		}
		for _, bind := range binds {
			label := string(bind.Variable)
			if strings.HasPrefix(label, indexes[0]) && label != "$" {
				items = append(items, createCompletionItem(label, "", protocol.VariableCompletion, bind.Body, pos))
			}
		}
	}
	return items
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

func createCompletionItem(label, prefix string, kind protocol.CompletionItemKind, body ast.Node, position protocol.Position) protocol.CompletionItem {
	paramsString := ""
	if asFunc, ok := body.(*ast.Function); ok {
		kind = protocol.FunctionCompletion
		params := []string{}
		for _, param := range asFunc.Parameters {
			params = append(params, string(param.Name))
		}
		paramsString = "(" + strings.Join(params, ", ") + ")"
	}

	insertText := formatLabel("['" + label + "']" + paramsString)

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
	}
	return reflect.TypeOf(t).String()
}

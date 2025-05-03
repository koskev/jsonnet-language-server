package server

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	"github.com/grafana/jsonnet-language-server/pkg/nodetree"
	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/sirupsen/logrus"
)

func (s *Server) InlayHint(_ context.Context, params *protocol.InlayHintParams) ([]protocol.InlayHint, error) {
	var inlayHints []protocol.InlayHint

	doc, err := s.cache.Get(params.TextDocument.URI)
	if err != nil {
		return nil, fmt.Errorf("getting document from cache: %w", err)
	}
	if doc.AST == nil {
		return nil, fmt.Errorf("document was never parsed")
	}

	// Generate complete AST for range
	// Iterate over all nodes
	// Apply inlay hint to all arguments
	// TODO: special case for stdlib
	tree := nodetree.BuildTree(nil, ast.Clone(doc.AST))
	vm := s.getVM(params.TextDocument.URI.SpanURI().Filename())

	if s.configuration.Inlay.EnableIndexValue {
		inlayHints = append(inlayHints, s.getInlayHintIndex(tree, vm)...)
	}
	if s.configuration.Inlay.EnableFunctionArgs {
		inlayHints = append(inlayHints, s.getInlayHintApplyArgs(tree, doc.AST, params.TextDocument.URI)...)
	}
	if s.configuration.Inlay.EnableDebugAst {
		inlayHints = append(inlayHints, s.getInlayHintASTDebug(tree)...)
	}
	return inlayHints, nil
}

func (s *Server) getInlayHintIndex(tree *nodetree.NodeTree, vm *jsonnet.VM) []protocol.InlayHint {
	var inlayHints []protocol.InlayHint
	for _, currentNode := range nodetree.GetTopNodesOfType[*ast.Index](tree) {
		// nolint: gocritic
		stack := nodestack.NewNodeStack(currentNode)
		processor := processing.NewProcessor(s.cache, vm)
		deepestNode := stack.Peek()

		for !stack.IsEmpty() {
			stackNode := stack.Pop()
			if stackNode, ok := stackNode.(*ast.Index); ok {
				if varNode, ok := stackNode.Target.(*ast.Var); ok {
					ref, err := processor.FindVarReference(varNode)
					if err != nil {
						continue
					}
					stackNode.Target = ref
				} else {
					stack.Push(stackNode.Target)
				}
			}
		}
		val, err := vm.Evaluate(deepestNode)
		if err != nil {
			continue
		}
		// Remove newline and duplicate whitespaces
		val = strings.Join(strings.Fields(val), " ")
		if len(val) > s.configuration.MaxInlayLength {
			val = fmt.Sprintf("%s...", val[:s.configuration.MaxInlayLength])
		}

		pos := position.ASTToProtocol(currentNode.Loc().End)
		// Push to end of line
		pos.Character += 1000
		inlayHints = append(inlayHints, protocol.InlayHint{
			Position:    &pos,
			PaddingLeft: true,
			Label:       []protocol.InlayHintLabelPart{{Value: val}},
		})
	}
	return inlayHints
}

func (s *Server) getInlayHintApplyArgs(tree *nodetree.NodeTree, root ast.Node, uri protocol.DocumentURI) []protocol.InlayHint {
	var inlayHints []protocol.InlayHint
	searchStack := nodestack.NodeStack{From: nil, Stack: tree.GetAllChildren()}
	for !searchStack.IsEmpty() {
		stackNode := searchStack.Pop()
		switch currentNode := stackNode.(type) {
		case *ast.DesugaredObject:
			for _, assertNode := range currentNode.Asserts {
				searchStack.PushNodes(nodetree.BuildTree(nil, assertNode).GetAllChildren())
			}
		case *ast.Apply:
			// Get target func
			functionNode, err := s.getFunctionCallTarget(root, currentNode.Target, uri)
			if err != nil {
				logrus.Warnf("Unable to get function call target for inlay hint: %v. %+v", err, currentNode.Target)
				continue
			}
			var names []string
			for _, param := range functionNode.Parameters {
				names = append(names, string(param.Name))
			}
			for i, applyParam := range currentNode.Arguments.Positional {
				if i >= len(names) {
					// Somehow we have more apply arguments, than function arguments
					break
				}
				if varNode, ok := applyParam.Expr.(*ast.Var); ok {
					// Hide the inlay hint if the name is the same
					if !s.configuration.Inlay.FunctionArgs.ShowWithSameName && string(varNode.Id) == names[i] {
						continue
					}
				}
				pos := position.ASTToProtocol(applyParam.Expr.Loc().Begin)
				inlayHints = append(inlayHints, protocol.InlayHint{
					Position:     &pos,
					PaddingRight: true,
					Label: []protocol.InlayHintLabelPart{
						{
							Value: fmt.Sprintf("%s:", names[i]),
						},
					},
				})
			}
		}
	}
	return inlayHints
}

func (s *Server) getInlayHintASTDebug(tree *nodetree.NodeTree) []protocol.InlayHint {
	var inlayHints []protocol.InlayHint

	for _, currentNode := range tree.GetAllChildren() {
		pos := position.ASTToProtocol(currentNode.Loc().Begin)
		inlayHints = append(inlayHints, protocol.InlayHint{
			Position:     &pos,
			PaddingRight: true,
			Label: []protocol.InlayHintLabelPart{
				{
					Value: reflect.TypeOf(currentNode).String(),
				},
			},
		})
	}
	return inlayHints
}

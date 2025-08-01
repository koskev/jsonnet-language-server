package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	"github.com/grafana/jsonnet-language-server/pkg/cst"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/grafana/jsonnet-language-server/pkg/stdlib"
	"github.com/grafana/jsonnet-language-server/pkg/utils"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

func getFunctionCallNode(stack *nodestack.NodeStack) (*ast.Apply, error) {
	for !stack.IsEmpty() {
		currnode := stack.Pop()
		// nolint:gocritic
		switch currnode := currnode.(type) {
		case *ast.Apply:
			// TODO: check if in range
			return currnode, nil
		}
	}
	return nil, fmt.Errorf("unable to find any locals")
}

func (s *Server) getFunctionCallTarget(root ast.Node, functionNode ast.Node, target protocol.DocumentURI) (*ast.Function, error) {
	vm := s.getVM(target.SpanURI().Filename())
	retFunc, err := stdlib.GetStdFunction(functionNode, &s.stdlibMap)
	if err == nil {
		return retFunc, nil
	}
	var locations []protocol.DefinitionLink
	beginLocations, err := s.findDefinition(root, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: target,
			},
			Position: position.ASTToProtocol(functionNode.Loc().Begin),
		},
	}, vm)
	if err == nil {
		locations = append(locations, beginLocations...)
	}

	endLocations, err := s.findDefinition(root, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: target,
			},
			Position: position.ASTToProtocol(functionNode.Loc().End),
		},
	}, vm)
	if err == nil {
		locations = append(locations, endLocations...)
	}

	for _, location := range locations {
		root, _, err := vm.ImportAST("", location.TargetURI.SpanURI().Filename())
		if err != nil {
			continue
		}
		stack, err := processing.FindNodeByPosition(root, position.ProtocolToAST(location.TargetRange.Start))
		if err != nil {
			continue
		}
		if stack.IsEmpty() {
			continue
		}
		functionNode, ok := stack.Pop().(*ast.Function)
		if !ok {
			continue
		}
		return functionNode, nil
	}

	return nil, fmt.Errorf("unable to find call target")
}

func (s *Server) SignatureHelp(_ context.Context, params *protocol.SignatureHelpParams) (*protocol.SignatureHelp, error) {
	var signatures []protocol.SignatureInformation
	doc, err := s.cache.Get(params.TextDocument.URI)
	if err != nil {
		return nil, utils.LogErrorf("Signature help: %s: %w", errorRetrievingDocument, err)
	}

	// TODO: patch in closing bracket if missing? probably in a general purpose "fixAST" function?
	stack, err := processing.FindNodeByPosition(doc.AST, position.ProtocolToAST(params.Position))
	if err != nil {
		return nil, fmt.Errorf("getting node stack %w", err)
	}

	node, err := getFunctionCallNode(stack)
	if err != nil {
		return nil, fmt.Errorf("getting current function node: %w", err)
	}
	// Go to definition
	// Get node
	// Get function signature
	functionNode, err := s.getFunctionCallTarget(doc.AST, node.Target, doc.Item.URI)
	if err != nil {
		return nil, fmt.Errorf("could not get target function: %w", err)
	}
	if len(node.FreeVars) == 0 {
		return nil, fmt.Errorf("could not extract function name")
	}
	funcName := string(node.FreeVars[0])
	// TODO: get documentation
	signatureInfo := protocol.SignatureInformation{
		Label: funcName,
	}
	var paramsString []string
	// Get name and args
	for _, param := range functionNode.Parameters {
		paramsString = append(paramsString, string(param.Name))
		signatureInfo.Parameters = append(signatureInfo.Parameters, protocol.ParameterInformation{
			Label: string(param.Name),
		})
	}

	pos, err := cst.GetParamPos(doc.Item.Text, position.ProtocolToCST(params.Position))
	if err != nil {
		return nil, fmt.Errorf("getting parameter pos: %w", err)
	}
	signatureInfo.Label = fmt.Sprintf("%s(%s)", funcName, strings.Join(paramsString, ", "))
	signatureInfo.ActiveParameter = pos
	signatures = append(signatures, signatureInfo)

	return &protocol.SignatureHelp{
		Signatures:      signatures,
		ActiveSignature: uint32(len(signatures) - 1),
		ActiveParameter: pos,
	}, nil
}

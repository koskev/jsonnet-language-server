package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/grafana/jsonnet-language-server/pkg/utils"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
)

func getArguments(node *ast.Apply) []*ast.Var {
	var vars []*ast.Var
	for _, arg := range node.Arguments.Positional {
		if v, ok := arg.Expr.(*ast.Var); ok {
			vars = append(vars, v)
		}
	}
	return vars
}

func getFunctionCallNode(stack *nodestack.NodeStack) (*ast.Apply, error) {
	for !stack.IsEmpty() {
		currnode := stack.Pop()
		switch currnode := currnode.(type) {
		case *ast.Apply:
			// TODO: check if in range
			return currnode, nil
		}
	}
	return nil, fmt.Errorf("unable to find any locals")
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
		return nil, fmt.Errorf("getting current function node")
	}
	// Go to definition
	// Get node
	// Get function signature
	vm := s.getVM(params.TextDocument.URI.SpanURI().Filename())
	locations, err := s.findDefinition(doc.AST, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{
				URI: params.TextDocument.URI,
			},
			Position: position.ASTToProtocol(node.Loc().Begin),
		},
	}, vm)
	if err != nil {
		return nil, fmt.Errorf("getting current function definition")
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
		signatureInfo.Label = fmt.Sprintf("%s(%s)", funcName, strings.Join(paramsString, ", "))
		signatureInfo.ActiveParameter = uint32(len(node.Arguments.Positional))
		// TODO: this does not work since the ast still has the old info with the old name. Therefore moving at the space before the old name or a new line this breaks.
		// Let's see if I can find an AST solution and not a line/text based
		for i, sourceParam := range node.Arguments.Positional {
			r := *sourceParam.Expr.Loc()
			// Allow a leading space or similar
			r.Begin.Column -= 1
			if processing.InRange(position.ProtocolToAST(params.Position), r) {
				signatureInfo.ActiveParameter = uint32(i)
				break
			}
		}
		signatures = append(signatures, signatureInfo)
	}

	return &protocol.SignatureHelp{
		Signatures:      signatures,
		ActiveSignature: uint32(len(signatures) - 1),
	}, nil
}

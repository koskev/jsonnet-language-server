package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/grafana/jsonnet-language-server/pkg/utils"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	log "github.com/sirupsen/logrus"
)

func (s *Server) Hover(_ context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	doc, err := s.cache.Get(params.TextDocument.URI)
	if err != nil {
		return nil, utils.LogErrorf("Hover: %s: %w", errorRetrievingDocument, err)
	}

	if doc.Err != nil {
		// Hover triggers often. Throwing an error on each request is noisy
		log.Errorf("Hover: %s", errorParsingDocument)
		return nil, nil
	}

	stack, err := processing.FindNodeByPosition(doc.AST, position.ProtocolToAST(params.Position))
	if err != nil {
		return nil, err
	}

	if stack.IsEmpty() {
		log.Debug("Hover: empty stack")
		return nil, nil
	}

	node := stack.Peek()

	_, isIndex := node.(*ast.Index)
	_, isVar := node.(*ast.Var)
	lineIndex := uint32(node.Loc().Begin.Line) - 1
	startIndex := uint32(node.Loc().Begin.Column) - 1
	line := strings.Split(doc.Item.Text, "\n")[lineIndex]
	if (isIndex || isVar) && strings.HasPrefix(line[startIndex:], utils.StdIdentifier) {
		functionNameIndex := startIndex + 4
		if functionNameIndex < uint32(len(line)) {
			functionName := utils.FirstWord(line[functionNameIndex:])
			functionName = strings.TrimSpace(functionName)

			for _, function := range s.stdlib {
				if function.Name == functionName {
					return &protocol.Hover{
						Range: protocol.Range{
							Start: protocol.Position{Line: lineIndex, Character: startIndex},
							End:   protocol.Position{Line: lineIndex, Character: functionNameIndex + uint32(len(functionName))}},
						Contents: protocol.MarkupContent{
							Kind:  protocol.Markdown,
							Value: fmt.Sprintf("`%s`\n\n%s", function.Signature(), function.MarkdownDescription),
						},
					}, nil
				}
			}
		}
	}

	definitionParams := &protocol.DefinitionParams{
		TextDocumentPositionParams: params.TextDocumentPositionParams,
	}
	definitions, err := s.findDefinition(doc.AST, definitionParams, s.getVM(doc.Item.URI.SpanURI().Filename()))
	if err != nil {
		log.Debugf("Hover: error finding definition: %s", err)
		return nil, nil
	}

	if len(definitions) == 0 {
		return nil, nil
	}

	// Show the contents at the target range
	// If there are multiple definitions, show the filenames+line numbers
	contentBuilder := strings.Builder{}
	for _, def := range definitions {
		if len(definitions) > 1 {
			header := fmt.Sprintf("%s:%d", def.TargetURI, def.TargetRange.Start.Line+1)
			if def.TargetRange.Start.Line != def.TargetRange.End.Line {
				header += fmt.Sprintf("-%d", def.TargetRange.End.Line+1)
			}
			contentBuilder.WriteString(fmt.Sprintf("## `%s`\n", header))
		}

		targetContent, err := s.cache.GetContents(def.TargetURI, def.TargetRange)
		if err != nil {
			log.Debugf("Hover: error reading target content: %s", err)
			return nil, nil
		}
		// Limit the content to 5 lines
		if strings.Count(targetContent, "\n") > 5 {
			targetContent = strings.Join(strings.Split(targetContent, "\n")[:5], "\n") + "\n..."
		}
		contentBuilder.WriteString(fmt.Sprintf("```jsonnet\n%s\n```\n", targetContent))

		if len(definitions) > 1 {
			contentBuilder.WriteString("\n")
		}
	}

	result := &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: contentBuilder.String(),
		},
	}
	if loc := node.Loc(); loc != nil {
		result.Range = position.RangeASTToProtocol(*loc)
	}

	return result, nil
}

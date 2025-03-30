package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/grafana/jsonnet-language-server/pkg/utils"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/sirupsen/logrus"
)

func (s *Server) getAst(fileName string, from string) (*jsonnet.VM, ast.Node, error) {
	vm := s.getVM(fileName)
	doc, err := s.cache.Get(protocol.DocumentURI(fileName))
	var root ast.Node
	if err != nil {
		root, _, err = vm.ImportAST(from, fileName)
		if err != nil {
			return nil, nil, err
		}
	} else {
		root = doc.AST
	}
	return vm, root, nil
}

func (s *Server) getSelectedIdentifier(filename string, pos protocol.Position) (string, error) {
	vm := s.getVM(filename)
	root, _, err := vm.ImportAST("", filename)
	if err != nil {
		return "", nil
	}

	searchStack, _ := processing.FindNodeByPositionForReference(root, position.ProtocolToAST(pos))
	for !searchStack.IsEmpty() {
		currentNode := searchStack.Pop()
		logrus.Tracef("Looking at %v", reflect.TypeOf(currentNode))
		switch currentNode := currentNode.(type) {
		case *ast.LiteralString:
			return currentNode.Value, nil
		case *ast.Var:
			return string(currentNode.Id), nil
		case *ast.Local:
			for _, bind := range currentNode.Binds {
				// TODO: why can there be multiple binds?
				if len(bind.Variable) > 0 {
					return string(bind.Variable), nil
				}
			}
		case *ast.Function:
			// Parameters
			for _, parameter := range currentNode.Parameters {
				if processing.InRange(position.ProtocolToAST(pos), parameter.LocRange) {
					return string(parameter.Name), nil
				}
			}
		}
	}

	return "", fmt.Errorf("unable to find selected identifier")
}

func (s *Server) findIdentifierLocations(path string, identifier string) ([]ast.LocationRange, error) {
	var ranges []ast.LocationRange

	doc, err := s.cache.Get(protocol.URIFromPath(path))
	var reader io.Reader
	if err == nil {
		reader = strings.NewReader(doc.Item.Text)
	} else {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening file %s", path)
		}
		reader = file
		defer file.Close()
	}

	scanner := bufio.NewScanner(reader)

	i := 0
	for scanner.Scan() {
		i++
		text := scanner.Text()
		currentLocation := 0
		for {
			location := strings.Index(text, identifier)
			if location < 0 || location > len(text) {
				break
			}
			text = text[location+len(identifier):]
			// TODO: check off by one
			ranges = append(ranges, ast.LocationRange{
				FileName: path,
				Begin: ast.Location{
					Line:   i,
					Column: currentLocation + location + 1,
				},
				End: ast.Location{
					Line: i,
					// TODO: Is the +1 correct? because we are already at the first character
					Column: currentLocation + location + len(identifier) + 1,
				},
			})
			currentLocation += location + len(identifier)
		}
	}

	return ranges, nil
}

func (s *Server) References(_ context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	response, err := s.findAllReferences(params.TextDocument.URI, params.Position, params.Context.IncludeDeclaration)
	return response, err
}

func (s *Server) findAllReferences(sourceURI protocol.DocumentURI, pos protocol.Position, includeSelf bool) ([]protocol.Location, error) {
	vm, root, err := s.getAst(sourceURI.SpanURI().Filename(), "")
	if err != nil {
		return nil, err
	}
	locations, err := s.findDefinition(root, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{

			Position: pos,
			TextDocument: protocol.TextDocumentIdentifier{
				URI: sourceURI,
			},
		},
	}, vm)
	if err == nil && len(locations) > 0 {
		definition := locations[len(locations)-1]
		sourceURI = definition.TargetURI
		pos = definition.TargetRange.Start
	}

	folders := s.configuration.JPaths
	u, err := url.Parse(string(sourceURI))
	if err != nil {
		return nil, fmt.Errorf("invalid params uri %s", sourceURI)
	}
	folders = append(folders, path.Dir(u.Path))
	allFiles := map[protocol.DocumentURI]struct{}{}

	// TODO: handle deleted and created files in cache
	for _, folder := range folders {
		files, err := utils.GetAllJsonnetFiles(folder)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			allFiles[protocol.URIFromPath(file)] = struct{}{}
		}
	}

	identifier, err := s.getSelectedIdentifier(sourceURI.SpanURI().Filename(), pos)
	if err != nil {
		return nil, err
	}
	if len(identifier) == 0 {
		return nil, fmt.Errorf("got empty identifier at document %s in position %v", sourceURI, pos)
	}
	var response []protocol.Location

	if includeSelf {
		sourcePos, err := s.findIdentifierLocations(sourceURI.SpanURI().Filename(), identifier)
		if err != nil {
			return nil, fmt.Errorf("getting source range")
		}
		for _, searchPos := range sourcePos {
			if processing.InRange(position.ProtocolToAST(pos), searchPos) {
				r := protocol.Location{
					Range: position.RangeASTToProtocol(searchPos),
					URI:   sourceURI,
				}
				response = append(response, r)
			}
		}
	}

	targetLocation := position.ProtocolToAST(pos)
	for uri := range allFiles {
		fileName := uri.SpanURI().Filename()
		locations, err := s.findIdentifierLocations(fileName, identifier)
		if err != nil {
			continue
		}
		if len(locations) == 0 {
			// No matches
			continue
		}
		vm, root, err := s.getAst(fileName, "")
		if err != nil {
			return nil, fmt.Errorf("getting ast for %s: %w", fileName, err)
		}
		response = append(response, s.findReference(root, &targetLocation, sourceURI.SpanURI().Filename(), vm, locations)...)
	}
	return response, nil
}

func (s *Server) findReference(root ast.Node, targetLocation *ast.Location, targetFilename string, vm *jsonnet.VM, testTargets []ast.LocationRange) []protocol.Location {
	var response []protocol.Location

	for _, currentTarget := range testTargets {
		loc := currentTarget.Begin
		logrus.Debugf("Trying to jump from %v in %v", loc, currentTarget.FileName)
		links, err := s.findDefinition(root, &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{

				Position: position.ASTToProtocol(loc),
				TextDocument: protocol.TextDocumentIdentifier{
					URI: protocol.DocumentURI(fmt.Sprintf("file://%v", &currentTarget.FileName)),
				},
			},
		}, vm)
		if err != nil {
			logrus.Debugf("Could not jump from %v: %v", loc.String(), err)
		}
		for _, link := range links {
			linkEnd := position.ProtocolToAST(link.TargetRange.End)
			linkStart := position.ProtocolToAST(link.TargetRange.Start)
			logrus.Debugf("Jumping from \"%s\"[%v] leads to \"%s\"[%v:%v]", currentTarget.FileName, loc.String(), link.TargetURI.SpanURI().Filename(), linkStart, linkEnd)
			if link.TargetURI.SpanURI().Filename() == targetFilename &&
				processing.InRange(*targetLocation, ast.LocationRange{Begin: linkStart, End: linkEnd}) {
				logrus.Debugf("hit target of %v", targetLocation)
				response = append(response, protocol.Location{
					URI:   protocol.DocumentURI(fmt.Sprintf("file://%s", currentTarget.FileName)),
					Range: position.RangeASTToProtocol(currentTarget),
				})
			}
		}
	}
	return response
}

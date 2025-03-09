package server

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/sirupsen/logrus"
)

func pointInRange(loc ast.Location, rangeBegin ast.Location, rangeEnd ast.Location) bool {
	return (rangeBegin.Line < loc.Line &&
		rangeEnd.Line > loc.Line) ||
		(loc.Line == rangeBegin.Line && rangeBegin.Column <= loc.Column) ||
		(loc.Line == rangeEnd.Line && rangeEnd.Column <= loc.Column)
}

func (s *Server) getSelectedIdentifier(filename string, pos protocol.Position) (string, error) {
	vm := s.getVM(filename)
	root, _, err := vm.ImportAST("", filename)
	if err != nil {
		logrus.Errorf("Getting ast %v", err)
		return "", nil
	}

	searchStack, _ := processing.FindNodeByPosition(root, position.ProtocolToAST(pos))
	for !searchStack.IsEmpty() {
		currentNode := searchStack.Pop()
		switch currentNode := currentNode.(type) {
		case *ast.LiteralString:
			return currentNode.Value, nil
		case *ast.Var:
			return string(currentNode.Id), nil
		}
	}

	return "", fmt.Errorf("unable to find selected identifier")
}

func getAllFiles(dir string) []string {
	var files []string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			if filepath.Ext(path) == ".libsonnet" || filepath.Ext(path) == ".jsonnet" {
				files = append(files, path)
			}
		}
		return nil
	})
	return files
}

func (s *Server) findIdentifierLocations(path string, identifier string) ([]ast.LocationRange, error) {
	var ranges []ast.LocationRange

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file %s", file)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	i := 0
	for scanner.Scan() {
		i++
		text := scanner.Text()
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
					Column: location + 1,
				},
				End: ast.Location{
					Line:   i,
					Column: location + len(identifier) + 1,
				},
			})
		}
	}

	return ranges, nil
}

func (s *Server) References(_ context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	response, _, err := s.findAllReferences(params.TextDocument.URI, params.Position)
	return response, err
}

func (s *Server) findAllReferences(sourceURI protocol.DocumentURI, pos protocol.Position) ([]protocol.Location, string, error) {
	folders := s.configuration.JPaths
	u, err := url.Parse(string(sourceURI))
	if err != nil {
		return nil, "", fmt.Errorf("invalid params uri %s", sourceURI)
	}
	folders = append(folders, path.Dir(u.Path))
	allFiles := map[string]struct{}{}

	for _, folder := range folders {
		files := getAllFiles(folder)
		for _, file := range files {
			allFiles[file] = struct{}{}
		}
	}

	identifier, err := s.getSelectedIdentifier(sourceURI.SpanURI().Filename(), pos)
	if err != nil {
		return nil, "", err
	}

	var response []protocol.Location
	targetLocation := position.ProtocolToAST(pos)
	for fileName, _ := range allFiles {
		locations, err := s.findIdentifierLocations(fileName, identifier)
		if err != nil {
			continue
		}
		if len(locations) == 0 {
			// No matches
			continue
		}
		vm := s.getVM(fileName)
		root, _, err := vm.ImportAST("", fileName)
		if err != nil {
			return nil, "", err
		}
		response = append(response, s.findReference(root, &targetLocation, sourceURI.SpanURI().Filename(), vm, locations)...)
	}
	return response, identifier, nil

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
					URI: protocol.DocumentURI(fmt.Sprintf("file://%s", &currentTarget.FileName)),
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
				pointInRange(*targetLocation, linkStart, linkEnd) {
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

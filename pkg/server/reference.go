package server

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"reflect"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/toolutils"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	"github.com/grafana/jsonnet-language-server/pkg/nodetree"
	position "github.com/grafana/jsonnet-language-server/pkg/position_conversion"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/sirupsen/logrus"
)

// TODO: should be a tree
func buildCompleteFlatStack(node ast.Node) *nodestack.NodeStack {

	newChildren := nodestack.NewNodeStack(node)
	stack := nodestack.NewNodeStack(node)

	for !newChildren.IsEmpty() {
		curr := newChildren.Pop()
		stack.Push(curr)
		logrus.Errorf("New child! Num left %d. Stack size %d", len(newChildren.Stack), len(stack.Stack))
		switch curr := curr.(type) {
		case *ast.DesugaredObject:
			for _, field := range curr.Fields {
				body := field.Body
				// Functions do not have a LocRange, so we use the one from the field's body
				if funcBody, isFunc := body.(*ast.Function); isFunc {
					funcBody.LocRange = field.LocRange
					newChildren.Push(funcBody)
				} else {
					newChildren.Push(field.Name)
					newChildren.Push(body)
				}
			}
			for _, local := range curr.Locals {
				newChildren.Push(local.Body)
			}
			for _, assert := range curr.Asserts {
				newChildren.Push(assert)
			}
		default:
			for _, c := range toolutils.Children(curr) {
				newChildren.Push(c)
			}
		}
	}
	return stack.ReorderDesugaredObjects()
}

func pointInRange(loc ast.Location, rangeBegin ast.Location, rangeEnd ast.Location) bool {
	return (rangeBegin.Line < loc.Line &&
		rangeEnd.Line > loc.Line) ||
		(loc.Line == rangeBegin.Line && rangeBegin.Column <= loc.Column) ||
		(loc.Line == rangeEnd.Line && rangeEnd.Column <= loc.Column)
}

func (s *Server) getSelectedNode(searchStack *nodestack.NodeStack, params *protocol.ReferenceParams) (*ast.Var, error) {
	selectedNode := searchStack.Pop()
	if selectedNode == nil {
		return nil, fmt.Errorf("finding a selected var")
	}
	switch selectedNode := selectedNode.(type) {
	case *ast.Var:
		return selectedNode, nil
	case *ast.Local:
		for _, bind := range selectedNode.Binds {
			searchStack.Push(bind.Body)
		}
		return s.getSelectedNode(searchStack, params)
	default:
		nodes := toolutils.Children(selectedNode)
		for _, node := range nodes {
			searchStack.Push(node)
		}
		return s.getSelectedNode(searchStack, params)
	}
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

func (s *Server) References(_ context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	// Load AST for all files in JPaths
	// Get current node from params location
	// search all asts for the current reference
	// profit?!

	folders := s.configuration.JPaths
	u, err := url.Parse(string(params.TextDocument.URI.SpanURI()))
	if err != nil {
		return nil, fmt.Errorf("invalid params uri %s", params.TextDocument.URI.SpanURI())
	}
	folders = append(folders, path.Dir(u.Path))
	allFiles := map[string]struct{}{}

	for _, folder := range folders {
		files := getAllFiles(folder)
		for _, file := range files {
			allFiles[file] = struct{}{}
		}
	}

	var response []protocol.Location
	targetLocation := position.ProtocolToAST(params.Position)
	for fileName, _ := range allFiles {
		vm := s.getVM(fileName)
		root, _, err := vm.ImportAST("", fileName)
		if err != nil {
			return nil, err
		}
		response = append(response, s.findReference(root, &targetLocation, params.TextDocument.URI.SpanURI().Filename(), vm)...)
	}

	return response, nil
}

func (s *Server) findReference(root ast.Node, targetLocation *ast.Location, targetFilename string, vm *jsonnet.VM) []protocol.Location {
	tree := nodetree.BuildTree(nil, root)
	logrus.Errorf("%s", tree)
	var response []protocol.Location

	for _, currentNode := range tree.GetAllChildren() {
		logrus.Errorf("Eval node %v at %v\n", reflect.TypeOf(currentNode), currentNode.Loc())
		patchedLoc := currentNode.Loc().Begin
		if currentNode.Loc().End.IsSet() {
			patchedLoc = currentNode.Loc().End
			// WHY?!
			// It seems the whole code base has an off by one error for the columns.
			// According to the comment Colum is 0 indexed and not 1 indexed like the line
			patchedLoc.Column -= 1
		}
		links, err := s.findDefinition(root, &protocol.DefinitionParams{
			TextDocumentPositionParams: protocol.TextDocumentPositionParams{

				Position: position.ASTToProtocol(patchedLoc),
				TextDocument: protocol.TextDocumentIdentifier{
					URI: protocol.DocumentURI(fmt.Sprintf("file://%s", currentNode.Loc().FileName)),
				},
			},
		}, vm)
		if err != nil {
			logrus.Errorf("Could not jump from %v with type %v: %w", patchedLoc.String(), reflect.TypeOf(currentNode), err)
		}
		for _, link := range links {
			linkEnd := position.ProtocolToAST(link.TargetRange.End)
			linkStart := position.ProtocolToAST(link.TargetRange.Start)
			logrus.Errorf("Jumping from \"%s\"[%v] with type %v leads to \"%s\"[%v:%v]", currentNode.Loc().FileName, patchedLoc.String(), reflect.TypeOf(currentNode), link.TargetURI.SpanURI().Filename(), linkStart, linkEnd)
			if link.TargetURI.SpanURI().Filename() == targetFilename &&
				pointInRange(*targetLocation, linkStart, linkEnd) {
				logrus.Errorf("hit target of %v", targetLocation)
				response = append(response, protocol.Location{
					URI: protocol.DocumentURI(fmt.Sprintf("file://%s", currentNode.Loc().FileName)),
					//Range: position.RangeASTToProtocol(*currentNode.Loc()),
					Range: protocol.Range{
						Start: position.ASTToProtocol(patchedLoc),
					},
				})
			}

		}
	}

	// for all nodes check if var
	// call findReference and check if same
	return response
}

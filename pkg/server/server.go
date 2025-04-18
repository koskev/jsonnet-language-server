package server

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/cache"
	"github.com/grafana/jsonnet-language-server/pkg/stdlib"
	"github.com/grafana/jsonnet-language-server/pkg/utils"
	tankaJsonnet "github.com/grafana/tanka/pkg/jsonnet/implementations/goimpl"
	"github.com/grafana/tanka/pkg/jsonnet/jpath"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	log "github.com/sirupsen/logrus"
)

const (
	errorRetrievingDocument = "unable to retrieve document from the cache"
	errorParsingDocument    = "error parsing the document"
)

// New returns a new language server.
func NewServer(name, version string, client protocol.ClientCloser, configuration Configuration) *Server {
	server := &Server{
		name:          name,
		version:       version,
		cache:         cache.New(),
		client:        client,
		configuration: configuration,

		diagQueue: make(map[protocol.DocumentURI]struct{}),
	}

	return server
}

// server is the Jsonnet language server.
type Server struct {
	name, version string

	stdlib []stdlib.Function
	cache  *cache.Cache
	client protocol.ClientCloser

	configuration      Configuration
	clientCapabilities protocol.ClientCapabilities

	// Diagnostics
	diagMutex   sync.RWMutex
	diagQueue   map[protocol.DocumentURI]struct{}
	diagRunning sync.Map
}

func (s *Server) GetCache() *cache.Cache {
	return s.cache
}

func (s *Server) getVM(path string) *jsonnet.VM {
	var vm *jsonnet.VM
	if s.configuration.ResolvePathsWithTanka {
		jpath, _, _, err := jpath.Resolve(path, false)
		if err != nil {
			log.Debugf("Unable to resolve jpath for %s: %s", path, err)
			// nolint: gocritic
			jpath = append(s.configuration.JPaths, filepath.Dir(path))
		}
		vm = tankaJsonnet.MakeRawVM(jpath, nil, nil, 0)
	} else {
		// nolint: gocritic
		jpath := append(s.configuration.JPaths, filepath.Dir(path))
		vm = jsonnet.MakeVM()
		importer := &jsonnet.FileImporter{JPaths: jpath}
		vm.Importer(importer)
	}

	resetExtVars(vm, s.configuration.ExtVars, s.configuration.ExtCode)
	return vm
}

func (s *Server) DidChange(_ context.Context, params *protocol.DidChangeTextDocumentParams) error {
	defer s.queueDiagnostics(params.TextDocument.URI)

	doc, err := s.cache.Get(params.TextDocument.URI)
	if err != nil {
		return utils.LogErrorf("DidChange: %s: %w", errorRetrievingDocument, err)
	}

	if params.TextDocument.Version > doc.Item.Version && len(params.ContentChanges) != 0 {
		oldText := doc.Item.Text
		// TODO: this is not LSP compatible. A change event might be limited by the range
		doc.Item.Text = params.ContentChanges[len(params.ContentChanges)-1].Text

		var ast ast.Node
		// Since go is stupid we are unable to get the internal error type and thus cannot get the error location. Nice one!
		ast, doc.Err = s.getFixedAst(doc.Item.URI.SpanURI().Filename(), doc.Item.Text, oldText)

		// If the AST parsed correctly, set it on the document
		// Otherwise, keep the old AST, and find all the lines that have changed since last AST
		if ast != nil {
			doc.AST = ast
			doc.LinesChangedSinceAST = map[int]bool{}
		} else {
			splitOldText := strings.Split(oldText, "\n")
			splitNewText := strings.Split(doc.Item.Text, "\n")
			for index, oldLine := range splitOldText {
				if index >= len(splitNewText) || oldLine != splitNewText[index] {
					doc.LinesChangedSinceAST[index] = true
				}
			}
		}
	}

	return s.cache.Put(doc)
}

func getDiffPosition(newText string, oldText string) int {
	for i := range newText {
		if i >= len(oldText) {
			return i
		}
		if i >= len(newText) {
			return i
		}
		if newText[i] != oldText[i] {
			return i
		}
	}
	return 0
}

func (s *Server) getFixedAst(filename string, newText string, oldText string) (ast.Node, error) {
	// TODO: Use treesitter and the lookahead iterator
	// Try new text without modification
	ast, err := jsonnet.SnippetToAST(filename, newText)
	if err == nil {
		return ast, nil
	}
	// TODO: make proper diff
	diffLocation := getDiffPosition(newText, oldText)
	lineEndingLocation := strings.Index(newText[diffLocation:], "\n") + diffLocation
	removeEndings := []rune{'.', ','}
	// Add ".a" to fix "super". This will insert an extra index, but it doesn't seem to break anything
	addEndings := []string{";", "),", ",", ")", "[]", "{}", ".a", "]", "],", `";`}

	// First remove all endings
	for len(newText) > 0 && lineEndingLocation > 0 && slices.Contains(removeEndings, rune(newText[lineEndingLocation-1])) {
		newText = newText[:lineEndingLocation-1] + newText[lineEndingLocation:]
		lineEndingLocation--
	}
	// Try to fix ast with all suffixes removed
	ast, err = jsonnet.SnippetToAST(filename, newText)
	if err == nil {
		return ast, nil
	}

	// Then add fixed endings
	for _, ending := range addEndings {
		// Ensure the line ends with ending
		testText := newText[:lineEndingLocation] + ending + newText[lineEndingLocation:]
		ast, err := jsonnet.SnippetToAST(filename, testText)
		if err == nil {
			log.Infof("Fixed ast with %v", ending)
			return ast, nil
		}
	}

	log.Warnf("Unable to fix ast!")
	return nil, fmt.Errorf("unable to fix ast")
}

func (s *Server) DidOpen(_ context.Context, params *protocol.DidOpenTextDocumentParams) (err error) {
	defer s.queueDiagnostics(params.TextDocument.URI)

	doc := &cache.Document{Item: params.TextDocument, LinesChangedSinceAST: map[int]bool{}}
	if params.TextDocument.Text != "" {
		doc.AST, doc.Err = jsonnet.SnippetToAST(params.TextDocument.URI.SpanURI().Filename(), params.TextDocument.Text)
	}
	return s.cache.Put(doc)
}

func (s *Server) DidClose(_ context.Context, params *protocol.DidCloseTextDocumentParams) error {
	s.cache.Remove(params.TextDocument.URI)
	return nil
}

func (s *Server) Initialize(_ context.Context, params *protocol.ParamInitialize) (*protocol.InitializeResult, error) {
	log.Infof("Initializing %s version %s", s.name, s.version)

	s.diagnosticsLoop()
	// TODO: this is probably not a JPath
	for _, folder := range params.WorkspaceFolders {
		s.configuration.JPaths = append(s.configuration.JPaths, folder.Name)
	}
	s.clientCapabilities = params.Capabilities

	var err error

	if s.stdlib == nil {
		log.Infoln("Reading stdlib")
		if s.stdlib, err = stdlib.Functions(); err != nil {
			return nil, err
		}
	}

	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			CompletionProvider:         protocol.CompletionOptions{TriggerCharacters: []string{".", "/"}},
			HoverProvider:              true,
			DefinitionProvider:         true,
			DocumentFormattingProvider: true,
			DocumentSymbolProvider:     true,
			ReferencesProvider:         true,
			ExecuteCommandProvider:     protocol.ExecuteCommandOptions{Commands: []string{}},
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				Change:    protocol.Full,
				OpenClose: true,
				Save: protocol.SaveOptions{
					IncludeText: false,
				},
			},
			RenameProvider: true,
			SignatureHelpProvider: protocol.SignatureHelpOptions{
				TriggerCharacters: []string{"(", ","},
			},
			InlayHintProvider: true,
			SemanticTokensProvider: protocol.SemanticTokensOptions{
				Range: false,
				Full:  true,
				Legend: protocol.SemanticTokensLegend{
					TokenTypes:     s.GetSemanticTokenTypes(),
					TokenModifiers: s.GetSemanticTokenModifiers(),
				},
			},
		},
		ServerInfo: protocol.PServerInfoMsg_initialize{
			Name:    s.name,
			Version: s.version,
		},
	}, nil
}

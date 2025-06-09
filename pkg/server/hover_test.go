package server

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/grafana/jsonnet-language-server/pkg/server/config"
	"github.com/grafana/jsonnet-language-server/pkg/stdlib"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	hoverTestStdlib = []stdlib.Function{
		{
			Name:                "thisFile",
			Params:              []string{},
			MarkdownDescription: "Note that this is a field. It contains the current Jsonnet filename as a string.",
		},
		{
			Name:                "objectFields",
			Params:              []string{"o"},
			MarkdownDescription: "Returns an array of strings, each element being a field from the given object. Does not include\nhidden fields.",
		},
		{
			Name:                "map",
			Params:              []string{"any"},
			MarkdownDescription: "desc",
		},
		{
			Name:                "manifestJson",
			Params:              []string{"any"},
			MarkdownDescription: "desc",
		},
	}
	expectedThisFileHover = &protocol.Hover{
		Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: "`std.thisFile`\n\nNote that this is a field. It contains the current Jsonnet filename as a string."},
		Range: protocol.Range{
			Start: protocol.Position{Line: 1, Character: 12},
			End:   protocol.Position{Line: 1, Character: 24},
		},
	}
	expectedObjectFieldsHover = &protocol.Hover{
		Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: "`std.objectFields(o)`\n\nReturns an array of strings, each element being a field from the given object. Does not include\nhidden fields."},
		Range: protocol.Range{
			Start: protocol.Position{Line: 2, Character: 10},
			End:   protocol.Position{Line: 2, Character: 26},
		},
	}
	expectedMapHover = &protocol.Hover{
		Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: "`std.map(any)`\n\ndesc"},
		Range: protocol.Range{
			Start: protocol.Position{Line: 5, Character: 17},
			End:   protocol.Position{Line: 5, Character: 24},
		},
	}
	expectedManifestJSON = &protocol.Hover{
		Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: "`std.manifestJson(any)`\n\ndesc"},
		Range: protocol.Range{
			Start: protocol.Position{Line: 7, Character: 71},
			End:   protocol.Position{Line: 7, Character: 87},
		},
	}
)

func TestHoverOnStdLib(t *testing.T) {
	logrus.SetOutput(io.Discard)

	var testCases = []struct {
		name        string
		document    string
		position    protocol.Position
		expected    *protocol.Hover
		expectedErr error
	}{
		{
			name:     "std.thisFile over std",
			document: "./testdata/hover-std.jsonnet",
			position: protocol.Position{Line: 1, Character: 14},
			expected: expectedThisFileHover,
		},
		{
			name:     "std.thisFile over std",
			document: "./testdata/hover-std.jsonnet",
			position: protocol.Position{Line: 1, Character: 19},
			expected: expectedThisFileHover,
		},
		{
			name:     "std.objectFields over std",
			document: "./testdata/hover-std.jsonnet",
			position: protocol.Position{Line: 2, Character: 12},
			expected: expectedObjectFieldsHover,
		},
		{
			name:     "std.objectFields over func name",
			document: "./testdata/hover-std.jsonnet",
			position: protocol.Position{Line: 2, Character: 22},
			expected: expectedObjectFieldsHover,
		},
		{
			name:     "std.map over std",
			document: "./testdata/hover-std.jsonnet",
			position: protocol.Position{Line: 5, Character: 19},
			expected: expectedMapHover,
		},
		{
			name:     "std.map over func name",
			document: "./testdata/hover-std.jsonnet",
			position: protocol.Position{Line: 5, Character: 23},
			expected: expectedMapHover,
		},
		{
			name:     "std.manifestJson over std",
			document: "./testdata/hover-std.jsonnet",
			position: protocol.Position{Line: 7, Character: 73},
			expected: expectedManifestJSON,
		},
		{
			name:     "std.manifestJson over func name",
			document: "./testdata/hover-std.jsonnet",
			position: protocol.Position{Line: 7, Character: 82},
			expected: expectedManifestJSON,
		},
		{
			name:     "list comprehension for",
			document: "./testdata/hover-std.jsonnet",
			position: protocol.Position{Line: 14, Character: 21},
			expected: &protocol.Hover{
				Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: "`std.objectFields(o)`\n\nReturns an array of strings, each element being a field from the given object. Does not include\nhidden fields."},
				Range: protocol.Range{
					Start: protocol.Position{Line: 14, Character: 7},
					End:   protocol.Position{Line: 14, Character: 23},
				},
			},
		},
		{
			name:     "list comprehension if",
			document: "./testdata/hover-std.jsonnet",
			position: protocol.Position{Line: 15, Character: 12},
			expected: &protocol.Hover{
				Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: "`std.map(any)`\n\ndesc"},
				Range: protocol.Range{
					Start: protocol.Position{Line: 15, Character: 7},
					End:   protocol.Position{Line: 15, Character: 14},
				},
			},
		},
		{
			name:     "map comprehension for",
			document: "./testdata/map-comprehension.jsonnet",
			position: protocol.Position{Line: 4, Character: 21},
			expected: &protocol.Hover{
				Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: "`std.objectFields(o)`\n\nReturns an array of strings, each element being a field from the given object. Does not include\nhidden fields."},
				Range: protocol.Range{
					Start: protocol.Position{Line: 4, Character: 7},
					End:   protocol.Position{Line: 4, Character: 23},
				},
			},
		},
		{
			name:     "map comprehension if",
			document: "./testdata/map-comprehension.jsonnet",
			position: protocol.Position{Line: 5, Character: 12},
			expected: &protocol.Hover{
				Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: "`std.map(any)`\n\ndesc"},
				Range: protocol.Range{
					Start: protocol.Position{Line: 5, Character: 7},
					End:   protocol.Position{Line: 5, Character: 14},
				},
			},
		},
		{
			name:     "local in local",
			document: "./testdata/hover-locals.jsonnet",
			position: protocol.Position{Line: 3, Character: 27},
			expected: &protocol.Hover{
				Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: "`std.objectFields(o)`\n\nReturns an array of strings, each element being a field from the given object. Does not include\nhidden fields."},
				Range: protocol.Range{
					Start: protocol.Position{Line: 3, Character: 13},
					End:   protocol.Position{Line: 3, Character: 29},
				},
			},
		},
		{
			name:     "local function call",
			document: "./testdata/hover-locals.jsonnet",
			position: protocol.Position{Line: 9, Character: 10},
			expected: &protocol.Hover{
				Contents: protocol.MarkupContent{Kind: protocol.Markdown, Value: "`std.objectFields(o)`\n\nReturns an array of strings, each element being a field from the given object. Does not include\nhidden fields."},
				Range: protocol.Range{
					Start: protocol.Position{Line: 9, Character: 2},
					End:   protocol.Position{Line: 9, Character: 18},
				},
			},
		},
		{
			// We don't want to crash the server if we get an error
			name:     "hover parsing error",
			document: "./testdata/hover-error.jsonnet",
			position: protocol.Position{Line: 0, Character: 0},
			expected: nil,
		},
		{
			name:     "no match on non-std",
			document: "./testdata/hover-std.jsonnet",
			position: protocol.Position{Line: 19, Character: 18},
			expected: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := testServer(t, hoverTestStdlib)
			uri := protocol.URIFromPath(tc.document)
			content, err := os.ReadFile(tc.document)
			require.NoError(t, err)
			err = server.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
				TextDocument: protocol.TextDocumentItem{
					URI:        uri,
					Text:       string(content),
					Version:    1,
					LanguageID: "jsonnet",
				},
			})
			require.NoError(t, err)

			result, err := server.Hover(context.TODO(), &protocol.HoverParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: uri},
					Position:     tc.position,
				},
			})
			if tc.expectedErr != nil {
				assert.EqualError(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestHover(t *testing.T) {
	logrus.SetOutput(io.Discard)

	testCases := []struct {
		name            string
		filename        string
		position        protocol.Position
		expectedContent protocol.Hover
	}{
		{
			name:     "hover on nested attribute",
			filename: "testdata/indexes.jsonnet",
			position: protocol.Position{Line: 9, Character: 16},
			expectedContent: protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.Markdown,
					Value: "```jsonnet\nbar: 'innerfoo'\n```\n",
				},
				Range: protocol.Range{
					Start: protocol.Position{Line: 9, Character: 5},
					End:   protocol.Position{Line: 9, Character: 18},
				},
			},
		},
		{
			name:     "hover on multi-line string",
			filename: "testdata/indexes.jsonnet",
			position: protocol.Position{Line: 8, Character: 9},
			expectedContent: protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.Markdown,
					Value: "```jsonnet\nobj = {\n  foo: {\n    bar: 'innerfoo',\n  },\n  bar: 'foo',\n}\n```\n",
				},
				Range: protocol.Range{
					Start: protocol.Position{Line: 8, Character: 8},
					End:   protocol.Position{Line: 8, Character: 11},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := &protocol.HoverParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{
						URI: protocol.URIFromPath(tc.filename),
					},
					Position: tc.position,
				},
			}

			server := NewServer("any", "test version", nil, config.Configuration{
				JPaths: []string{"testdata", filepath.Join(filepath.Dir(tc.filename), "vendor")},
			})
			serverOpenTestFile(t, server, tc.filename)
			response, err := server.Hover(context.Background(), params)

			require.NoError(t, err)
			assert.Equal(t, &tc.expectedContent, response)
		})
	}
}

func TestHoverGoToDefinitionTests(t *testing.T) {
	logrus.SetOutput(io.Discard)

	for _, tc := range definitionTestCases {
		t.Run(tc.name, func(t *testing.T) {
			params := &protocol.HoverParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{
						URI: protocol.URIFromPath(tc.filename),
					},
					Position: tc.position,
				},
			}

			server := NewServer("any", "test version", nil, config.Configuration{
				JPaths: []string{"testdata", filepath.Join(filepath.Dir(tc.filename), "vendor")},
			})
			serverOpenTestFile(t, server, tc.filename)
			response, err := server.Hover(context.Background(), params)

			// We only want to check that it found something. In combination with other tests, we can assume the content is OK.
			require.NoError(t, err)
			require.NotNil(t, response)
		})
	}
}

package server

import (
	_ "embed"
	"path/filepath"
	"testing"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type referenceResult struct {
	// Defaults to filename
	targetFilename string
	targetBegin    protocol.Position
	// Defaults to targetRange
	targetSelectionRange protocol.Range
}

type referenceTestCase struct {
	name     string
	filename string
	position protocol.Position

	identifier string
	results    []referenceResult
}

var libFile = "./testdata/reference/lib.libsonnet"
var mainFile = "./testdata/reference/main.jsonnet"
var referenceTestCases = []referenceTestCase{
	{
		name:     "local ref",
		filename: libFile,
		position: protocol.Position{
			Line:      0,
			Character: 6,
		},
		identifier: "test2",
		results: []referenceResult{
			{
				targetFilename: libFile,
				targetBegin:    protocol.Position{Line: 4, Character: 9},
			},
		},
	},
	{
		name:     "container ref",
		filename: libFile,
		position: protocol.Position{
			Line:      2,
			Character: 2,
		},
		identifier: "functions",
		results: []referenceResult{
			{
				targetFilename: mainFile,
				targetBegin:    protocol.Position{Line: 3, Character: 4},
			},
			{
				targetFilename: mainFile,
				targetBegin:    protocol.Position{Line: 6, Character: 4},
			},
		},
	},
	{
		name:     "function ref",
		filename: libFile,
		position: protocol.Position{
			Line:      3,
			Character: 4,
		},
		identifier: "coolFunc",
		results: []referenceResult{
			{
				targetFilename: mainFile,
				targetBegin:    protocol.Position{Line: 3, Character: 14},
			},
		},
	},
	{
		name:     "weird function ref",
		filename: libFile,
		position: protocol.Position{
			Line:      4,
			Character: 4,
		},
		identifier: "a",
		results: []referenceResult{{
			targetFilename: mainFile,
			targetBegin:    protocol.Position{Line: 8, Character: 4},
		},
		},
	},
	{
		name:     "single letter function ref",
		filename: libFile,
		position: protocol.Position{
			Line:      6,
			Character: 2,
		},
		identifier: "x",
		results: []referenceResult{{
			targetFilename: mainFile,
			targetBegin:    protocol.Position{Line: 9, Character: 4},
		},
		},
	},
	{
		name:     "argument ref",
		filename: libFile,
		position: protocol.Position{
			Line:      3,
			Character: 13,
		},
		identifier: "val",
		results: []referenceResult{{
			targetFilename: libFile,
			targetBegin:    protocol.Position{Line: 3, Character: 24},
		},
		},
	},
	{
		name:     "multi argument ref 1",
		filename: libFile,
		position: protocol.Position{
			Line:      7,
			Character: 12,
		},
		identifier: "argOne",
		results: []referenceResult{{
			targetFilename: libFile,
			targetBegin:    protocol.Position{Line: 8, Character: 4},
		},
		},
	},
	{
		name:     "multi argument ref 2 (default)",
		filename: libFile,
		position: protocol.Position{
			Line:      7,
			Character: 20,
		},
		identifier: "argTwo",
		results: []referenceResult{{
			targetFilename: libFile,
			targetBegin:    protocol.Position{Line: 9, Character: 4},
		},
		},
	},
	{
		name:     "multi argument ref 3",
		filename: libFile,
		position: protocol.Position{
			Line:      7,
			Character: 35,
		},
		identifier: "argThree",
		results: []referenceResult{{
			targetFilename: libFile,
			targetBegin:    protocol.Position{Line: 10, Character: 4},
		},
		},
	},
}

func TestReference(t *testing.T) {
	for _, tc := range referenceTestCases {
		t.Run(tc.name, func(t *testing.T) {
			params := &protocol.ReferenceParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{
						URI: protocol.URIFromPath(tc.filename),
					},
					Position: tc.position,
				},
			}

			server := NewServer("any", "test version", nil, Configuration{
				JPaths: []string{"testdata", filepath.Join(filepath.Dir(tc.filename), "vendor")},
			})
			serverOpenTestFile(t, server, tc.filename)
			identifier, err := server.getSelectedIdentifier(tc.filename, tc.position)
			require.NoError(t, err)
			assert.Equal(t, tc.identifier, identifier)
			response, err := server.findAllReferences(protocol.URIFromPath(tc.filename), params.Position, false)
			require.NoError(t, err)

			var expected []protocol.Location
			for _, r := range tc.results {
				if r.targetFilename == "" {
					r.targetFilename = tc.filename
				}
				//range := protocol.Range {
				//}
				computedRange := protocol.Range{
					Start: r.targetBegin,
					End: protocol.Position{
						Line:      r.targetBegin.Line,
						Character: r.targetBegin.Character + uint32(len(tc.identifier)),
					},
				}
				expected = append(expected, protocol.Location{
					URI:   protocol.URIFromPath(r.targetFilename),
					Range: computedRange,
				})
			}

			assert.Equal(t, expected, response)
		})
	}
}

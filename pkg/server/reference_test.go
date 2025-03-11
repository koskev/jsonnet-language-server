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
	targetRange    protocol.Range
	// Defaults to targetRange
	targetSelectionRange protocol.Range
}

type referenceTestCase struct {
	name     string
	filename string
	position protocol.Position

	results []referenceResult
}

var libFile = "./testdata/reference/lib.libsonnet"
var mainFile = "./testdata/reference/main.jsonnet"
var referenceTestCases = []referenceTestCase{
	{
		name:     "function ref",
		filename: libFile,
		position: protocol.Position{
			Line:      3,
			Character: 4,
		},
		results: []referenceResult{{
			targetFilename: mainFile,
			targetRange: protocol.Range{
				Start: protocol.Position{Line: 3, Character: 14},
				End:   protocol.Position{Line: 3, Character: 22},
			},
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
			response, err := server.findAllReferences(protocol.URIFromPath(tc.filename), params.Position, false)
			require.NoError(t, err)

			var expected []protocol.Location
			for _, r := range tc.results {
				// Defaults
				if r.targetSelectionRange.End.Character == 0 {
					r.targetSelectionRange = r.targetRange
				}
				if r.targetFilename == "" {
					r.targetFilename = tc.filename
				}
				expected = append(expected, protocol.Location{
					URI:   protocol.URIFromPath(r.targetFilename),
					Range: r.targetRange,
				})
			}

			assert.Equal(t, expected, response)
		})
	}
}

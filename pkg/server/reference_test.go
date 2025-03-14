package server

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/ast/processing"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type referenceResult struct {
	// Defaults to filename
	targetFilename string
	targetBegin    protocol.Position
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
		name:     "single letter member ref",
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
	{
		name:     "deep nested",
		filename: libFile,
		position: protocol.Position{
			Line:      19,
			Character: 10,
		},
		identifier: "test4",
		results: []referenceResult{{
			targetFilename: mainFile,
			targetBegin:    protocol.Position{Line: 10, Character: 47},
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

func checkPoints(t *testing.T, points map[ast.Location]bool, begin ast.Location, end ast.Location) {
	for loc, res := range points {
		not := ""
		if !res {
			not = "not"
		}
		assert.Equal(t, res, processing.InRange(loc, ast.LocationRange{Begin: begin, End: end}), fmt.Sprintf("%v should %s be in [%v|%v]", loc, not, begin, end))
	}
}

func TestInRangeSingleLine(t *testing.T) {
	begin := ast.Location{Line: 8, Column: 10}
	end := ast.Location{Line: 8, Column: 20}

	points := map[ast.Location]bool{
		ast.Location{Line: 8, Column: 11}: true,
		ast.Location{Line: 8, Column: 10}: true,
		ast.Location{Line: 8, Column: 9}:  false,
		ast.Location{Line: 8, Column: 20}: true,
		ast.Location{Line: 8, Column: 21}: false,
		ast.Location{Line: 7, Column: 15}: false,
	}
	checkPoints(t, points, begin, end)
}

func TestInRangeMultiLine(t *testing.T) {
	begin := ast.Location{Line: 5, Column: 10}
	end := ast.Location{Line: 10, Column: 20}

	points := map[ast.Location]bool{
		ast.Location{Line: 8, Column: 11}:  true,
		ast.Location{Line: 8, Column: 0}:   true,
		ast.Location{Line: 8, Column: 30}:  true,
		ast.Location{Line: 5, Column: 9}:   false,
		ast.Location{Line: 5, Column: 10}:  true,
		ast.Location{Line: 10, Column: 20}: true,
		ast.Location{Line: 10, Column: 21}: false,
	}
	checkPoints(t, points, begin, end)
}

func TestIdentifierVal(t *testing.T) {
	filename := "./testdata/reference/lib.libsonnet"
	server := NewServer("any", "test version", nil, Configuration{
		JPaths: []string{"testdata", filepath.Join(filepath.Dir(filename), "vendor")},
	})

	locations, err := server.findIdentifierLocations(filename, "val")
	require.NoError(t, err)

	expected := []ast.LocationRange{
		{
			FileName: filename,
			Begin:    ast.Location{Line: 4, Column: 14},
			End:      ast.Location{Line: 4, Column: 17},
		},
		{
			FileName: filename,
			Begin:    ast.Location{Line: 4, Column: 25},
			End:      ast.Location{Line: 4, Column: 28},
		},
	}

	assert.Equal(t, expected, locations)
}

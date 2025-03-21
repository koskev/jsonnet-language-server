package cst

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type CstExpected struct {
	nodeRange tree_sitter.Range
	nodeType  NodeType
	inject    bool
}

type CstNodeTestCase struct {
	name             string
	filename         string
	replaceString    string
	replaceByString  string
	completionOffset int
	lineOverride     int
	expected         CstExpected
}

var testCases = []CstNodeTestCase{
	{
		name:            "Simple object from dot",
		filename:        "./testdata/object.jsonnet",
		replaceString:   "myObj.key,",
		replaceByString: "myObj.",
		expected: CstExpected{
			inject:   true,
			nodeType: NodeID,
			nodeRange: tree_sitter.Range{
				StartPoint: tree_sitter.NewPoint(3, 2),
				EndPoint:   tree_sitter.NewPoint(3, 7),
			},
		},
	},
	{
		name:            "Simple object from val end",
		filename:        "./testdata/object.jsonnet",
		replaceString:   "myObj.key,",
		replaceByString: "myObj.k",
		expected: CstExpected{
			nodeType: NodeID,
			nodeRange: tree_sitter.Range{
				StartPoint: tree_sitter.NewPoint(3, 8),
				EndPoint:   tree_sitter.NewPoint(3, 9),
			},
		},
	},
	{
		name:             "Simple object from val dot",
		filename:         "./testdata/object.jsonnet",
		replaceString:    "myObj.key,",
		replaceByString:  "myObj.,",
		completionOffset: -1,
		expected: CstExpected{
			inject:   true,
			nodeType: NodeID,
			nodeRange: tree_sitter.Range{
				StartPoint: tree_sitter.NewPoint(3, 2),
				EndPoint:   tree_sitter.NewPoint(3, 7),
			},
		},
	},
	{
		name:             "Simple object from val comma",
		filename:         "./testdata/object.jsonnet",
		replaceString:    "myObj.key,",
		replaceByString:  "myObj.k,",
		completionOffset: -1,
		expected: CstExpected{
			nodeType: NodeID,
			nodeRange: tree_sitter.Range{
				StartPoint: tree_sitter.NewPoint(3, 8),
				EndPoint:   tree_sitter.NewPoint(3, 9),
			},
		},
	},
	//{
	//	name:            "Global from array",
	//	filename:        "./testdata/object.jsonnet",
	//	replaceString:   "myObj.key,",
	//	replaceByString: "",
	//	expected: CstExpected{
	//		inject:   true,
	//		nodeType: NodeID,
	//		nodeRange: tree_sitter.Range{
	//			StartPoint: tree_sitter.NewPoint(3, 2),
	//			EndPoint:   tree_sitter.NewPoint(3, 7),
	//		},
	//	},
	//},
	{
		name:            "Simple object from func",
		filename:        "./testdata/func.jsonnet",
		replaceString:   "myFunc(1).key,",
		replaceByString: "myFunc(1).",
		expected: CstExpected{
			inject:   true,
			nodeType: NodeFunctionCall,
			nodeRange: tree_sitter.Range{
				StartPoint: tree_sitter.NewPoint(2, 2),
				EndPoint:   tree_sitter.NewPoint(2, 11),
			},
		},
	},
	//{
	//	name:            "Object function from dot",
	//	filename:        "./testdata/nested_object_func.jsonnet",
	//	replaceString:   "myObj.objFunc(5).funcKey",
	//	replaceByString: "myObj.objFunc(5).",
	//	expected: CstExpected{
	//		nodeType: NodeID,
	//		nodeRange: tree_sitter.Range{
	//			StartPoint: tree_sitter.NewPoint(3, 2),
	//			EndPoint:   tree_sitter.NewPoint(3, 7),
	//		},
	//	},
	//},
}

func TestCSTNode(t *testing.T) {
	ctx := context.Background()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := os.ReadFile(tc.filename)
			require.NoError(t, err)

			replacedContent := strings.ReplaceAll(string(content), tc.replaceString, tc.replaceByString)

			cursorPosition := protocol.Position{}
			for _, line := range strings.Split(replacedContent, "\n") {
				if strings.Contains(line, tc.replaceByString) {
					cursorPosition.Character = uint32(strings.Index(line, tc.replaceByString) + len(tc.replaceByString))
					break
				}
				cursorPosition.Line++
			}
			if tc.lineOverride != 0 {
				cursorPosition.Line = uint32(tc.lineOverride)
			}
			// This is worse than rust...
			cursorPosition.Character = min(uint32(int64(cursorPosition.Character)+int64(tc.completionOffset)), cursorPosition.Character)
			//if cursorPosition.Character == 0 {
			//	t.Fatal(fmt.Sprintf("Could not find cursor position for test. Replace probably didn't work: %+v", cursorPosition))
			//}

			require.NoError(t, err)
			found, err := FindCompletionNode(ctx, replacedContent, cursorPosition)
			require.NoError(t, err)
			require.NotNil(t, found.Node)
			assert.Equal(t, string(tc.expected.nodeType), found.Node.GrammarName())
			assert.Equal(t, tc.expected.nodeRange.StartPoint, found.Node.Range().StartPoint)
			//assert.Equal(t, tc.expected.nodeRange.EndPoint, found.node.Range().EndPoint)
		})
	}
}

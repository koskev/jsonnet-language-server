package server

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndex(t *testing.T) {
	var testCases = []struct {
		name                           string
		filename                       string
		replaceString, replaceByString string
		expected                       uint32
		completionOffset               int
		disabled                       bool
	}{
		{
			name:             "first argument",
			filename:         "./testdata/signature/functioncall.jsonnet",
			replaceString:    "a: myFunc(1, 2, 3),",
			replaceByString:  "a: myFunc(),",
			completionOffset: -2,
			expected:         0,
		},
		{
			name:             "second argument",
			filename:         "./testdata/signature/functioncall.jsonnet",
			replaceString:    "a: myFunc(1, 2, 3),",
			replaceByString:  "a: myFunc(1,),",
			completionOffset: -2,
			expected:         1,
		},
		{
			name:             "third argument",
			filename:         "./testdata/signature/functioncall.jsonnet",
			replaceString:    "a: myFunc(1, 2, 3),",
			replaceByString:  "a: myFunc(1, 23, ),",
			completionOffset: -2,
			expected:         2,
		},
		{
			name:             "completed second argument",
			filename:         "./testdata/signature/functioncall.jsonnet",
			replaceString:    "a: myFunc(1, 2, 3),",
			replaceByString:  "a: myFunc(1, , 3),",
			completionOffset: -6,
			expected:         1,
		},
		{
			name:             "multiline first",
			filename:         "./testdata/signature/functioncall.jsonnet",
			replaceString:    "1,  // First",
			replaceByString:  ", // X",
			completionOffset: -5,
			expected:         0,
			disabled:         true,
		},
		{
			name:             "multiline second",
			filename:         "./testdata/signature/functioncall.jsonnet",
			replaceString:    "2,  // Second",
			replaceByString:  ", // X",
			completionOffset: -5,
			expected:         1,
		},
		{
			name:             "multiline third",
			filename:         "./testdata/signature/functioncall.jsonnet",
			replaceString:    "3,  // Third",
			replaceByString:  ", // X",
			completionOffset: -5,
			expected:         2,
			disabled:         true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := os.ReadFile(tc.filename)
			require.NoError(t, err)

			server, fileURI := testServerWithFile(t, completionTestStdlib, string(content))
			server.configuration.JPaths = []string{"testdata", "testdata/complete", "testdata/complete/import"}
			server.configuration.ExtCode = map[string]string{
				"code": "{ objA: 5, ['%s' % 'computed']: 3}",
			}
			var version int32 = 2

			replacedContent := strings.ReplaceAll(string(content), tc.replaceString, tc.replaceByString)

			updateText(t, server, replacedContent, fileURI, version)

			cursorPosition := protocol.Position{}
			for _, line := range strings.Split(replacedContent, "\n") {
				if strings.Contains(line, tc.replaceByString) {
					cursorPosition.Character = uint32(strings.Index(line, tc.replaceByString) + len(tc.replaceByString))
					break
				}
				cursorPosition.Line++
			}
			// This is worse than rust...
			cursorPosition.Character = min(uint32(int64(cursorPosition.Character)+int64(tc.completionOffset)), cursorPosition.Character)
			if cursorPosition.Character == 0 {
				t.Fatal("Could not find cursor position for test. Replace probably didn't work")
			}

			result, err := server.SignatureHelp(context.Background(), &protocol.SignatureHelpParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: fileURI},
					Position:     cursorPosition,
				},
			})
			require.NoError(t, err)

			if tc.disabled {
				t.Skip()
			} else {
				assert.Equal(t, tc.expected, result.ActiveParameter)
			}
		})
	}
}

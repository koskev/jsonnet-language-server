package server

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/grafana/jsonnet-language-server/pkg/stdlib"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	completionTestStdlib = []stdlib.Function{
		// Starts with aaa to be the first match
		// A `min` subquery should matche this and `min`, but `min` should be first anyways
		{
			Name:                "aaaotherMin",
			Params:              []string{"a"},
			MarkdownDescription: "blabla",
		},
		{
			Name:                "max",
			Params:              []string{"a", "b"},
			MarkdownDescription: "max gets the max",
		},
		{
			Name:                "min",
			Params:              []string{"a", "b"},
			MarkdownDescription: "min gets the min",
		},
	}

	otherMinItem = protocol.CompletionItem{
		Label:         "aaaotherMin",
		Kind:          protocol.FunctionCompletion,
		Detail:        "std.aaaotherMin(a)",
		InsertText:    "aaaotherMin(a)",
		Documentation: "blabla",
	}
	minItem = protocol.CompletionItem{
		Label:         "min",
		Kind:          protocol.FunctionCompletion,
		Detail:        "std.min(a, b)",
		InsertText:    "min(a, b)",
		Documentation: "min gets the min",
	}
	maxItem = protocol.CompletionItem{
		Label:         "max",
		Kind:          protocol.FunctionCompletion,
		Detail:        "std.max(a, b)",
		InsertText:    "max(a, b)",
		Documentation: "max gets the max",
	}
)

func TestCompletionStdLib(t *testing.T) {
	var testCases = []struct {
		name        string
		line        string
		expected    *protocol.CompletionList
		expectedErr error
	}{
		{
			name: "std: no suggestion 1",
			line: "no_std1: d",
		},
		{
			name: "std: no suggestion 2",
			line: "no_std2: s",
		},
		{
			name: "std: no suggestion 3",
			line: "no_std3: d.",
		},
		{
			name: "std: no suggestion 4",
			line: "no_std4: s.",
		},
		{
			name: "std: all functions",
			line: "all_std_funcs: std.",
			expected: &protocol.CompletionList{
				Items:        []protocol.CompletionItem{otherMinItem, maxItem, minItem},
				IsIncomplete: false,
			},
		},
		{
			name: "std: starting with aaa",
			line: "std_funcs_starting_with: std.aaa",
			expected: &protocol.CompletionList{
				Items:        []protocol.CompletionItem{otherMinItem},
				IsIncomplete: false,
			},
		},
		{
			name: "std: partial match",
			line: "partial_match: std.ther",
			expected: &protocol.CompletionList{
				Items:        []protocol.CompletionItem{otherMinItem},
				IsIncomplete: false,
			},
		},
		{
			name: "std: case insensitive",
			line: "case_insensitive: std.MAX",
			expected: &protocol.CompletionList{
				Items:        []protocol.CompletionItem{maxItem},
				IsIncomplete: false,
			},
		},
		{
			name: "std: submatch + startswith",
			line: "submatch_and_startwith: std.Min",
			expected: &protocol.CompletionList{
				Items:        []protocol.CompletionItem{minItem, otherMinItem},
				IsIncomplete: false,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			document := fmt.Sprintf("{ %s }", tc.line)

			server, fileURI := testServerWithFile(t, completionTestStdlib, document)

			result, err := server.Completion(context.TODO(), &protocol.CompletionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: fileURI},
					Position:     protocol.Position{Line: 0, Character: uint32(len(tc.line) + 2)},
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

func TestCompletion(t *testing.T) {
	var testCases = []struct {
		name                           string
		filename                       string
		replaceString, replaceByString string
		expected                       protocol.CompletionList
		completionOffset               int
		lineOverride                   int
	}{
		{
			name:            "self function",
			filename:        "testdata/test_basic_lib.libsonnet",
			replaceString:   "self.greet('Zack')",
			replaceByString: "self.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "greet",
						Kind:       protocol.FunctionCompletion,
						Detail:     "self.greet(name)",
						InsertText: "greet(name)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					// TODO: do we want to complete this?
					{
						Label:      "message",
						Kind:       protocol.VariableCompletion,
						Detail:     "message",
						InsertText: "message",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "*ast.Self",
						},
					},
				},
			},
		},
		{
			name:            "self function with bad first letter letter",
			filename:        "testdata/test_basic_lib.libsonnet",
			replaceString:   "self.greet('Zack')",
			replaceByString: "self.h",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items:        []protocol.CompletionItem{},
			},
		},
		{
			name:            "self function with first letter",
			filename:        "testdata/test_basic_lib.libsonnet",
			replaceString:   "self.greet('Zack')",
			replaceByString: "self.g",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{{
					Label:      "greet",
					Kind:       protocol.FunctionCompletion,
					Detail:     "self.greet(name)",
					InsertText: "greet(name)",
					LabelDetails: &protocol.CompletionItemLabelDetails{
						Description: "function",
					},
				}},
			},
		},
		{
			name:            "autocomplete through binary",
			filename:        "testdata/basic-object.jsonnet",
			replaceString:   "bar: 'foo',",
			replaceByString: "bar: self.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					// TODO: remove this test as it leads to an endless loop
					{
						Label:      "bar",
						Kind:       protocol.FieldCompletion,
						Detail:     "self.bar",
						InsertText: "bar",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "*ast.Self",
						},
					},
					{
						Label:      "foo",
						Kind:       protocol.FieldCompletion,
						Detail:     "self.foo",
						InsertText: "foo",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
				},
			},
		},
		{
			name:            "autocomplete locals",
			filename:        "testdata/basic-object.jsonnet",
			replaceString:   "bar: 'foo',",
			replaceByString: "bar: ",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "somevar2",
						Kind:       protocol.VariableCompletion,
						Detail:     "somevar2",
						InsertText: "somevar2",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "somevar",
						Kind:       protocol.VariableCompletion,
						Detail:     "somevar",
						InsertText: "somevar",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
				},
			},
		},
		{
			name:            "autocomplete locals: good prefix",
			filename:        "testdata/basic-object.jsonnet",
			replaceString:   "bar: 'foo',",
			replaceByString: "bar: some",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "somevar2",
						Kind:       protocol.VariableCompletion,
						Detail:     "somevar2",
						InsertText: "somevar2",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "somevar",
						Kind:       protocol.VariableCompletion,
						Detail:     "somevar",
						InsertText: "somevar",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
				},
			},
		},
		{
			name:            "autocomplete locals: bad prefix",
			filename:        "testdata/basic-object.jsonnet",
			replaceString:   "bar: 'foo',",
			replaceByString: "bar: bad",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items:        []protocol.CompletionItem{},
			},
		},
		{
			name:            "autocomplete through import",
			filename:        "testdata/imported-file.jsonnet",
			replaceString:   "b: otherfile.bar,",
			replaceByString: "b: otherfile.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "bar",
						Kind:       protocol.FieldCompletion,
						Detail:     "otherfile.bar",
						InsertText: "bar",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "foo",
						Kind:       protocol.FieldCompletion,
						Detail:     "otherfile.foo",
						InsertText: "foo",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
				},
			},
		},
		{
			name:            "autocomplete through import with prefix",
			filename:        "testdata/imported-file.jsonnet",
			replaceString:   "b: otherfile.bar,",
			replaceByString: "b: otherfile.b",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "bar",
						Kind:       protocol.FieldCompletion,
						Detail:     "otherfile.bar",
						InsertText: "bar",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
				},
			},
		},
		{
			name:            "autocomplete dollar sign",
			filename:        "testdata/dollar-simple.jsonnet",
			replaceString:   "test: $.attribute,",
			replaceByString: "test: $.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "attribute",
						Kind:       protocol.FieldCompletion,
						Detail:     "$.attribute",
						InsertText: "attribute",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
					{
						Label:      "attribute2",
						Kind:       protocol.FieldCompletion,
						Detail:     "$.attribute2",
						InsertText: "attribute2",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:             "autocomplete dollar sign, end with comma",
			filename:         "testdata/dollar-simple.jsonnet",
			replaceString:    "test: $.attribute,",
			replaceByString:  "test: $.,",
			completionOffset: -1, // Start at "." and not at ","
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "attribute",
						Kind:       protocol.FieldCompletion,
						Detail:     "$.attribute",
						InsertText: "attribute",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
					{
						Label:      "attribute2",
						Kind:       protocol.FieldCompletion,
						Detail:     "$.attribute2",
						InsertText: "attribute2",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:            "autocomplete nested imported file",
			filename:        "testdata/nested-imported-file.jsonnet",
			replaceString:   "foo: file.foo,",
			replaceByString: "foo: file.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "bar",
						Kind:       protocol.FieldCompletion,
						Detail:     "file.bar",
						InsertText: "bar",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "foo",
						Kind:       protocol.FieldCompletion,
						Detail:     "file.foo",
						InsertText: "foo",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
				},
			},
		},
		{
			name:            "autocomplete multiple fields within local",
			filename:        "testdata/indexes.jsonnet",
			replaceString:   "attr: obj.foo",
			replaceByString: "attr: obj.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "bar",
						Kind:       protocol.FieldCompletion,
						Detail:     "obj.bar",
						InsertText: "bar",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "foo",
						Kind:       protocol.FieldCompletion,
						Detail:     "obj.foo",
						InsertText: "foo",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:            "autocomplete local at root",
			filename:        "testdata/local-at-root.jsonnet",
			replaceString:   "hello.hello",
			replaceByString: "hello.hel",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "hel",
						Kind:       protocol.FieldCompletion,
						Detail:     "hello.hel",
						InsertText: "hel",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
					{
						Label:      "hello",
						Kind:       protocol.FieldCompletion,
						Detail:     "hello.hello",
						InsertText: "hello",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			// This checks that we don't match on `hello.hello.*` if we autocomplete on `hello.hel.`
			name:            "autocomplete local at root, no partial match if full match exists",
			filename:        "testdata/local-at-root.jsonnet",
			replaceString:   "hello.hello",
			replaceByString: "hello.hel.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "wel",
						Kind:       protocol.FieldCompletion,
						Detail:     "hello.hel.wel",
						InsertText: "wel",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
				},
			},
		},
		{
			// This checks that we don't match anything on `hello.hell.*`
			name:            "autocomplete local at root, no match on unknown field",
			filename:        "testdata/local-at-root.jsonnet",
			replaceString:   "hello.hello",
			replaceByString: "hello.hell.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items:        []protocol.CompletionItem{},
			},
		},
		{
			name:            "autocomplete local at root 2",
			filename:        "testdata/local-at-root-2.jsonnet",
			replaceString:   "hello.to",
			replaceByString: "hello.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "to",
						Kind:       protocol.FieldCompletion,
						Detail:     "hello.to",
						InsertText: "to",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:            "autocomplete local at root 2, nested",
			filename:        "testdata/local-at-root-2.jsonnet",
			replaceString:   "hello.to",
			replaceByString: "hello.to.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "the",
						Kind:       protocol.FieldCompletion,
						Detail:     "hello.to.the",
						InsertText: "the",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:            "autocomplete local at root 3, import chain",
			filename:        "testdata/local-at-root-3.jsonnet",
			replaceString:   "hello2.the",
			replaceByString: "hello2.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "the",
						Kind:       protocol.FieldCompletion,
						Detail:     "hello2.the",
						InsertText: "the",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:            "autocomplete local at root 4, import chain",
			filename:        "testdata/local-at-root-4.jsonnet",
			replaceString:   "hello3.world",
			replaceByString: "hello3.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "world",
						Kind:       protocol.FieldCompletion,
						Detail:     "hello3.world",
						InsertText: "world",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
				},
			},
		},
		{
			name:            "autocomplete fix doubled index bug",
			filename:        "testdata/doubled-index-bug-4.jsonnet",
			replaceString:   "a: g.hello",
			replaceByString: "a: g.hello.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "to",
						Kind:       protocol.FieldCompletion,
						Detail:     "g.hello.to",
						InsertText: "to",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:            "quote label",
			filename:        "testdata/quote_label.jsonnet",
			replaceString:   "lib",
			replaceByString: "lib.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "1num",
						Kind:       protocol.FieldCompletion,
						Detail:     "lib['1num']",
						InsertText: "['1num']",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
						// TODO: is 10 correct?
						TextEdit: &protocol.TextEdit{
							Range: protocol.Range{
								Start: protocol.Position{
									Line:      0,
									Character: 10,
								},
								End: protocol.Position{
									Line:      0,
									Character: 10,
								},
							},
							NewText: "['1num']",
						},
					},
					{
						Label:      "abc#func",
						Kind:       protocol.FunctionCompletion,
						Detail:     "lib['abc#func'](param)",
						InsertText: "['abc#func'](param)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
						TextEdit: &protocol.TextEdit{
							Range: protocol.Range{
								Start: protocol.Position{
									Line:      0,
									Character: 10,
								},
								End: protocol.Position{
									Line:      0,
									Character: 10,
								},
							},
							NewText: "['abc#func'](param)",
						},
					},
					{
						Label:      "abc#var",
						Kind:       protocol.FieldCompletion,
						Detail:     "lib['abc#var']",
						InsertText: "['abc#var']",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
						TextEdit: &protocol.TextEdit{
							Range: protocol.Range{
								Start: protocol.Position{
									Line:      0,
									Character: 10,
								},
								End: protocol.Position{
									Line:      0,
									Character: 10,
								},
							},
							NewText: "['abc#var']",
						},
					},
				},
			},
		},
		{
			name:            "complete attribute from function",
			filename:        "testdata/functions.libsonnet",
			replaceString:   "test: myfunc(arg1, arg2)",
			replaceByString: "test: myfunc(arg1, arg2).",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "atb1",
						Kind:       protocol.FieldCompletion,
						Detail:     "myfunc(arg1, arg2).atb1",
						InsertText: "atb1",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "atb2",
						Kind:       protocol.FieldCompletion,
						Detail:     "myfunc(arg1, arg2).atb2",
						InsertText: "atb2",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
				},
			},
		},
		{
			name:            "completion in function arguments",
			filename:        "testdata/functions.libsonnet",
			replaceString:   "test: myfunc(arg1, arg2)",
			replaceByString: "test: myfunc(arg1, self.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "a",
						Kind:       protocol.FieldCompletion,
						Detail:     "self.a",
						InsertText: "a",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "b",
						Kind:       protocol.FieldCompletion,
						Detail:     "self.b",
						InsertText: "b",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "c",
						Kind:       protocol.FieldCompletion,
						Detail:     "self.c",
						InsertText: "c",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "test",
						Kind:       protocol.FieldCompletion,
						Detail:     "self.test",
						InsertText: "test",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "*ast.Apply",
						},
					},
				},
			},
		},
		{
			name:            "completion in named function arguments",
			filename:        "./testdata/complete/functionargs.jsonnet",
			replaceString:   "a: localfunc(arg=data),",
			replaceByString: "a: localfunc(arg=d",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "data",
						Kind:       protocol.VariableCompletion,
						Detail:     "data",
						InsertText: "data",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:            "completion in named object arguments",
			filename:        "./testdata/complete/functionargs.jsonnet",
			replaceString:   "a: localfunc(arg=data),",
			replaceByString: "a: d",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "data",
						Kind:       protocol.VariableCompletion,
						Detail:     "data",
						InsertText: "data",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:            "completion of argument",
			filename:        "./testdata/complete/functionargs.jsonnet",
			replaceString:   "arg.coolkey,",
			replaceByString: "a",
			lineOverride:    3,
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "arg",
						Kind:       protocol.VariableCompletion,
						Detail:     "arg",
						InsertText: "arg",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
				},
			},
		},
		{
			name:            "completion of argument default value",
			filename:        "./testdata/complete/functionargs.jsonnet",
			replaceString:   "arg.coolkey,",
			replaceByString: "arg.",
			lineOverride:    3,
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "coolkey",
						Kind:       protocol.FieldCompletion,
						Detail:     "arg.coolkey",
						InsertText: "coolkey",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
				},
			},
		},
		{
			name:            "completion for extcode",
			filename:        "./testdata/complete/extcode.jsonnet",
			replaceString:   "extcode.objA,",
			replaceByString: "extcode.o",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "objA",
						Kind:       protocol.FieldCompletion,
						Detail:     "extcode.objA",
						InsertText: "objA",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "completion for extcode computed",
			filename:        "./testdata/complete/extcode.jsonnet",
			replaceString:   "extcode.objA,",
			replaceByString: "extcode.c",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "computed",
						Kind:       protocol.FieldCompletion,
						Detail:     "extcode.computed",
						InsertText: "computed",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "completion for binary object",
			filename:        "./testdata/complete/binaryobject.jsonnet",
			replaceString:   "a: binaryObject.one,",
			replaceByString: "a: binaryObject.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "one",
						Kind:       protocol.FieldCompletion,
						Detail:     "binaryObject.one",
						InsertText: "one",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
					{
						Label:      "two",
						Kind:       protocol.FieldCompletion,
						Detail:     "binaryObject.two",
						InsertText: "two",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
				},
			},
		},
		{
			name:            "completion for in object hardcoded",
			filename:        "./testdata/complete/forobj.jsonnet",
			replaceString:   "a: forObj.one,",
			replaceByString: "a: forObj.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "one",
						Kind:       protocol.FieldCompletion,
						Detail:     "forObj.one",
						InsertText: "one",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "two",
						Kind:       protocol.FieldCompletion,
						Detail:     "forObj.two",
						InsertText: "two",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
				},
			},
		},
		{
			name:            "completion with arg value",
			filename:        "./testdata/complete/functionbody.jsonnet",
			replaceString:   "a: myFunc(exampleArg).field,",
			replaceByString: "a: myFunc(exampleArg).field.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "argField",
						Kind:       protocol.FieldCompletion,
						Detail:     "argField",
						InsertText: "argField",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "binary with override",
			filename:        "./testdata/complete/binaryobject_override.jsonnet",
			replaceString:   "a: binaryObject.one,",
			replaceByString: "a: binaryObject.override.f",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "field",
						Kind:       protocol.FieldCompletion,
						InsertText: "field",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "function import call 1",
			filename:        "./testdata/complete/import/functionimport.jsonnet",
			replaceString:   "key1: funcArg.key1.number1,",
			replaceByString: "key1: funcArg.key1.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "number1",
						Kind:       protocol.FieldCompletion,
						InsertText: "number1",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "function import call 2",
			filename:        "./testdata/complete/import/functionimport.jsonnet",
			replaceString:   "key1: funcArg.key1.number1,",
			replaceByString: "key1: funcArg.key2.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "number2",
						Kind:       protocol.FieldCompletion,
						InsertText: "number2",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "function call body global complete",
			filename:        "./testdata/complete/function/funcbody.libsonnet",
			replaceString:   "{}",
			replaceByString: "{} + ",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "functionarg",
						Kind:       protocol.FieldCompletion,
						InsertText: "functionarg",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "var",
						Kind:       protocol.FieldCompletion,
						InsertText: "var",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:            "function call body global complete without space",
			filename:        "./testdata/complete/function/funcbody.libsonnet",
			replaceString:   "{}",
			replaceByString: "{}+",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "functionarg",
						Kind:       protocol.FieldCompletion,
						InsertText: "functionarg",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "var",
						Kind:       protocol.FieldCompletion,
						InsertText: "var",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:            "function call body var body",
			filename:        "./testdata/complete/function/funcbody.libsonnet",
			replaceString:   "{}",
			replaceByString: "{} + var.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "varKey",
						Kind:       protocol.FieldCompletion,
						InsertText: "varKey",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "nested object with function calls",
			filename:        "./testdata/complete/functionargsnested.jsonnet",
			replaceString:   "a: localfunc(arg=data).funcBody.nested1.nested2Func().nested3.nested4Func(data2).nested5.nested2data.nested2data2,",
			replaceByString: "a: localfunc(arg=data).funcBody.nested1.nested2Func().nested3.nested4Func(data2).nested5.nested2data.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "nested2data2",
						Kind:       protocol.FieldCompletion,
						InsertText: "nested2data2",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
				},
			},
		},
		{
			name:            "conditional from local",
			filename:        "./testdata/complete/conditional.jsonnet",
			replaceString:   "a: myCondLocal(),",
			replaceByString: "a: myCondLocal().",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "base",
						Kind:       protocol.FieldCompletion,
						InsertText: "base",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
					{
						Label:      "trueField",
						Kind:       protocol.FieldCompletion,
						InsertText: "trueField",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "boolean",
						},
					},
				},
			},
		},
		// --- Builder Pattern ---
		{
			name:            "builder pattern simple",
			filename:        "./testdata/builder-pattern.jsonnet",
			replaceString:   "test: self.util.new().withAttr('hello').withAttr2('world').build(),",
			replaceByString: "test: self.util.new().withAttr('hello').b",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "build",
						Kind:       protocol.FieldCompletion,
						InsertText: "build()",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
				},
			},
		},
		{
			name:            "builder pattern this",
			filename:        "./testdata/builder-pattern.jsonnet",
			replaceString:   "test: self.util.new().withAttr('hello').withAttr2('world').build(),",
			replaceByString: "test: self.util.new().withAttr2('hello').b",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "build",
						Kind:       protocol.FieldCompletion,
						InsertText: "build()",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
				},
			},
		},
		{
			name:            "builder pattern chained",
			filename:        "./testdata/builder-pattern.jsonnet",
			replaceString:   "test: self.util.new().withAttr('hello').withAttr2('world').build(),",
			replaceByString: "test: self.util.new().withAttr('hello').withAttr('world').b",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "build",
						Kind:       protocol.FieldCompletion,
						InsertText: "build()",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
				},
			},
		},
		{
			name:            "builder pattern argument",
			filename:        "./testdata/builder-pattern.jsonnet",
			replaceString:   "test: self.util.new().withAttr('hello').withAttr2('world').build(),",
			replaceByString: "test: self.util.new().withAttr({myKey: 5}).attr.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "myKey",
						Kind:       protocol.FieldCompletion,
						InsertText: "myKey",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
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

			replacedContent := strings.ReplaceAll(string(content), tc.replaceString, tc.replaceByString)

			err = server.DidChange(context.Background(), &protocol.DidChangeTextDocumentParams{
				ContentChanges: []protocol.TextDocumentContentChangeEvent{{
					Text: replacedContent,
				}},
				TextDocument: protocol.VersionedTextDocumentIdentifier{
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: fileURI},
					Version:                2,
				},
			})
			require.NoError(t, err)

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
			if cursorPosition.Character == 0 {
				t.Fatal("Could not find cursor position for test. Replace probably didn't work")
			}

			result, err := server.Completion(context.TODO(), &protocol.CompletionParams{
				TextDocumentPositionParams: protocol.TextDocumentPositionParams{
					TextDocument: protocol.TextDocumentIdentifier{URI: fileURI},
					Position:     cursorPosition,
				},
			})
			require.NoError(t, err)
			// Testing details makes it practically impossible to change these
			for i := range result.Items {
				result.Items[i].Detail = ""
				result.Items[i].Kind = protocol.VariableCompletion
			}
			for i := range tc.expected.Items {
				tc.expected.Items[i].Detail = ""
				// TODO: Remove this
				tc.expected.Items[i].Kind = protocol.VariableCompletion
			}
			assert.Equal(t, tc.expected, *result, "position", cursorPosition, "file", tc.filename)
		})
	}
}

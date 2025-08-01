package server

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/grafana/jsonnet-language-server/pkg/server/config"
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
	testCases := []struct {
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

type completionCase struct {
	name                           string
	filename                       string
	replaceString, replaceByString string
	expected, unexpected           protocol.CompletionList
	completionOffset               int
	lineOverride                   int
	disable                        bool
	onlyCheckIfPresent             bool
}

func TestCompletion(t *testing.T) {
	testCases := []completionCase{
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
							Description: "self",
						},
					},
				},
			},
		},
		{
			name:            "self function with first letter",
			filename:        "testdata/test_basic_lib.libsonnet",
			replaceString:   "self.greet('Zack')",
			replaceByString: "self.g",
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
					{
						Label:      "message",
						Kind:       protocol.FunctionCompletion,
						Detail:     "self.greet(name)",
						InsertText: "message",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object field",
						},
					},
				},
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
							Description: "self",
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
			name:            "self completion in function arguments",
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
							Description: "apply",
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
			lineOverride:    5,
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
			lineOverride:    5,
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
			name:            "completion of named argument with object",
			filename:        "./testdata/complete/functionargs.jsonnet",
			replaceString:   "a: localfunc(arg=data),",
			replaceByString: "a: localfunc(arg=data.",
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
			name:            "completion of unnamed argument with object",
			filename:        "./testdata/complete/functionargs.jsonnet",
			replaceString:   "a: localfunc(arg=data),",
			replaceByString: "a: localfunc(data.",
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
					{
						Label:      "objA",
						Kind:       protocol.FieldCompletion,
						Detail:     "objA",
						InsertText: "objA",
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
			name:            "completion for in object var",
			filename:        "./testdata/complete/forobj.jsonnet",
			replaceString:   "a: forObj.one,",
			replaceByString: "a: forVar.",
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
		{
			name:            "conditional from object inline",
			filename:        "./testdata/complete/conditional.jsonnet",
			replaceString:   "a: myCondLocal(),",
			replaceByString: "a: conditionalObject.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "field",
						Kind:       protocol.FieldCompletion,
						InsertText: "field",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "inlineConditional",
						Kind:       protocol.FieldCompletion,
						InsertText: "inlineConditional",
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
						Label:      "attr",
						Kind:       protocol.FieldCompletion,
						InsertText: "attr",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "attr2",
						Kind:       protocol.FieldCompletion,
						InsertText: "attr2",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "build",
						Kind:       protocol.FieldCompletion,
						InsertText: "build()",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "withAttr",
						Kind:       protocol.FieldCompletion,
						InsertText: "withAttr(v)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "withAttr2",
						Kind:       protocol.FieldCompletion,
						InsertText: "withAttr2(v)",
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
						Label:      "attr",
						Kind:       protocol.FieldCompletion,
						InsertText: "attr",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "attr2",
						Kind:       protocol.FieldCompletion,
						InsertText: "attr2",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "build",
						Kind:       protocol.FieldCompletion,
						InsertText: "build()",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "withAttr",
						Kind:       protocol.FieldCompletion,
						InsertText: "withAttr(v)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "withAttr2",
						Kind:       protocol.FieldCompletion,
						InsertText: "withAttr2(v)",
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
						Label:      "attr",
						Kind:       protocol.FieldCompletion,
						InsertText: "attr",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "attr2",
						Kind:       protocol.FieldCompletion,
						InsertText: "attr2",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "build",
						Kind:       protocol.FieldCompletion,
						InsertText: "build()",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "withAttr",
						Kind:       protocol.FieldCompletion,
						InsertText: "withAttr(v)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "withAttr2",
						Kind:       protocol.FieldCompletion,
						InsertText: "withAttr2(v)",
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
		{
			name:            "builder pattern argument second this",
			filename:        "./testdata/builder-pattern.jsonnet",
			replaceString:   "test: self.util.new().withAttr('hello').withAttr2('world').build(),",
			replaceByString: "test: self.util.new().withAttr({myKey: 5}).withAttr2('world').attr.",
			disable:         true,
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
		{
			name:            "builder pattern argument second same",
			filename:        "./testdata/builder-pattern.jsonnet",
			replaceString:   "test: self.util.new().withAttr('hello').withAttr2('world').build(),",
			replaceByString: "test: self.util.new().withAttr({myKeyA: 5}).withAttr({myKeyB: 5}).attr.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "myKeyB",
						Kind:       protocol.FieldCompletion,
						InsertText: "myKeyB",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "super object",
			filename:        "./testdata/complete/selfbinary.jsonnet",
			replaceString:   "thirdObj:: 3,",
			replaceByString: "thirdObj:: super.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "firstObj",
						Kind:       protocol.FieldCompletion,
						InsertText: "firstObj",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
					{
						Label:      "secondObj",
						Kind:       protocol.FieldCompletion,
						InsertText: "secondObj",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
					// TODO: loop
					{
						Label:      "thirdObj",
						Kind:       protocol.FieldCompletion,
						InsertText: "thirdObj",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "super",
						},
					},
				},
			},
		},
		{
			name:            "self object below",
			filename:        "./testdata/complete/selfbinary.jsonnet",
			replaceString:   "thirdObj:: 3,",
			replaceByString: "thirdObj:: self.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "firstObj",
						Kind:       protocol.FieldCompletion,
						InsertText: "firstObj",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
					{
						Label:      "fourthObj",
						Kind:       protocol.FieldCompletion,
						InsertText: "fourthObj",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
					{
						Label:      "secondObj",
						Kind:       protocol.FieldCompletion,
						InsertText: "secondObj",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
					// TODO: loop
					{
						Label:      "thirdObj",
						Kind:       protocol.FieldCompletion,
						InsertText: "thirdObj",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "self",
						},
					},
				},
			},
		},
		{
			name:            "outer self global",
			filename:        "./testdata/complete/selfvar.jsonnet",
			replaceString:   "innerKey:: 5,",
			replaceByString: "innerKey:: ",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "innerSelf",
						Kind:       protocol.FieldCompletion,
						InsertText: "innerSelf",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "self",
						},
					},
					{
						Label:      "outerSelf",
						Kind:       protocol.FieldCompletion,
						InsertText: "outerSelf",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "self",
						},
					},
				},
			},
		},
		{
			name:            "outer self local",
			filename:        "./testdata/complete/selfvar.jsonnet",
			replaceString:   "innerKey:: 5,",
			replaceByString: "innerKey:: outerSelf.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "innerVals",
						Kind:       protocol.FieldCompletion,
						InsertText: "innerVals",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
					{
						Label:      "outerKey",
						Kind:       protocol.FieldCompletion,
						InsertText: "outerKey",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "array access 0",
			filename:        "./testdata/complete/array.jsonnet",
			replaceString:   "a: myArray[0].firstArray,",
			replaceByString: "a: myArray[0].",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "firstArray",
						Kind:       protocol.FieldCompletion,
						InsertText: "firstArray",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "array access 1",
			filename:        "./testdata/complete/array.jsonnet",
			replaceString:   "a: myArray[0].firstArray,",
			replaceByString: "a: myArray[1].",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "secondArray",
						Kind:       protocol.FieldCompletion,
						InsertText: "secondArray",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:            "array access 1 with key",
			filename:        "./testdata/complete/array.jsonnet",
			replaceString:   "a: myArray[0].firstArray,",
			replaceByString: "a: myArray[1].secondArray.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "key",
						Kind:       protocol.FieldCompletion,
						InsertText: "key",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "complete multiple format",
			filename:        "./testdata/complete/formatstringmultiple.jsonnet",
			replaceString:   "a: '%s:%s' % [myVar, myVar],",
			replaceByString: "a: '%s:%s' % [myVar, my",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "myVar",
						Kind:       protocol.FieldCompletion,
						InsertText: "myVar",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "complete in array with objects after",
			filename:        "./testdata/complete/topofarray.jsonnet",
			replaceString:   "myObj.keyB,",
			replaceByString: "myObj.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "keyA",
						Kind:       protocol.FieldCompletion,
						InsertText: "keyA",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
					{
						Label:      "keyB",
						Kind:       protocol.FieldCompletion,
						InsertText: "keyB",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "builder pattern in multiple lines with index",
			filename:        "./testdata/builder-pattern.jsonnet",
			replaceString:   ".build(),  // Last build line",
			replaceByString: ".b",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "attr",
						Kind:       protocol.FieldCompletion,
						InsertText: "attr",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "attr2",
						Kind:       protocol.FieldCompletion,
						InsertText: "attr2",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "build",
						Kind:       protocol.FieldCompletion,
						InsertText: "build()",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "withAttr",
						Kind:       protocol.FieldCompletion,
						InsertText: "withAttr(v)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "withAttr2",
						Kind:       protocol.FieldCompletion,
						InsertText: "withAttr2(v)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
				},
			},
		},
		{
			name:            "builder pattern in multiple lines without index",
			filename:        "./testdata/builder-pattern.jsonnet",
			replaceString:   ".build(),  // Last build line",
			replaceByString: ".",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "attr",
						Kind:       protocol.FieldCompletion,
						InsertText: "attr",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "attr2",
						Kind:       protocol.FieldCompletion,
						InsertText: "attr2",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "string",
						},
					},
					{
						Label:      "build",
						Kind:       protocol.FieldCompletion,
						InsertText: "build()",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "withAttr",
						Kind:       protocol.FieldCompletion,
						InsertText: "withAttr(v)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "withAttr2",
						Kind:       protocol.FieldCompletion,
						InsertText: "withAttr2(v)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
				},
			},
		},
		{
			name:            "complete bracket import no import",
			filename:        "./testdata/complete/import/directcomplete.jsonnet",
			replaceString:   "a: (import 'object.libsonnet'),",
			replaceByString: "a: (import 'object.libsonnet').",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "hiddenKey",
						Kind:       protocol.FieldCompletion,
						InsertText: "hiddenKey",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
					{
						Label:      "keyA",
						Kind:       protocol.FieldCompletion,
						InsertText: "keyA",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
					{
						Label:      "nestedKey",
						Kind:       protocol.FieldCompletion,
						InsertText: "nestedKey",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:            "complete bracket import nested",
			filename:        "./testdata/complete/import/directcomplete.jsonnet",
			replaceString:   "a: (import 'object.libsonnet'),",
			replaceByString: "a: (import 'object.libsonnet').nestedKey.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "innerKeyA",
						Kind:       protocol.FieldCompletion,
						InsertText: "innerKeyA",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
						},
					},
				},
			},
		},
		{
			name:            "complete bracket import nested 2",
			filename:        "./testdata/complete/import/directcomplete.jsonnet",
			replaceString:   "a: (import 'object.libsonnet'),",
			replaceByString: "a: (import 'object.libsonnet').nestedKey.innerKeyA.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "innerKeyB",
						Kind:       protocol.FieldCompletion,
						InsertText: "innerKeyB",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
				},
			},
		},
		{
			name:            "complete my builder pattern with assert",
			filename:        "./testdata/complete/builderpattern.jsonnet",
			replaceString:   "test: self.new('mybuilder'),",
			replaceByString: "test: self.new('mybuilder').withVal(1).",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "_name",
						Kind:       protocol.FieldCompletion,
						InsertText: "_name",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "_val",
						Kind:       protocol.FieldCompletion,
						InsertText: "_val",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "binary",
						},
					},
					{
						Label:      "_vals",
						Kind:       protocol.FieldCompletion,
						InsertText: "_vals",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "array",
						},
					},
					{
						Label:      "withName",
						Kind:       protocol.FieldCompletion,
						InsertText: "withName(name)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "withVal",
						Kind:       protocol.FieldCompletion,
						InsertText: "withVal(arg)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
				},
			},
		},
		{
			name:            "complete imported builder pattern",
			filename:        "./testdata/complete/builderpatternimport.jsonnet",
			replaceString:   "builder.new('middle'),",
			replaceByString: "builder.new('middle').",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "_name",
						Kind:       protocol.FieldCompletion,
						InsertText: "_name",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "_vals",
						Kind:       protocol.FieldCompletion,
						InsertText: "_vals",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "array",
						},
					},
					{
						Label:      "withName",
						Kind:       protocol.FieldCompletion,
						InsertText: "withName(name)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "withVal",
						Kind:       protocol.FieldCompletion,
						InsertText: "withVal(arg)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
				},
			},
		},
		{
			name:            "imported self val",
			filename:        "./testdata/complete/import/selfimport.jsonnet",
			replaceString:   "a: sefVal.selfVal,",
			replaceByString: "a: sefVal.selfVal.",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "myVar",
						Kind:       protocol.FieldCompletion,
						InsertText: "myVar",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "number",
						},
					},
					{
						Label:      "selfVal",
						Kind:       protocol.FieldCompletion,
						InsertText: "selfVal",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "self",
						},
					},
				},
			},
		},
		{
			name:               "named function args open func",
			filename:           "./testdata/complete/functionargs.jsonnet",
			replaceString:      "a: localfunc(arg=data),",
			replaceByString:    "a: localfunc(",
			onlyCheckIfPresent: true,
			disable:            true,
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "arg1=",
						Kind:       protocol.FieldCompletion,
						InsertText: "arg1=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
				},
			},
		},
		{
			name:               "named function args first",
			filename:           "./testdata/complete/functionargs.jsonnet",
			replaceString:      "b: multiArguments(1, 2, 3),",
			replaceByString:    "b: multiArguments(),",
			completionOffset:   -2,
			onlyCheckIfPresent: true,
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "arg1=",
						Kind:       protocol.FieldCompletion,
						InsertText: "arg1=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "arg2=",
						Kind:       protocol.FieldCompletion,
						InsertText: "arg2=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "arg3=",
						Kind:       protocol.FieldCompletion,
						InsertText: "arg3=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
				},
			},
		},
		{
			name:               "named function args second",
			filename:           "./testdata/complete/functionargs.jsonnet",
			replaceString:      "b: multiArguments(1, 2, 3),",
			replaceByString:    "b: multiArguments(1, ),",
			completionOffset:   -2,
			onlyCheckIfPresent: true,
			unexpected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "arg1=",
						Kind:       protocol.FieldCompletion,
						InsertText: "arg1=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
				},
			},
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "arg2=",
						Kind:       protocol.FieldCompletion,
						InsertText: "arg2=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "arg3=",
						Kind:       protocol.FieldCompletion,
						InsertText: "arg3=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
				},
			},
		},
		{
			name:               "named function args third",
			filename:           "./testdata/complete/functionargs.jsonnet",
			replaceString:      "b: multiArguments(1, 2, 3),",
			replaceByString:    "b: multiArguments(1, 2, ),",
			completionOffset:   -2,
			onlyCheckIfPresent: true,
			unexpected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "arg1=",
						Kind:       protocol.FieldCompletion,
						InsertText: "arg1=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "arg2=",
						Kind:       protocol.FieldCompletion,
						InsertText: "arg2=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
				},
			},
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "arg3=",
						Kind:       protocol.FieldCompletion,
						InsertText: "arg3=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
				},
			},
		},
		{
			name:               "named function args out of order",
			filename:           "./testdata/complete/functionargs.jsonnet",
			replaceString:      "b: multiArguments(1, 2, 3),",
			replaceByString:    "b: multiArguments(1, arg3=4, ),",
			completionOffset:   -2,
			onlyCheckIfPresent: true,
			unexpected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "arg1=",
						Kind:       protocol.FieldCompletion,
						InsertText: "arg1=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
					{
						Label:      "arg3=",
						Kind:       protocol.FieldCompletion,
						InsertText: "arg3=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
				},
			},
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "arg2=",
						Kind:       protocol.FieldCompletion,
						InsertText: "arg2=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
				},
			},
		},
		{
			name:               "named function args for imported",
			filename:           "./testdata/complete/functionargs.jsonnet",
			replaceString:      "c: builder.new('test').withVal(1),",
			replaceByString:    "c: builder.new(),",
			completionOffset:   -2,
			onlyCheckIfPresent: true,
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "name=",
						Kind:       protocol.FieldCompletion,
						InsertText: "name=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
				},
			},
		},
		{
			name:               "named function args in prev arg for imported",
			filename:           "./testdata/complete/functionargs.jsonnet",
			replaceString:      "c: builder.new('test').withVal(1),",
			replaceByString:    "c: builder.new().withVal(1),",
			completionOffset:   -13,
			onlyCheckIfPresent: true,
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "name=",
						Kind:       protocol.FieldCompletion,
						InsertText: "name=",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "variable",
						},
					},
				},
			},
		},
		{
			name:            "nested function args",
			filename:        "./testdata/complete/nested_func_arg.jsonnet",
			replaceString:   "test:: self.func2({}),",
			replaceByString: "test:: self.func2({}).",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "func1",
						Kind:       protocol.FieldCompletion,
						InsertText: "func1(arg1)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "func2",
						Kind:       protocol.FieldCompletion,
						InsertText: "func2(arg2)",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "function",
						},
					},
					{
						Label:      "test",
						Kind:       protocol.FieldCompletion,
						InsertText: "test",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "apply",
						},
					},
				},
			},
		},
		{
			name:            "functionarg in empty body",
			filename:        "./testdata/complete/function/argument.jsonnet",
			replaceString:   "functionarg  // body",
			replaceByString: " ",
			disable:         true,
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "local",
						Kind:       protocol.FieldCompletion,
						InsertText: "local",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "local",
						},
					},
					{
						Label:      "functionarg",
						Kind:       protocol.FieldCompletion,
						InsertText: "functionarg",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "object",
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
			name:             "extvar completion",
			filename:         "./testdata/complete/extcode.jsonnet",
			replaceString:    "'code'",
			replaceByString:  "''",
			completionOffset: -1,
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "code",
						Kind:       protocol.FieldCompletion,
						InsertText: "code",
					},
				},
			},
		},
	}
	keywordTestCases := []completionCase{
		{
			name:            "self global outside of object",
			filename:        "./testdata/complete/keywords/self.jsonnet",
			replaceString:   "local myVar = 5;",
			replaceByString: "local myVar = s",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items:        []protocol.CompletionItem{},
			},
		},
		{
			name:            "self global inside of object",
			filename:        "./testdata/complete/keywords/self.jsonnet",
			replaceString:   "keyA: 4",
			replaceByString: "keyA: s",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "self",
						Kind:       protocol.FieldCompletion,
						InsertText: "self",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "self",
						},
					},
				},
			},
		},
		{
			name:            "super global without base",
			filename:        "./testdata/complete/keywords/self.jsonnet",
			replaceString:   "keyA: 4",
			replaceByString: "keyA: su",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items:        []protocol.CompletionItem{},
			},
		},
		{
			name:            "super global with base",
			filename:        "./testdata/complete/keywords/super.jsonnet",
			replaceString:   "keyA: 4",
			replaceByString: "keyA: su",
			expected: protocol.CompletionList{
				IsIncomplete: false,
				Items: []protocol.CompletionItem{
					{
						Label:      "super",
						Kind:       protocol.FieldCompletion,
						InsertText: "super",
						LabelDetails: &protocol.CompletionItemLabelDetails{
							Description: "super",
						},
					},
				},
			},
		},
	}
	completionConfig := config.Configuration{
		JPaths: []string{"testdata", "testdata/complete", "testdata/complete/import"},
		ExtCode: map[string]string{
			"code": "{ objA: 5, ['%s' % 'computed']: 3}",
		},
		Workarounds: config.WorkaroundConfig{
			AssumeTrueConditionOnError: true,
		},
	}
	testCompletion(t, &completionConfig, testCases)

	completionConfig.Completion.EnableKeywords = true
	testCompletion(t, &completionConfig, keywordTestCases)
}

func testCompletion(t *testing.T, config *config.Configuration, testCases []completionCase) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content, err := os.ReadFile(tc.filename)
			require.NoError(t, err)

			server, fileURI := testServerWithFile(t, completionTestStdlib, string(content))
			server.configuration = *config
			var version int32 = 2

			replacedContent := strings.ReplaceAll(string(content), tc.replaceString, tc.replaceByString)

			updateText(t, server, replacedContent, fileURI, version)

			cursorPosition := protocol.Position{}
			for _, line := range strings.Split(replacedContent, "\n") {
				if strings.Contains(line, tc.replaceByString) {
					cursorPosition.Character = uint32(strings.Index(line,
						tc.replaceByString) + len(tc.replaceByString))
					break
				}
				cursorPosition.Line++
			}
			if tc.lineOverride != 0 {
				cursorPosition.Line = uint32(tc.lineOverride)
			}
			// This is worse than rust...
			cursorPosition.Character = min(uint32(int64(cursorPosition.Character)+
				int64(tc.completionOffset)), cursorPosition.Character)
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

			testResult(t, result, tc, cursorPosition)

			// Test if the completion also works if the last change was in a different place
			newText := "local abcde = 5;\n" + replacedContent
			version++
			updateText(t, server, newText, fileURI, version)
			version++
			updateText(t, server, replacedContent, fileURI, version)

			testResult(t, result, tc, cursorPosition)

			// TODO: test typing char by char
			// TODO: support loading a broken document
		})
	}
}

func updateText(t *testing.T, server *Server, replacedContent string, fileURI protocol.DocumentURI, version int32) {
	err := server.DidChange(context.Background(), &protocol.DidChangeTextDocumentParams{
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{
			Text: replacedContent,
		}},
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: fileURI},
			Version:                version,
		},
	})
	require.NoError(t, err)
}

func testResult(t *testing.T, result *protocol.CompletionList, tc completionCase, cursorPosition protocol.Position) {
	t.Helper()
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

	for i := range tc.unexpected.Items {
		tc.unexpected.Items[i].Detail = ""
		tc.unexpected.Items[i].Kind = protocol.VariableCompletion
	}
	if !tc.disable {
		if tc.onlyCheckIfPresent {
			for _, item := range tc.expected.Items {
				assert.Contains(t, result.Items, item)
			}
		} else {
			require.Equal(t, tc.expected, *result, "position", cursorPosition, "file", tc.filename)
		}

		for _, item := range tc.unexpected.Items {
			for _, resultItem := range result.Items {
				assert.NotEqual(t, item, resultItem)
			}
		}
	} else {
		t.Skipf("Skipping disabled test case %s", tc.name)
	}
}

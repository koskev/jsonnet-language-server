package server

import (
	"context"
	"testing"

	"github.com/google/go-jsonnet/formatter"
	"github.com/grafana/jsonnet-language-server/pkg/server/config"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestConfiguration(t *testing.T) {
	type kase struct {
		name               string
		settings           any
		expectedFileOutput string
		fileContent        string

		errorExpected bool
	}

	testCases := []kase{
		{
			name:          "settings is not an object",
			settings:      []string{""},
			fileContent:   `[]`,
			errorExpected: true,
		},
		{
			name: "ext_var config is empty",
			settings: map[string]any{
				"ext_vars": map[string]any{},
			},
			fileContent:        `[]`,
			expectedFileOutput: `[]`,
		},
		{
			name:               "ext_var config is missing",
			settings:           map[string]any{},
			fileContent:        `[]`,
			expectedFileOutput: `[]`,
		},
		{
			name: "ext_var config is not an object",
			settings: map[string]any{
				"ext_vars": []string{},
			},
			fileContent:   `[]`,
			errorExpected: true,
		},
		{
			name: "ext_var config value is not a string",
			settings: map[string]any{
				"ext_vars": map[string]any{
					"foo": true,
				},
			},
			fileContent:   `[]`,
			errorExpected: true,
		},
		{
			name: "ext_var config is valid",
			settings: map[string]any{
				"ext_vars": map[string]any{
					"hello": "world",
				},
			},
			fileContent: `
{
	hello: std.extVar("hello"),
}
			`,
			expectedFileOutput: `
{
	"hello": "world"
}
			`,
		},
		{
			name: "ext_code config is not an object",
			settings: map[string]any{
				"ext_code": []string{},
			},
			fileContent:   `[]`,
			errorExpected: true,
		},
		{
			name: "ext_code config is empty",
			settings: map[string]any{
				"ext_code": map[string]any{},
			},
			fileContent:        `[]`,
			expectedFileOutput: `[]`,
		},
		{
			name: "ext_code config is valid",
			settings: map[string]any{
				"ext_code": map[string]any{
					"hello": "{\"world\": true,}",
				},
			},
			fileContent: `
{
	hello: std.extVar("hello"),
}
			`,
			expectedFileOutput: `
{
	"hello": {
		"world": true
	}
}
			`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, fileURI := testServerWithFile(t, nil, tc.fileContent)

			err := s.DidChangeConfiguration(
				context.TODO(),
				&protocol.DidChangeConfigurationParams{
					Settings: tc.settings,
				},
			)
			if tc.errorExpected {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			vm := s.getVM("any")

			doc, err := s.cache.Get(fileURI)
			assert.NoError(t, err)

			json, err := vm.Evaluate(doc.AST)
			assert.NoError(t, err)
			assert.JSONEq(t, tc.expectedFileOutput, json)
		})
	}
}

func TestConfiguration_Formatting(t *testing.T) {
	type kase struct {
		name                  string
		settings              any
		expectedConfiguration config.Configuration
		errorExpected         bool
	}

	testCases := []kase{
		{
			name: "formatting opts",
			settings: map[string]any{
				"formatting": map[string]any{
					"Indent":           4,
					"MaxBlankLines":    10,
					"StringStyle":      "single",
					"CommentStyle":     "leave",
					"PrettyFieldNames": true,
					"PadArrays":        false,
					"PadObjects":       true,
					"SortImports":      false,
					"UseImplicitPlus":  true,
					"StripEverything":  false,
					"StripComments":    false,
					// not setting StripAllButComments
				},
			},
			expectedConfiguration: config.Configuration{
				FormattingOptions: func() formatter.Options {
					opts := formatter.DefaultOptions()
					opts.Indent = 4
					opts.MaxBlankLines = 10
					opts.StringStyle = formatter.StringStyleSingle
					opts.CommentStyle = formatter.CommentStyleLeave
					opts.PrettyFieldNames = true
					opts.PadArrays = false
					opts.PadObjects = true
					opts.SortImports = false
					opts.UseImplicitPlus = true
					opts.StripEverything = false
					opts.StripComments = false
					return opts
				}(),
			},
		},
		{
			name: "invalid string style",
			settings: map[string]any{
				"formatting": map[string]any{
					"StringStyle": "invalid",
				},
			},
			errorExpected: true,
		},
		{
			name: "invalid comment style",
			settings: map[string]any{
				"formatting": map[string]any{
					"CommentStyle": "invalid",
				},
			},
			errorExpected: true,
		},
		{
			name: "invalid comment style type",
			settings: map[string]any{
				"formatting": map[string]any{
					"CommentStyle": 123,
				},
			},
			errorExpected: true,
		},
		{
			name: "does not override default values",
			settings: map[string]any{
				"formatting": map[string]any{},
			},
			expectedConfiguration: config.Configuration{FormattingOptions: formatter.DefaultOptions()},
		},
		{
			name: "invalid jpath type",
			settings: map[string]any{
				"jpath": 123,
			},
			errorExpected: true,
		},
		{
			name: "invalid jpath item type",
			settings: map[string]any{
				"jpath": []any{123},
			},
			errorExpected: true,
		},
		{
			name: "invalid bool",
			settings: map[string]any{
				"resolve_paths_with_tanka": "true",
			},
			errorExpected: true,
		},
		{
			name: "invalid log level",
			settings: map[string]any{
				"log_level": "bad",
			},
			errorExpected: true,
		},
		{
			name: "all settings",
			settings: map[string]any{
				"log_level": "error",
				"formatting": map[string]any{
					"Indent":              4,
					"MaxBlankLines":       10,
					"StringStyle":         "double",
					"CommentStyle":        "slash",
					"PrettyFieldNames":    false,
					"PadArrays":           true,
					"PadObjects":          false,
					"SortImports":         false,
					"UseImplicitPlus":     false,
					"StripEverything":     true,
					"StripComments":       true,
					"StripAllButComments": true,
				},
				"ext_vars": map[string]any{
					"hello": "world",
				},
				"ext_code": map[string]any{
					"hello": "{\"world\": true,}",
				},
				"resolve_paths_with_tanka": false,
				"jpath":                    []any{"blabla", "blabla2"},
				"diagnostics": map[string]any{
					"enable_eval_diagnostics": false,
					"enable_lint_diagnostics": true,
				},
			},
			expectedConfiguration: config.Configuration{
				LogLevel: logrus.ErrorLevel,
				FormattingOptions: func() formatter.Options {
					opts := formatter.DefaultOptions()
					opts.Indent = 4
					opts.MaxBlankLines = 10
					opts.StringStyle = formatter.StringStyleDouble
					opts.CommentStyle = formatter.CommentStyleSlash
					opts.PrettyFieldNames = false
					opts.PadArrays = true
					opts.PadObjects = false
					opts.SortImports = false
					opts.UseImplicitPlus = false
					opts.StripEverything = true
					opts.StripComments = true
					opts.StripAllButComments = true
					return opts
				}(),
				ExtVars: map[string]string{
					"hello": "world",
				},
				ExtCode: map[string]string{
					"hello": "{\"world\": true,}",
				},
				ResolvePathsWithTanka: false,
				JPaths:                []string{"blabla", "blabla2"},
				Diagnostics: config.DiagnosticConfig{
					EnableEvalDiagnostics: false,
					EnableLintDiagnostics: true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := testServerWithFile(t, nil, "")

			err := s.DidChangeConfiguration(
				context.TODO(),
				&protocol.DidChangeConfigurationParams{
					Settings: tc.settings,
				},
			)
			if tc.errorExpected {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// FUCK YOU GO AND YOUR MISSING FEATURES!!
			if tc.expectedConfiguration.ExtCode == nil {
				tc.expectedConfiguration.ExtCode = make(map[string]string)
			}

			assert.Equal(t, tc.expectedConfiguration, s.configuration)
		})
	}
}

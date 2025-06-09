package config

import "github.com/google/go-jsonnet/formatter"

type InlayFunctionArgs struct {
	ShowWithSameName bool `json:"show_with_same_name"`
}

type ConfigurationInlay struct {
	// Of course go does neither support options nor default values...
	// So since go is a stupid language and I don't want to hack proper defaults in they are just all false by default
	EnableDebugAst     bool `json:"enable_debug_ast"`
	EnableIndexValue   bool `json:"enable_index_value"`
	EnableFunctionArgs bool `json:"enable_function_args"`

	FunctionArgs InlayFunctionArgs `json:"function_args"`
}

type WorkaroundConfig struct {
	AssumeTrueConditionOnError bool `json:"assume_true_condition_on_error"`
}

type ExtCodeConfig struct {
	FindUpwards bool `json:"find_upwards"`
}

type CompletionConfig struct {
	EnableSnippets  bool `json:"enable_snippets"`
	UseTypeInDetail bool `json:"use_type_in_detail"`
}

type Configuration struct {
	ResolvePathsWithTanka bool
	JPaths                []string
	ExtVars               map[string]string
	ExtCode               map[string]string
	FormattingOptions     formatter.Options

	EnableEvalDiagnostics     bool
	EnableLintDiagnostics     bool
	ShowDocstringInCompletion bool
	MaxInlayLength            int
	Inlay                     ConfigurationInlay

	EnableSemanticTokens bool

	Workarounds WorkaroundConfig

	ExtCodeConfig ExtCodeConfig
	Completion    CompletionConfig
}

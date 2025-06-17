package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"

	"github.com/google/go-jsonnet/formatter"
	"github.com/jdbaldry/go-language-server-protocol/jsonrpc2"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
)

var extCodeSuffix = ".extcode.jsonnet"

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

	MaxLength int `json:"max_length"`
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
	ShowDocstring   bool `json:"show_docstring"`
}

type PathConfig struct {
	ExtCode        ExtCodeConfig `json:"ext_code"`
	RelativeJPaths []string      `json:"relative_jpaths"`
}

type DiagnosticConfig struct {
	EnableEvalDiagnostics bool `json:"enable_eval_diagnostics"`
	EnableLintDiagnostics bool `json:"enable_lint_diagnostics"`
}

type Configuration struct {
	LogLevel              log.Level         `jsoin:"log_level"`
	ResolvePathsWithTanka bool              `json:"resolve_paths_with_tanka"`
	JPaths                []string          `json:"jpath"`
	ExtVars               map[string]string `json:"ext_vars"`
	ExtCode               map[string]string `json:"ext_code"`
	FormattingOptions     formatter.Options `json:"formatting"`
	Diagnostics           DiagnosticConfig  `json:"diagnostics"`

	Inlay ConfigurationInlay `json:"inlay"`

	EnableSemanticTokens bool `json:"enable_semantic_tokens"`

	Workarounds WorkaroundConfig `json:"workarounds"`

	Completion CompletionConfig `json:"completion"`

	workspaces []protocol.WorkspaceFolder
}

func NewConfiguration(data any, workspaces []protocol.WorkspaceFolder) (*Configuration, error) {
	settings := Configuration{}
	settings.workspaces = workspaces
	configBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshalling extcode config: %w", err)
	}
	err = json.Unmarshal(configBytes, &settings)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling extcode config: %w", err)
	}

	return &settings, nil
}

func (c *Configuration) UnmarshalJSON(data []byte) error {
	type LSPConfig Configuration
	aux := &struct {
		FormattingOptions map[string]any `json:"formatting"`
		LogLevel          string         `json:"log_level"`
		Paths             PathConfig     `json:"paths"`
		*LSPConfig
	}{
		LSPConfig: (*LSPConfig)(c),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	c = (*Configuration)(aux.LSPConfig)

	var err error
	c.FormattingOptions, err = parseFormattingOpts(aux.FormattingOptions)
	if err != nil {
		return err
	}

	if len(aux.LogLevel) > 0 {
		level, err := log.ParseLevel(aux.LogLevel)
		if err != nil {
			return fmt.Errorf("%w: %v", jsonrpc2.ErrInvalidParams, err)
		}
		c.LogLevel = level
	}
	c.buildJPaths(aux.Paths.RelativeJPaths)
	extCode, err := c.loadExtCodeFiles(aux.Paths.ExtCode)
	if err != nil {
		return err
	}
	maps.Copy(c.ExtCode, extCode)

	return nil
}

func (c *Configuration) buildJPaths(relativePaths []string) {
	for _, folder := range c.workspaces {
		for _, name := range relativePaths {
			c.JPaths = append(c.JPaths, fmt.Sprintf("%s/%s", folder.Name, name))
		}
	}
}

func parseFormattingOpts(unparsed any) (formatter.Options, error) {
	newOpts, ok := unparsed.(map[string]any)
	if !ok {
		return formatter.Options{}, fmt.Errorf("unsupported settings value for formatting. expected json object. got: %T", unparsed)
	}

	opts := formatter.DefaultOptions()
	config := mapstructure.DecoderConfig{
		Result: &opts,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			stringStyleDecodeFunc,
			commentStyleDecodeFunc,
		),
	}
	decoder, err := mapstructure.NewDecoder(&config)
	if err != nil {
		return formatter.Options{}, fmt.Errorf("decoder construction failed: %v", err)
	}

	if err := decoder.Decode(newOpts); err != nil {
		return formatter.Options{}, fmt.Errorf("map decode failed: %v", err)
	}
	return opts, nil
}

func stringStyleDecodeFunc(_, to reflect.Type, unparsed any) (any, error) {
	if to != reflect.TypeOf(formatter.StringStyleDouble) {
		return unparsed, nil
	}

	str, ok := unparsed.(string)
	if !ok {
		return nil, fmt.Errorf("expected string, got: %T", unparsed)
	}
	// will not panic because of the kind == string check above
	switch str {
	case "double":
		return formatter.StringStyleDouble, nil
	case "single":
		return formatter.StringStyleSingle, nil
	case "leave":
		return formatter.StringStyleLeave, nil
	default:
		return nil, fmt.Errorf("expected one of 'double', 'single', 'leave', got: %q", str)
	}
}

func commentStyleDecodeFunc(_, to reflect.Type, unparsed any) (any, error) {
	if to != reflect.TypeOf(formatter.CommentStyleHash) {
		return unparsed, nil
	}

	str, ok := unparsed.(string)
	if !ok {
		return nil, fmt.Errorf("expected string, got: %T", unparsed)
	}
	switch str {
	case "hash":
		return formatter.CommentStyleHash, nil
	case "slash":
		return formatter.CommentStyleSlash, nil
	case "leave":
		return formatter.CommentStyleLeave, nil
	default:
		return nil, fmt.Errorf("expected one of 'hash', 'slash', 'leave', got: %q", str)
	}
}

func (c *Configuration) loadExtCodeFiles(config ExtCodeConfig) (map[string]string, error) {
	currentPath := "./"

	fileMap := map[string]string{}

	var err error
	for {
		currentPath, err = filepath.Abs(currentPath)
		if err != nil {
			return nil, fmt.Errorf("getting abs path for %s: %w", currentPath, err)
		}
		files, err := c.findNextExtCodeFile(currentPath)
		if err == nil {
			for _, found := range files {
				paramName := strings.TrimSuffix(found, extCodeSuffix)
				// WTF GO!? No simple "has" function?
				if _, exists := fileMap[paramName]; exists {
					// Skip existing config as the lower ones should have precedence
					continue
				}
				content, err := os.ReadFile(found)
				if err != nil {
					return nil, fmt.Errorf("reading extcode file %s: %w", found, err)
				}
				fileMap[paramName] = string(content)
			}
		}

		// I can't test on Windows and honestly don't even want to mess with it. Therefore I'll just skip it
		if !config.FindUpwards || currentPath == "/" || runtime.GOOS == "windows" {
			break
		}
		currentPath += "/.."
	}

	return fileMap, nil
}

func (c *Configuration) findNextExtCodeFile(currentPath string) ([]string, error) {
	foundFiles := []string{}
	cwd, err := filepath.Abs(currentPath)
	if err != nil {
		return nil, fmt.Errorf("getting cwd for ext code file: %w", err)
	}
	files, err := os.ReadDir(cwd)
	if err != nil {
		return nil, fmt.Errorf("reading dir %s: %w", cwd, err)
	}
	// Am I just spoiled or is go just this stupid? Again this would be a simple "filter" in almost any other language
	for {
		idx := slices.IndexFunc(files, func(entry os.DirEntry) bool {
			return !entry.IsDir() && strings.HasSuffix(entry.Name(), extCodeSuffix)
		})

		if idx < 0 {
			break
		}

		foundFiles = append(foundFiles, files[idx].Name())

		files = files[idx+1:]
	}

	return foundFiles, nil
}

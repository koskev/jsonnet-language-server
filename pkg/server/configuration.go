package server

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"

	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/formatter"
	"github.com/grafana/jsonnet-language-server/pkg/server/config"
	"github.com/jdbaldry/go-language-server-protocol/jsonrpc2"
	"github.com/jdbaldry/go-language-server-protocol/lsp/protocol"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
)

var extCodeSuffix = ".extcode.jsonnet"

// TODO: currently partial config changes are not supported for all settings (due to just unmarshalling the configs). However, the old code is stupid as well (but correct :) )
//
//nolint:gocyclo
func (s *Server) DidChangeConfiguration(_ context.Context, params *protocol.DidChangeConfigurationParams) error {
	settingsMap, ok := params.Settings.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: unsupported settings payload. expected json object, got: %T", jsonrpc2.ErrInvalidParams, params.Settings)
	}

	for sk, sv := range settingsMap {
		switch sk {
		case "log_level":
			sv, ok := sv.(string)
			if !ok {
				return fmt.Errorf("log_level config has wrong type")
			}
			level, err := log.ParseLevel(sv)
			if err != nil {
				return fmt.Errorf("%w: %v", jsonrpc2.ErrInvalidParams, err)
			}
			log.SetLevel(level)
		case "resolve_paths_with_tanka":
			if boolVal, ok := sv.(bool); ok {
				s.configuration.ResolvePathsWithTanka = boolVal
			} else {
				return fmt.Errorf("%w: unsupported settings value for resolve_paths_with_tanka. expected boolean. got: %T", jsonrpc2.ErrInvalidParams, sv)
			}
		case "jpath":
			if svList, ok := sv.([]interface{}); ok {
				s.configuration.JPaths = make([]string, len(svList))
				for i, v := range svList {
					if strVal, ok := v.(string); ok {
						s.configuration.JPaths[i] = strVal
					} else {
						return fmt.Errorf("%w: unsupported settings value for jpath. expected string. got: %T", jsonrpc2.ErrInvalidParams, v)
					}
				}
			} else {
				return fmt.Errorf("%w: unsupported settings value for jpath. expected array of strings. got: %T", jsonrpc2.ErrInvalidParams, sv)
			}

		case "enable_eval_diagnostics":
			if boolVal, ok := sv.(bool); ok {
				s.configuration.EnableEvalDiagnostics = boolVal
			} else {
				return fmt.Errorf("%w: unsupported settings value for enable_eval_diagnostics. expected boolean. got: %T", jsonrpc2.ErrInvalidParams, sv)
			}
		case "enable_lint_diagnostics":
			if boolVal, ok := sv.(bool); ok {
				s.configuration.EnableLintDiagnostics = boolVal
			} else {
				return fmt.Errorf("%w: unsupported settings value for enable_lint_diagnostics. expected boolean. got: %T", jsonrpc2.ErrInvalidParams, sv)
			}
		case "show_docstring_in_completion":
			if boolVal, ok := sv.(bool); ok {
				s.configuration.ShowDocstringInCompletion = boolVal
			} else {
				return fmt.Errorf("%w: unsupported settings value for show_docstring_in_completion. expected boolean. got: %T", jsonrpc2.ErrInvalidParams, sv)
			}
		case "ext_vars":
			newVars, err := s.parseExtVars(sv)
			if err != nil {
				return fmt.Errorf("%w: ext_vars parsing failed: %v", jsonrpc2.ErrInvalidParams, err)
			}
			s.configuration.ExtVars = newVars
		case "formatting":
			newFmtOpts, err := s.parseFormattingOpts(sv)
			if err != nil {
				return fmt.Errorf("%w: formatting options parsing failed: %v", jsonrpc2.ErrInvalidParams, err)
			}
			s.configuration.FormattingOptions = newFmtOpts

		case "ext_code":
			newCode, err := s.parseExtCode(sv)
			if err != nil {
				return fmt.Errorf("%w: ext_code parsing failed: %v", jsonrpc2.ErrInvalidParams, err)
			}
			if s.configuration.ExtCode == nil {
				s.configuration.ExtCode = map[string]string{}
			}
			maps.Copy(s.configuration.ExtCode, newCode)
		case "max_inlay_length":
			if length, ok := sv.(int); ok {
				s.configuration.MaxInlayLength = length
			} else {
				return fmt.Errorf("%w: unsupported settings value for max_inlay_length. expected int. got: %T", jsonrpc2.ErrInvalidParams, sv)
			}
		case "inlay_config":
			var inlayConfig config.ConfigurationInlay
			stringMap, ok := sv.(map[string]any)
			if !ok {
				return fmt.Errorf("%w: unsupported settings value for inlay_config. Expected json object. got: %T", jsonrpc2.ErrInvalidParams, sv)
			}
			configBytes, err := json.Marshal(stringMap)
			if err != nil {
				return fmt.Errorf("marshalling inlay config: %w", err)
			}
			err = json.Unmarshal(configBytes, &inlayConfig)
			if err != nil {
				return fmt.Errorf("unmarshalling inlay config: %w", err)
			}
			s.configuration.Inlay = inlayConfig
		case "workarounds":
			var workaroundConfig config.WorkaroundConfig
			stringMap, ok := sv.(map[string]any)
			if !ok {
				return fmt.Errorf("%w: unsupported settings value for workarounds. Expected json object. got: %T", jsonrpc2.ErrInvalidParams, sv)
			}
			configBytes, err := json.Marshal(stringMap)
			if err != nil {
				return fmt.Errorf("marshalling inlay config: %w", err)
			}
			err = json.Unmarshal(configBytes, &workaroundConfig)
			if err != nil {
				return fmt.Errorf("unmarshalling inlay config: %w", err)
			}
			s.configuration.Workarounds = workaroundConfig
		case "ext_code_config":
			var extCodeConfig config.ExtCodeConfig
			stringMap, ok := sv.(map[string]any)
			if !ok {
				return fmt.Errorf("%w: unsupported settings value for ext_code_config. Expected json object. got: %T", jsonrpc2.ErrInvalidParams, sv)
			}
			configBytes, err := json.Marshal(stringMap)
			if err != nil {
				return fmt.Errorf("marshalling extcode config: %w", err)
			}
			err = json.Unmarshal(configBytes, &extCodeConfig)
			if err != nil {
				return fmt.Errorf("unmarshalling extcode config: %w", err)
			}
			s.configuration.ExtCodeConfig = extCodeConfig

			extCode, err := s.loadExtCodeFiles(extCodeConfig)
			if err != nil {
				return fmt.Errorf("%w: ext_code_config parsing failed: %v", jsonrpc2.ErrInvalidParams, err)
			}
			maps.Copy(extCode, s.configuration.ExtCode)
			s.configuration.ExtCode = extCode

		case "enable_semantic_tokens":
			if boolVal, ok := sv.(bool); ok {
				s.configuration.EnableSemanticTokens = boolVal
			} else {
				return fmt.Errorf("%w: unsupported settings value for enable_semantic_tokens. expected boolean. got: %T", jsonrpc2.ErrInvalidParams, sv)
			}

		case "completion":
			var completionConfig config.CompletionConfig
			stringMap, ok := sv.(map[string]any)
			if !ok {
				return fmt.Errorf("%w: unsupported settings value for completion. Expected json object. got: %T", jsonrpc2.ErrInvalidParams, sv)
			}
			configBytes, err := json.Marshal(stringMap)
			if err != nil {
				return fmt.Errorf("marshalling completion config: %w", err)
			}
			err = json.Unmarshal(configBytes, &completionConfig)
			if err != nil {
				return fmt.Errorf("unmarshalling completion config: %w", err)
			}
			s.configuration.Completion = completionConfig

		default:
			return fmt.Errorf("%w: unsupported settings key: %q", jsonrpc2.ErrInvalidParams, sk)
		}
	}
	log.Infof("configuration updated: %+v", s.configuration)

	return nil
}

func (s *Server) parseExtVars(unparsed interface{}) (map[string]string, error) {
	newVars, ok := unparsed.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unsupported settings value for ext_vars. expected json object. got: %T", unparsed)
	}

	extVars := make(map[string]string, len(newVars))
	for varKey, varValue := range newVars {
		vv, ok := varValue.(string)
		if !ok {
			return nil, fmt.Errorf("unsupported settings value for ext_vars.%s. expected string. got: %T", varKey, varValue)
		}
		extVars[varKey] = vv
	}
	return extVars, nil
}

func (s *Server) parseFormattingOpts(unparsed interface{}) (formatter.Options, error) {
	newOpts, ok := unparsed.(map[string]interface{})
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

func (s *Server) loadExtCodeFiles(config config.ExtCodeConfig) (map[string]string, error) {
	currentPath := "./"

	fileMap := map[string]string{}

	var err error
	for {
		currentPath, err = filepath.Abs(currentPath)
		if err != nil {
			return nil, fmt.Errorf("getting abs path for %s: %w", currentPath, err)
		}
		files, err := s.findNextExtCodeFile(currentPath)
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

func (s *Server) findNextExtCodeFile(currentPath string) ([]string, error) {
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

func (s *Server) parseExtCode(unparsed any) (map[string]string, error) {
	newVars, ok := unparsed.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unsupported settings value for ext_code. expected json object. got: %T", unparsed)
	}

	extCode := make(map[string]string, len(newVars))
	for varKey, varValue := range newVars {
		vv, ok := varValue.(string)
		if !ok {
			return nil, fmt.Errorf("unsupported settings value for ext_code.%s. expected string. got: %T", varKey, varValue)
		}
		extCode[varKey] = vv
	}

	return extCode, nil
}

func resetExtVars(vm *jsonnet.VM, vars map[string]string, code map[string]string) {
	vm.ExtReset()
	for vk, vv := range vars {
		vm.ExtVar(vk, vv)
	}
	for vk, vv := range code {
		vm.ExtCode(vk, vv)
	}
}

func stringStyleDecodeFunc(_, to reflect.Type, unparsed interface{}) (interface{}, error) {
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

func commentStyleDecodeFunc(_, to reflect.Type, unparsed interface{}) (interface{}, error) {
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

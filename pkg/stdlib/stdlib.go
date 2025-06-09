package stdlib

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
)

var (
	//go:embed html.libsonnet
	htmlLibsonnetContent string
	//go:embed stdlib-content.jsonnet
	stdLibContent string

	mathFuncRegex = regexp.MustCompile(`<ul><code>std\.(?P<name>[a-zA-Z0-9]+)\((?P<params>[a-z, ]+)\)<\/code><\/ul>`)
	typeFuncRegex = regexp.MustCompile(`<code>std\.(?P<name>[a-zA-Z0-9]+)\(v\)<\/code>`)
)

type Function struct {
	Name           string   `json:"name"`
	AvailableSince string   `json:"availableSince"`
	Params         []string `json:"params"`

	Description         any    `json:"description"`
	RenderedDescription string `json:"renderedDescription"`
	MarkdownDescription string

	// This function only applies to the given types
	// TODO: make it per parameter
	TypeLimitations []reflect.Type
}

func (f *Function) Signature() string {
	sig := "std." + f.Name
	if len(f.Params) > 0 {
		sig += "(" + strings.Join(f.Params, ", ") + ")"
	}
	return sig
}

type group struct {
	ID            string     `json:"id"`
	Intro         any        `json:"intro"`
	RenderedIntro string     `json:"renderedIntro"`
	Name          string     `json:"name"`
	Fields        []Function `json:"fields"`
}

type stdlib struct {
	Prefix string  `json:"prefix"`
	Groups []group `json:"groups"`
}

var stdLibLimitations = map[string][]reflect.Type{
	"length": {reflect.TypeFor[*ast.Array](), reflect.TypeFor[*ast.LiteralString](), reflect.TypeFor[*ast.DesugaredObject](), reflect.TypeFor[*ast.Function]()},
}

func Functions() ([]Function, error) {
	var lib stdlib

	vm := jsonnet.MakeVM()
	vm.Importer(&jsonnet.MemoryImporter{
		Data: map[string]jsonnet.Contents{
			"html.libsonnet": jsonnet.MakeContents(htmlLibsonnetContent),
		},
	})

	// Hack. Remove the examples, they use some new functions that may not be ready yet in the go lib
	modifiedStdLibContent := strings.ReplaceAll(stdLibContent, "examples:", "examples::")
	// Hack. Render some of the descriptions
	modifiedStdLibContent = strings.ReplaceAll(modifiedStdLibContent, "intro:", "renderedIntro: html.render(self.intro), \nintro:")
	modifiedStdLibContent = strings.ReplaceAll(modifiedStdLibContent, "description:", "renderedDescription: html.render(self.description), \ndescription:")

	jsonContent, err := vm.EvaluateAnonymousSnippet("", modifiedStdLibContent)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(jsonContent), &lib); err != nil {
		return nil, err
	}

	converter := md.NewConverter("", true, nil)
	allFunctions := []Function{}
	for _, group := range lib.Groups {
		// Add math library functions
		if group.ID == "math" {
			mathFuncs := mathFuncRegex.FindAllStringSubmatch(group.RenderedIntro, -1)
			for _, mathFunc := range mathFuncs {
				params := strings.Split(mathFunc[2], ",")
				for i, param := range params {
					params[i] = strings.TrimSpace(param)
				}
				allFunctions = append(allFunctions, Function{
					Name:            mathFunc[1],
					Params:          params,
					TypeLimitations: []reflect.Type{reflect.TypeFor[*ast.LiteralNumber]()},
				})
			}
		}

		for _, field := range group.Fields {
			if field.AvailableSince == "upcoming" {
				continue
			}
			field.MarkdownDescription, err = converter.ConvertString(field.RenderedDescription)
			if err != nil {
				return nil, err
			}

			// Add type library functions
			if field.Name == "type" {
				typeFuncs := typeFuncRegex.FindAllStringSubmatch(field.RenderedDescription, -1)
				for _, typeFunc := range typeFuncs {
					allFunctions = append(allFunctions, Function{
						Name:   typeFunc[1],
						Params: []string{"v"},
					})
				}
			}
			if group.ID == "string" || group.ID == "parsing" {
				field.TypeLimitations = append(field.TypeLimitations, reflect.TypeFor[*ast.LiteralString]())
			}
			if group.ID == "objects" {
				field.TypeLimitations = append(field.TypeLimitations, reflect.TypeFor[*ast.DesugaredObject]())
			}

			allFunctions = append(allFunctions, field)
		}
	}

	// Add undocumented functions
	// https://github.com/google/go-jsonnet/blob/12bd29d164b131a4cd84f22f1456fe37136abc6d/linter/internal/types/stdlib.go#L162-L170
	for key, params := range map[string][]string{
		"manifestJson":    {"value"},
		"objectHasEx":     {"obj", "fname", "hidden"},
		"objectFieldsEx":  {"obj", "hidden"},
		"modulo":          {"x", "y"},
		"primitiveEquals": {"x", "y"},
		"mod":             {"a", "b"},
		"native":          {"x"},
	} {
		allFunctions = append(allFunctions, Function{
			Name:                key,
			Params:              params,
			MarkdownDescription: "**Undocumented**\n\nSee https://github.com/google/go-jsonnet/blob/12bd29d164b131a4cd84f22f1456fe37136abc6d/linter/internal/types/stdlib.go#L162-L170",
		},
		)
	}

	for i, stdFunc := range allFunctions {
		restrictedTypes, ok := stdLibLimitations[stdFunc.Name]
		if !ok {
			continue
		}
		allFunctions[i].TypeLimitations = restrictedTypes
	}

	return allFunctions, nil
}

func GetStdFunction(functionNode ast.Node, stdLib *map[string]Function) (*ast.Function, error) {
	for _, freeVar := range functionNode.FreeVariables() {
		if freeVar == "std" {
			indexNode, ok := functionNode.(*ast.Index)
			if !ok {
				return nil, fmt.Errorf("std func is not have an index. Is %T", functionNode)
			}
			stringNode, ok := indexNode.Index.(*ast.LiteralString)
			if !ok {
				return nil, fmt.Errorf("std func index does not have a name")
			}
			stdFunc, ok := (*stdLib)[stringNode.Value]
			if !ok {
				return nil, fmt.Errorf("finding std function \"%s\"", stringNode.Value)
			}
			retFunc := &ast.Function{}
			retFunc.LocRange = stringNode.LocRange
			for _, param := range stdFunc.Params {
				retFunc.Parameters = append(retFunc.Parameters, ast.Parameter{Name: ast.Identifier(param)})
			}
			return retFunc, nil
		}
	}
	return nil, fmt.Errorf("not an std func")
}

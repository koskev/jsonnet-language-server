package utils

import (
	"fmt"
	"maps"
	"reflect"
	"strings"

	"github.com/google/go-jsonnet/ast"
)

func DesugaredObjectFieldsToString(node *ast.DesugaredObject) string {
	var builder strings.Builder

	if node == nil {
		return "nil"
	}
	for _, field := range node.Fields {
		if fieldName, ok := field.Name.(*ast.LiteralString); ok {
			builder.WriteString(fmt.Sprintf("Name: %v type: %+v\n", fieldName.Value, reflect.TypeOf(field.Body)))
			if child, ok := field.Body.(*ast.DesugaredObject); ok {
				builder.WriteString(DesugaredObjectFieldsToString(child))
			}
		}
	}
	return builder.String()
}

func GetObjectFieldMap(object *ast.DesugaredObject) map[string]ast.DesugaredObjectField {
	fieldMap := map[string]ast.DesugaredObjectField{}
	for _, newField := range object.Fields {
		if nameNode, ok := newField.Name.(*ast.LiteralString); ok {
			fieldMap[nameNode.Value] = newField
		}
	}
	return fieldMap
}

// Merges all desugared Objects into one
// TODO: does not support + at the moment
func MergeDesugaredObjects(objects []*ast.DesugaredObject) *ast.DesugaredObject {
	if len(objects) == 0 {
		return nil
	}
	var newObject ast.DesugaredObject

	for len(objects) != 0 {
		var object *ast.DesugaredObject
		object, objects = objects[0], objects[1:]
		newObject.Asserts = append(newObject.Asserts, object.Asserts...)
		newObject.Fields = append(newObject.Fields, object.Fields...)
		newFields := GetObjectFieldMap(&newObject)
		currentFields := GetObjectFieldMap(object)
		maps.Copy(newFields, currentFields)
		// FUCK YOU GO AND YOUR STUPID FAKE ITERATORS! There is no way this isn't a feature. I have to miss something...
		// This is the long version of a simple "map" call...
		vals := make([]ast.DesugaredObjectField, 0, len(newFields))
		for _, v := range newFields {
			vals = append(vals, v)
		}
		newObject.Fields = vals

		newObject.Locals = append(newObject.Locals, object.Locals...)
	}
	return &newObject
}

package processing

import (
	"reflect"

	"github.com/google/go-jsonnet/ast"
	"github.com/grafana/jsonnet-language-server/pkg/nodestack"
	log "github.com/sirupsen/logrus"
)

func (p *Processor) FindTopLevelObjectsInFile(filename, importedFrom string) []*ast.DesugaredObject {
	v, ok := p.cache.GetTopLevelObject(filename, importedFrom)
	if !ok {
		rootNode, _, _ := p.vm.ImportAST(importedFrom, filename)
		v = p.FindTopLevelObjects(nodestack.NewNodeStack(rootNode))
		p.cache.PutTopLevelObject(filename, importedFrom, v)
	}
	return v
}

// Find all ast.DesugaredObject's from NodeStack
func (p *Processor) FindTopLevelObjects(stack *nodestack.NodeStack) []*ast.DesugaredObject {
	visitedLocations := map[*ast.LocationRange]bool{}
	var objects []*ast.DesugaredObject
	for !stack.IsEmpty() {
		curr := stack.Pop()
		if curr == nil {
			continue
		}
		if _, ok := visitedLocations[curr.Loc()]; ok {
			log.Debugf("Detected loop....")
			continue
		}
		visitedLocations[curr.Loc()] = true
		switch curr := curr.(type) {
		case *ast.DesugaredObject:
			objects = append(objects, curr)
		case *ast.Binary:
			stack.Push(curr.Left)
			stack.Push(curr.Right)
		case *ast.Local:
			stack.Push(curr.Body)
		case *ast.Import:
			filename := curr.File.Value
			rootNode, _, _ := p.vm.ImportAST(string(curr.Loc().File.DiagnosticFileName), filename)
			stack.Push(rootNode)
		case *ast.Index:
			indexValue, indexIsString := curr.Index.(*ast.LiteralString)
			if !indexIsString {
				continue
			}

			var container ast.Node
			// If our target is a var, the container for the index is the var ref
			if varTarget, targetIsVar := curr.Target.(*ast.Var); targetIsVar {
				log.Tracef("is var %v with target %v", varTarget.Id, reflect.TypeOf(curr))
				ref, err := p.FindVarReference(varTarget)
				if err != nil {
					log.WithError(err).Errorf("Error finding var reference, ignoring this node")
					continue
				}
				container = ref
			}

			// If we have not found a viable container, peek at the next object on the stack
			if container == nil {
				container = stack.Peek()
			}

			var possibleObjects []*ast.DesugaredObject
			if containerObj, containerIsObj := container.(*ast.DesugaredObject); containerIsObj {
				possibleObjects = []*ast.DesugaredObject{containerObj}
			} else if containerImport, containerIsImport := container.(*ast.Import); containerIsImport {
				possibleObjects = p.FindTopLevelObjectsInFile(containerImport.File.Value, string(containerImport.Loc().File.DiagnosticFileName))
			}

			for _, obj := range possibleObjects {
				for _, field := range FindObjectFieldsInObject(obj, indexValue.Value, false) {
					stack.Push(field.Body)
				}
			}
		case *ast.Var:
			varReference, err := p.FindVarReference(curr)
			if err != nil {
				log.WithError(err).Errorf("Error finding var reference, ignoring this node")
				continue
			}
			stack.Push(varReference)
		case *ast.Function:
			// XXX: This might cause cycles
			stack.Push(curr.Body)
		}
	}
	return objects
}
